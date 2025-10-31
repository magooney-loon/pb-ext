package monitoring

import (
	"context"
	"errors"
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
	var multiError []error
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
	} else {
		multiError = append(multiError, err)
	}

	if memPercent, err := proc.MemoryPercentWithContext(ctx); err == nil {
		result.MemoryPercent = float64(memPercent)
	} else {
		multiError = append(multiError, err)
	}

	select {
	case <-ctx.Done():
		return result, ctx.Err()
	default:
	}

	if memInfo, err := proc.MemoryInfoWithContext(ctx); err == nil {
		result.RSS = memInfo.RSS
		result.VMS = memInfo.VMS
	} else {
		multiError = append(multiError, err)
	}

	if numThreads, err := proc.NumThreadsWithContext(ctx); err == nil {
		result.NumThreads = numThreads
	} else {
		multiError = append(multiError, err)
	}

	if numFiles, err := proc.NumFDsWithContext(ctx); err == nil {
		result.OpenFiles = numFiles
	} else {
		multiError = append(multiError, err)
	}

	err = nil
	if len(multiError) >= 1 {
		err = errors.Join(multiError...)
	}

	return result, err
}

// CollectProcessInfo uses background context
func CollectProcessInfo() (ProcessInfo, error) {
	return CollectProcessInfoWithContext(context.Background())
}
