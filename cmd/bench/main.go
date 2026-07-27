// Command bench boots a real pb-ext server in a temp data dir, drives it
// with a realistic mix of HTTP traffic at increasing concurrency, and
// reports throughput, latency, error rate, CPU/memory/disk, and data.db /
// auxiliary.db growth for each stage. See cmd/bench's design notes for the
// traffic mix and the reasons this can run as one self-contained binary.
//
//	go run ./cmd/bench -workers=10,50,200,1000 -duration=15s
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	pbext "github.com/magooney-loon/pb-ext/core"
)

func main() {
	os.Exit(run())
}

// run holds the whole lifecycle so cleanup (the deferred temp-dir removal in
// particular) always executes: os.Exit does not run deferred calls, so
// exiting with a nonzero status happens only once, here, after run returns.
func run() int {
	httpAddr := flag.String("http", "127.0.0.1:8091", "address for the embedded server (kept off the usual 8090 dev port)")
	dataDirFlag := flag.String("data-dir", "", "data directory to use (default: auto-created temp dir, removed after the run unless -keep)")
	keep := flag.Bool("keep", false, "keep an auto-created temp data dir after the run for inspection")
	workersFlag := flag.String("workers", "10,50,200,1000", "comma-separated concurrency levels; one stage per value")
	duration := flag.Duration("duration", 15*time.Second, "duration of each stage")
	sampleInterval := flag.Duration("sample-interval", 2*time.Second, "resource/DB sampling interval during a stage")
	flag.Parse()

	stages, err := parseWorkerStages(*workersFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bench:", err)
		return 1
	}

	dataDir, ownDataDir, err := resolveDataDir(*dataDirFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bench: preparing data dir:", err)
		return 1
	}
	if ownDataDir && !*keep {
		defer os.RemoveAll(dataDir)
	}

	srv := buildBenchServer(dataDir)

	// PocketBase's serve command only parses --http off os.Args when
	// srv.Start() executes its root command, so this must be set right
	// before Start rather than earlier — see server.go's buildBenchServer
	// doc comment for why --dir goes through Config instead.
	os.Args = []string{"pbext-bench", "serve", "--http=" + *httpAddr}
	startErr := make(chan error, 1)
	go func() { startErr <- srv.Start() }()

	if err := waitForHealthy(*httpAddr, 15*time.Second); err != nil {
		fmt.Fprintln(os.Stderr, "bench:", err)
		return 1
	}

	authToken, err := authenticateBenchSuperuser(*httpAddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bench: warning: dashboard auth failed, /_/_ will hit the login page instead of the full render:", err)
	}

	maxW := maxWorkers(stages)
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        maxW * 2,
			MaxIdleConnsPerHost: maxW * 2,
		},
	}

	fmt.Printf("bench: server ready at http://%s (data dir: %s)\n", *httpAddr, dataDir)
	fmt.Printf("bench: running %d stage(s), %s each: workers=%v\n\n", len(stages), *duration, stages)

	results := make([]stageResult, 0, len(stages))
	for i, w := range stages {
		results = append(results, runStage(srv, client, *httpAddr, authToken, dataDir, w, *duration, *sampleInterval, i+1, len(stages)))
	}

	shutdownBenchServer(srv, startErr)

	// Taken after the graceful shutdown's final analytics/audit flush and WAL
	// checkpoint, so this is a truer "after" figure than the last stage's own
	// snapshot (which was read a few seconds before that flush).
	finalDB := readDBSizes(dataDir)

	printReport(*httpAddr, dataDir, results, finalDB)
	return 0
}

func parseWorkerStages(s string) ([]int, error) {
	parts := strings.Split(s, ",")
	stages := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		n, err := strconv.Atoi(p)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid -workers value %q: must be a comma-separated list of positive integers", p)
		}
		stages = append(stages, n)
	}
	return stages, nil
}

func maxWorkers(stages []int) int {
	m := 0
	for _, w := range stages {
		if w > m {
			m = w
		}
	}
	return m
}

// resolveDataDir returns the data dir to use and whether bench created it
// itself. Only a self-created temp dir is ever auto-removed.
func resolveDataDir(flagValue string) (dir string, ownDataDir bool, err error) {
	if flagValue != "" {
		if err := os.MkdirAll(flagValue, 0o755); err != nil {
			return "", false, err
		}
		return flagValue, false, nil
	}
	dir, err = os.MkdirTemp("", "pbext-bench-")
	return dir, true, err
}

// shutdownBenchServer sends the process the same signal Ctrl+C would, so the
// server takes its normal OnTerminate path — flushing analytics and the
// audit log — before main reads final DB state.
func shutdownBenchServer(srv *pbext.Server, startErr chan error) {
	proc, err := os.FindProcess(os.Getpid())
	if err == nil {
		_ = proc.Signal(os.Interrupt)
	}

	select {
	case err := <-startErr:
		if err != nil {
			fmt.Fprintln(os.Stderr, "bench: server stopped with error:", err)
		}
	case <-time.After(10 * time.Second):
		fmt.Fprintln(os.Stderr, "bench: warning: server did not shut down within 10s")
	}
}
