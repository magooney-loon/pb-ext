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

// CollectProcessInfoWithContext gathers current process info with context
func CollectProcessInfoWithContext(ctx context.Context) (ProcessInfo, error) {
	select {
	case <-ctx.Done():
		return ProcessInfo{}, ctx.Err()
	default:
	}

	result := ProcessInfo{}

	proc, err := process.NewProcessWithContext(ctx, int32(os.Getpid()))
	if err != nil {
		return result, err
	}

	result.PID = proc.Pid

	select {
	case <-ctx.Done():
		return result, ctx.Err()
	default:
	}

	if cpuPercent, err := proc.CPUPercentWithContext(ctx); err == nil {
		result.CPUPercent = cpuPercent
	}

	if memPercent, err := proc.MemoryPercentWithContext(ctx); err == nil {
		result.MemoryPercent = float64(memPercent)
	}

	select {
	case <-ctx.Done():
		return result, ctx.Err()
	default:
	}

	if memInfo, err := proc.MemoryInfoWithContext(ctx); err == nil {
		result.RSS = memInfo.RSS
		result.VMS = memInfo.VMS
	}

	if numThreads, err := proc.NumThreadsWithContext(ctx); err == nil {
		result.NumThreads = numThreads
	}

	if numFiles, err := proc.NumFDsWithContext(ctx); err == nil {
		result.OpenFiles = numFiles
	}

	return result, nil
}

// CollectProcessInfo uses background context
func CollectProcessInfo() (ProcessInfo, error) {
	return CollectProcessInfoWithContext(context.Background())
}
