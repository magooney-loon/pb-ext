package monitoring

import (
	"runtime"
	"time"
)

// RuntimeStats holds Go runtime statistics
type RuntimeStats struct {
	NumGoroutines   int           `json:"num_goroutines"`
	NumCPU          int           `json:"num_cpu"`
	NumCgoCall      int64         `json:"num_cgo_call"`
	AllocatedBytes  uint64        `json:"allocated_bytes"`
	TotalAllocBytes uint64        `json:"total_alloc_bytes"`
	HeapObjects     uint64        `json:"heap_objects"`
	GCPauseTotal    uint64        `json:"gc_pause_total"`
	LastGCTime      time.Time     `json:"last_gc_time"`
	NextGC          uint64        `json:"next_gc"`
	NumGC           uint32        `json:"num_gc"`
	LastGCDuration  time.Duration `json:"last_gc_duration"`
}

// CollectRuntimeStats gathers Go runtime metrics
func CollectRuntimeStats() RuntimeStats {
	var rtStats runtime.MemStats
	runtime.ReadMemStats(&rtStats)

	result := RuntimeStats{
		NumGoroutines:   runtime.NumGoroutine(),
		NumCPU:          runtime.NumCPU(),
		NumCgoCall:      runtime.NumCgoCall(),
		AllocatedBytes:  rtStats.Alloc,
		TotalAllocBytes: rtStats.TotalAlloc,
		HeapObjects:     rtStats.HeapObjects,
		GCPauseTotal:    rtStats.PauseTotalNs,
		LastGCTime:      time.Unix(0, int64(rtStats.LastGC)),
		NextGC:          rtStats.NextGC,
		NumGC:           rtStats.NumGC,
		LastGCDuration:  time.Duration(rtStats.PauseNs[(rtStats.NumGC+255)%256]),
	}

	return result
}
