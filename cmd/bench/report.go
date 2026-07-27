package main

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"
)

// printReport renders the final summary once every stage has run: throughput
// and latency per stage, resource usage per stage, database growth for the
// whole run, and a per-endpoint breakdown of the final (heaviest) stage.
func printReport(addr, dataDir string, stages []stageResult, finalDB dbSizes) {
	fmt.Println()
	fmt.Println("=== pb-ext bench ===")
	fmt.Printf("server: %s   data dir: %s\n", addr, dataDir)
	fmt.Println("note: the load generator runs in the same process as the server under test, so CPU/RSS below is server + client combined.")
	fmt.Println()

	printThroughputTable(stages)
	fmt.Println()
	printResourceTable(stages)
	fmt.Println()
	printDBGrowth(stages, finalDB)
	fmt.Println()
	printEndpointBreakdown(stages[len(stages)-1])
}

func printThroughputTable(stages []stageResult) {
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "STAGE\tWORKERS\tDURATION\tREQUESTS\tREQ/S\tP50\tP90\tP99\tMAX\tERR%")
	for i, s := range stages {
		fmt.Fprintf(w, "%d\t%d\t%s\t%d\t%.1f\t%s\t%s\t%s\t%s\t%.2f%%\n",
			i+1, s.workers, s.duration.Round(time.Second), s.requests, s.reqPerSec(),
			s.p50, s.p90, s.p99, s.max, s.errRate()*100)
	}
	w.Flush()
}

func printResourceTable(stages []stageResult) {
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "STAGE\tCPU% (avg/peak)\tRSS (avg/peak)\tDISK% (avg/peak)")
	for i, s := range stages {
		cpuAvg, cpuPeak := s.cpuAvgPeak()
		rssAvg, rssPeak := s.rssAvgPeak()
		diskAvg, diskPeak := s.diskAvgPeak()
		fmt.Fprintf(w, "%d\t%.1f%% / %.1f%%\t%s / %s\t%.1f%% / %.1f%%\n",
			i+1, cpuAvg, cpuPeak, formatBytes(int64(rssAvg)), formatBytes(int64(rssPeak)), diskAvg, diskPeak)
	}
	w.Flush()
}

// printDBGrowth reports growth across the whole run. finalDB is read after
// the graceful shutdown's flush and WAL checkpoint, so it is a truer "after"
// figure than the last stage's own mid-run snapshot; row counts still come
// from that snapshot since the app is no longer reachable once shut down.
func printDBGrowth(stages []stageResult, finalDB dbSizes) {
	first := stages[0]
	last := stages[len(stages)-1]

	fmt.Println("Database growth (whole run):")
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "FILE\tBEFORE\tAFTER\tDELTA")
	fmt.Fprintf(w, "data.db\t%s\t%s\t%s\n", formatBytes(first.dbBefore.dataDB), formatBytes(finalDB.dataDB), formatDelta(finalDB.dataDB-first.dbBefore.dataDB))
	fmt.Fprintf(w, "data.db-wal\t%s\t%s\t%s\n", formatBytes(first.dbBefore.dataWAL), formatBytes(finalDB.dataWAL), formatDelta(finalDB.dataWAL-first.dbBefore.dataWAL))
	fmt.Fprintf(w, "auxiliary.db\t%s\t%s\t%s\n", formatBytes(first.dbBefore.auxDB), formatBytes(finalDB.auxDB), formatDelta(finalDB.auxDB-first.dbBefore.auxDB))
	fmt.Fprintf(w, "auxiliary.db-wal\t%s\t%s\t%s\n", formatBytes(first.dbBefore.auxWAL), formatBytes(finalDB.auxWAL), formatDelta(finalDB.auxWAL-first.dbBefore.auxWAL))
	w.Flush()

	fmt.Printf("  %s rows: %d (as of end of load, before the final flush)\n", benchCollectionName, last.benchItems)
	fmt.Printf("  _analytics rows: %d (tracked views: %d; as of end of load, before the final flush)\n", last.analyticsRows, last.analyticsViews)
}

func printEndpointBreakdown(final stageResult) {
	fmt.Printf("Per-endpoint breakdown (final stage, %d workers):\n", final.workers)
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "ENDPOINT\tREQUESTS\tERRORS")
	for _, k := range final.byKind {
		fmt.Fprintf(w, "%s\t%d\t%d\n", k.label, k.requests, k.errors)
	}
	w.Flush()
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < 0 {
		return "-" + formatBytes(-b)
	}
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatDelta(b int64) string {
	if b >= 0 {
		return "+" + formatBytes(b)
	}
	return formatBytes(b)
}
