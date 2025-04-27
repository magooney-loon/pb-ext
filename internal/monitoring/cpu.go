package monitoring

import (
	"context"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
)

// CPUInfo holds detailed CPU information
type CPUInfo struct {
	ModelName   string  `json:"model_name"`
	Cores       int32   `json:"cores"`
	Frequency   float64 `json:"frequency_mhz"`
	Usage       float64 `json:"usage"`
	Temperature float64 `json:"temperature"`
}

// CollectCPUInfoWithContext gathers CPU information with context support
func CollectCPUInfoWithContext(ctx context.Context) ([]CPUInfo, error) {
	const op = "CollectCPUInfo"

	select {
	case <-ctx.Done():
		return nil, NewTimeoutError(op, "context deadline exceeded")
	default:
	}

	cpuInfos, err := cpu.InfoWithContext(ctx)
	if err != nil {
		return nil, NewSystemError(op, "failed to get CPU info", err)
	}

	result := make([]CPUInfo, len(cpuInfos))
	for i, info := range cpuInfos {
		result[i] = CPUInfo{
			ModelName: info.ModelName,
			Cores:     info.Cores,
			Frequency: float64(info.Mhz),
		}
	}

	select {
	case <-ctx.Done():
		return nil, NewTimeoutError(op, "context deadline exceeded during CPU usage collection")
	default:
	}

	if percents, err := cpu.PercentWithContext(ctx, 0, false); err == nil {
		for i := range result {
			if i < len(percents) {
				result[i].Usage = percents[i]
			}
		}
	} else {
		// Continue with partial data
		return result, NewSystemError(op, "failed to get CPU usage percentages", err)
	}

	select {
	case <-ctx.Done():
		return nil, NewTimeoutError(op, "context deadline exceeded during temperature collection")
	default:
	}

	if temps, err := host.SensorsTemperaturesWithContext(ctx); err == nil {
		for _, temp := range temps {
			if IsCPUTemp(temp.SensorKey) {
				// Apply temperature to all cores
				for i := range result {
					result[i].Temperature = temp.Temperature
				}
				break
			}
		}
	} else {
		// Temperature collection is optional
		return result, NewSensorError(op, "failed to get temperature data", err)
	}

	return result, nil
}

// CollectCPUInfo gathers CPU information with background context
func CollectCPUInfo() ([]CPUInfo, error) {
	return CollectCPUInfoWithContext(context.Background())
}

// IsCPUTemp identifies CPU temperature sensors
func IsCPUTemp(sensor string) bool {
	cpuSensors := []string{
		"coretemp",
		"k10temp",
		"cpu_thermal",
		"cpu-thermal",
		"cpu temperature",
	}
	for _, s := range cpuSensors {
		if strings.Contains(strings.ToLower(sensor), s) {
			return true
		}
	}
	return false
}
