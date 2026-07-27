package monitoring

import (
	"context"
	"strings"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
)

// DiskInfo holds disk information.
//
// As with memory, Used + Free does not equal Total: Free counts blocks
// available to unprivileged users while Total counts every block, and the
// difference is the filesystem's reserved blocks (5% by default on ext4).
// Usage is therefore Used/(Used+Free) — the same ratio df reports — and not
// Used/Total, which would understate how full the disk is.
type DiskInfo struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
	// Free is space available to unprivileged users, excluding reserved blocks.
	Free  uint64  `json:"free"`
	Usage float64 `json:"usage_percent"`
	Path  string  `json:"path"`
}

// DefaultDiskPath is the filesystem measured when no path is supplied, and the
// fallback when the requested path cannot be queried.
const DefaultDiskPath = "/"

// CollectDiskInfoWithContext reports usage for the filesystem containing path.
//
// Callers should pass the directory they actually care about — for a PocketBase
// server that is the data directory, since "/" is frequently a small read-only
// image layer in a container and says nothing about where the database will run
// out of room. An empty path, or one that cannot be queried, falls back to
// DefaultDiskPath; the returned Path reports whichever was measured.
func CollectDiskInfoWithContext(ctx context.Context, path string) (DiskInfo, error) {
	select {
	case <-ctx.Done():
		return DiskInfo{}, ctx.Err()
	default:
	}

	if path == "" {
		path = DefaultDiskPath
	}

	usage, err := disk.UsageWithContext(ctx, path)
	if err != nil && path != DefaultDiskPath {
		// The data directory may not exist yet on a first run.
		if fallback, fallbackErr := disk.UsageWithContext(ctx, DefaultDiskPath); fallbackErr == nil {
			usage, err, path = fallback, nil, DefaultDiskPath
		}
	}

	result := DiskInfo{Path: path}
	if err != nil {
		return result, err
	}

	result.Total = usage.Total
	result.Used = usage.Used
	result.Free = usage.Free
	result.Usage = usage.UsedPercent

	return result, nil
}

// CollectDiskInfo uses background context
func CollectDiskInfo(path string) (DiskInfo, error) {
	return CollectDiskInfoWithContext(context.Background(), path)
}

// GetDiskTemperatureWithContext retrieves disk temperature with context
func GetDiskTemperatureWithContext(ctx context.Context) (float64, bool) {
	select {
	case <-ctx.Done():
		return 0, false
	default:
	}

	temps, err := host.SensorsTemperaturesWithContext(ctx)
	if err != nil {
		return 0, false
	}

	for _, temp := range temps {
		if IsDiskTemp(temp.SensorKey) {
			return temp.Temperature, true
		}
	}

	return 0, false
}

// GetDiskTemperature uses background context
func GetDiskTemperature() (float64, bool) {
	return GetDiskTemperatureWithContext(context.Background())
}

// IsDiskTemp identifies disk temperature sensors
func IsDiskTemp(sensor string) bool {
	diskSensors := []string{
		"nvme",
		"drive",
		"hdd",
		"ssd",
		"disk",
	}
	for _, s := range diskSensors {
		if strings.Contains(strings.ToLower(sensor), s) {
			return true
		}
	}
	return false
}
