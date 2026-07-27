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

	// OpenFilesLimit is the soft RLIMIT_NOFILE, or 0 where it is unknown
	// (Windows, or a failed lookup).
	OpenFilesLimit uint64 `json:"open_files_limit"`
	// OpenFilesPercent is OpenFiles as a share of OpenFilesLimit, and 0 when the
	// limit is unknown. Descriptor exhaustion is abrupt and total — accepts start
	// failing, uploads stop, SQLite cannot open its WAL — so the ratio is worth
	// watching even though the raw count is not.
	OpenFilesPercent float64 `json:"open_files_percent"`
}

// maxPlausibleFDLimit is the largest descriptor ceiling treated as a real
// measurement. Actual limits top out around 2^30; anything at or above this is
// RLIM_INFINITY, a sign-conversion artefact, or nonsense.
const maxPlausibleFDLimit uint64 = 1 << 31

// normalizeFDLimit maps an unusable descriptor ceiling to 0, which every
// consumer already reads as "skip the saturation check".
//
// "Unlimited" has to collapse to the same answer as "unknown", and not just for
// tidiness: RLIM_INFINITY is ^uint64(0) on Linux and max-int64 on the BSDs, so
// passing it through verbatim put 18446744073709551615 into the dashboard and
// the alert metrics. There is no ratio to compute against an absent ceiling, and
// a process that cannot open a single file is not a state worth a percentage
// either.
//
// Keeping this out of the build-tagged file makes it testable on any platform,
// including the values this host will never produce.
func normalizeFDLimit(raw uint64) uint64 {
	if raw == 0 || raw >= maxPlausibleFDLimit {
		return 0
	}
	return raw
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

	// A host that will not report its descriptor ceiling is not a collection
	// failure; the ratio is simply unavailable, and stays 0 so consumers can
	// tell "unknown" from "empty".
	result.OpenFilesLimit = openFilesLimit()
	if result.OpenFilesLimit > 0 && result.OpenFiles > 0 {
		result.OpenFilesPercent = float64(result.OpenFiles) / float64(result.OpenFilesLimit) * 100
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
