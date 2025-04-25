package monitoring

import (
	"context"
	"os"

	"github.com/shirou/gopsutil/v3/process"
)

// ProcessInfo holds process-specific information
type ProcessInfo struct {
	PID           int32   `json:"pid"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	RSS           uint64  `json:"rss"`
	VMS           uint64  `json:"vms"`
	OpenFiles     int32   `json:"open_files"`
	NumThreads    int32   `json:"num_threads"`
}

// CollectProcessInfoWithContext gathers information about the current process with context support
func CollectProcessInfoWithContext(ctx context.Context) (ProcessInfo, error) {
	// Check context
	select {
	case <-ctx.Done():
		return ProcessInfo{}, ctx.Err()
	default:
		// Continue collection
	}

	result := ProcessInfo{}

	// Get process info for current PID
	proc, err := process.NewProcessWithContext(ctx, int32(os.Getpid()))
	if err != nil {
		return result, err
	}

	// Set PID
	result.PID = proc.Pid

	// Check context between operations
	select {
	case <-ctx.Done():
		return result, ctx.Err()
	default:
		// Continue collection
	}

	// Get CPU usage
	if cpuPercent, err := proc.CPUPercentWithContext(ctx); err == nil {
		result.CPUPercent = cpuPercent
	}

	// Get memory usage
	if memPercent, err := proc.MemoryPercentWithContext(ctx); err == nil {
		result.MemoryPercent = float64(memPercent)
	}

	// Check context again
	select {
	case <-ctx.Done():
		return result, ctx.Err()
	default:
		// Continue collection
	}

	// Get memory details
	if memInfo, err := proc.MemoryInfoWithContext(ctx); err == nil {
		result.RSS = memInfo.RSS
		result.VMS = memInfo.VMS
	}

	// Get thread count
	if numThreads, err := proc.NumThreadsWithContext(ctx); err == nil {
		result.NumThreads = numThreads
	}

	// Get open file count
	if numFiles, err := proc.NumFDsWithContext(ctx); err == nil {
		result.OpenFiles = numFiles
	}

	return result, nil
}

// CollectProcessInfo gathers information about the current process
// Legacy function that uses a background context
func CollectProcessInfo() (ProcessInfo, error) {
	return CollectProcessInfoWithContext(context.Background())
}
