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
		return result, NewTimeoutError(op, "context deadline exceeded during CPU usage collection")
	default:
	}

	// percpu must be true: with percpu=false gopsutil returns a single aggregate
	// value, and assigning it positionally would leave every entry after the
	// first at zero — which then gets averaged across all of them, reporting
	// usage smaller by a factor of the CPU count.
	if percents, err := cpu.PercentWithContext(ctx, 0, true); err == nil {
		assignUsage(result, percents)
	} else {
		// Continue with partial data
		return result, NewSystemError(op, "failed to get CPU usage percentages", err)
	}

	select {
	case <-ctx.Done():
		return result, NewTimeoutError(op, "context deadline exceeded during temperature collection")
	default:
	}

	// Temperature is optional: plenty of VMs and containers expose no sensors,
	// and that is not a collection failure.
	if temps, err := host.SensorsTemperaturesWithContext(ctx); err == nil {
		hottest := 0.0
		for _, temp := range temps {
			// coretemp reports a package sensor plus one per core; the hottest
			// is the meaningful figure rather than whichever came last.
			if IsCPUTemp(temp.SensorKey) && temp.Temperature > hottest {
				hottest = temp.Temperature
			}
		}
		if hottest > 0 {
			for i := range result {
				result[i].Temperature = hottest
			}
		}
	}

	return result, nil
}

// assignUsage maps measured percentages onto the collected CPU entries.
//
// cpu.Info and cpu.Percent do not always report the same number of entries
// (some platforms describe sockets rather than logical CPUs), so anything other
// than a 1:1 match falls back to the mean. That keeps the average across
// entries equal to overall system usage either way.
func assignUsage(cpus []CPUInfo, percents []float64) {
	if len(cpus) == 0 || len(percents) == 0 {
		return
	}

	if len(cpus) == len(percents) {
		for i := range cpus {
			cpus[i].Usage = percents[i]
		}
		return
	}

	var sum float64
	for _, p := range percents {
		sum += p
	}
	mean := sum / float64(len(percents))
	for i := range cpus {
		cpus[i].Usage = mean
	}
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
