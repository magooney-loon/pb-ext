package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// requestKind is one entry in the weighted traffic mix. expected404 marks the
// scanner-noise endpoint, whose 404s are the point rather than a failure, so
// the error-rate metric can exclude them.
type requestKind struct {
	label       string
	weight      int
	expected404 bool
	build       func(seq int64) (method, path string, body string)
}

var pagePaths = func() []string {
	paths := make([]string, 0, len(benchPages))
	for p := range benchPages {
		paths = append(paths, p)
	}
	return paths
}()

var trafficMix = []requestKind{
	{
		label:  "GET /api/health",
		weight: 25,
		build: func(seq int64) (string, string, string) {
			return http.MethodGet, "/api/health", ""
		},
	},
	{
		label:  "GET page",
		weight: 20,
		build: func(seq int64) (string, string, string) {
			return http.MethodGet, pagePaths[int(seq)%len(pagePaths)], ""
		},
	},
	{
		label:  "GET records",
		weight: 20,
		build: func(seq int64) (string, string, string) {
			return http.MethodGet, "/api/collections/" + benchCollectionName + "/records?perPage=20&sort=-created", ""
		},
	},
	{
		label:  "POST records",
		weight: 20,
		build: func(seq int64) (string, string, string) {
			return http.MethodPost, "/api/collections/" + benchCollectionName + "/records",
				fmt.Sprintf(`{"title":"bench-%d"}`, seq)
		},
	},
	{
		label:  "GET dashboard",
		weight: 10,
		build: func(seq int64) (string, string, string) {
			return http.MethodGet, "/_/_", ""
		},
	},
	{
		label:       "GET 404",
		weight:      5,
		expected404: true,
		build: func(seq int64) (string, string, string) {
			return http.MethodGet, fmt.Sprintf("/nope/%d", seq), ""
		},
	},
}

// pick returns a weighted-random index into trafficMix.
func pickTrafficKind(rng *rand.Rand) int {
	total := 0
	for _, k := range trafficMix {
		total += k.weight
	}
	r := rng.Intn(total)
	for i, k := range trafficMix {
		if r < k.weight {
			return i
		}
		r -= k.weight
	}
	return len(trafficMix) - 1
}

// endpointStats accumulates per-request-kind counters across all workers in
// a stage.
type endpointStats struct {
	requests atomic.Int64
	errors   atomic.Int64
}

// loadResult is what a stage's worker pool produces: overall latency
// distribution plus a breakdown by traffic-mix entry.
type loadResult struct {
	hist      histogram
	byKind    []endpointStats // indices align with trafficMix
	total     atomic.Int64
	errors    atomic.Int64 // 5xx, unexpected 4xx, and transport errors
	transport atomic.Int64
	seq       atomic.Int64
}

func newLoadResult() *loadResult {
	return &loadResult{byKind: make([]endpointStats, len(trafficMix))}
}

// runLoad drives workers goroutines against addr until ctx is done, using a
// shared client so connections are reused within and across stages.
func runLoad(ctx context.Context, client *http.Client, addr, authToken string, workers int, result *loadResult) {
	done := make(chan struct{})
	for w := 0; w < workers; w++ {
		go func(seed int64) {
			rng := rand.New(rand.NewSource(seed))
			for {
				select {
				case <-ctx.Done():
					done <- struct{}{}
					return
				default:
				}
				fireOneRequest(ctx, client, addr, authToken, rng, result)
			}
		}(time.Now().UnixNano() + int64(w))
	}

	for w := 0; w < workers; w++ {
		<-done
	}
}

func fireOneRequest(ctx context.Context, client *http.Client, addr, authToken string, rng *rand.Rand, result *loadResult) {
	idx := pickTrafficKind(rng)
	kind := trafficMix[idx]
	seq := result.seq.Add(1)
	method, path, body := kind.build(seq)

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, "http://"+addr+path, strings.NewReader(body))
	if err != nil {
		result.transport.Add(1)
		result.total.Add(1)
		return
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if path == "/_/_" && authToken != "" {
		req.Header.Set("Authorization", authToken)
	}

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)

	result.total.Add(1)
	result.byKind[idx].requests.Add(1)
	result.hist.record(elapsed)

	if err != nil {
		result.transport.Add(1)
		result.errors.Add(1)
		result.byKind[idx].errors.Add(1)
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	isError := false
	switch {
	case kind.expected404:
		isError = resp.StatusCode != http.StatusNotFound
	case resp.StatusCode >= 400:
		isError = true
	}
	if isError {
		result.errors.Add(1)
		result.byKind[idx].errors.Add(1)
	}
}
