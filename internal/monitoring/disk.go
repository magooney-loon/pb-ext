package monitoring

import (
	"context"
	"strings"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
)

// DiskInfo holds detailed disk information
type DiskInfo struct {
	Total uint64  `json:"total"`
	Used  uint64  `json:"used"`
	Free  uint64  `json:"free"`
	Usage float64 `json:"usage_percent"`
	Path  string  `json:"path"`
}

// CollectDiskInfoWithContext gathers disk usage information for the root filesystem with context support
func CollectDiskInfoWithContext(ctx context.Context) (DiskInfo, error) {
	// Check context
	select {
	case <-ctx.Done():
		return DiskInfo{}, ctx.Err()
	default:
		// Continue collection
	}

	result := DiskInfo{
		Path: "/",
	}

	// Get disk usage stats
	if diskInfo, err := disk.UsageWithContext(ctx, "/"); err == nil {
		result.Total = diskInfo.Total
		result.Used = diskInfo.Used
		result.Free = diskInfo.Free
		result.Usage = diskInfo.UsedPercent
	} else {
		return result, err
	}

	return result, nil
}

// CollectDiskInfo gathers disk usage information for the root filesystem
// Legacy function that uses a background context
func CollectDiskInfo() (DiskInfo, error) {
	return CollectDiskInfoWithContext(context.Background())
}

// GetDiskTemperatureWithContext tries to retrieve disk temperature if available with context support
func GetDiskTemperatureWithContext(ctx context.Context) (float64, bool) {
	// Check context
	select {
	case <-ctx.Done():
		return 0, false
	default:
		// Continue collection
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

// GetDiskTemperature tries to retrieve disk temperature if available
// Legacy function that uses a background context
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
