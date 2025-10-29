package monitoring

import (
	"context"
	"strings"

	"github.com/shirou/gopsutil/v3/host"
)

// TemperatureInfo holds temperature sensor data
type TemperatureInfo struct {
	CPUTemp     float64 `json:"cpu_temp"`
	SystemTemp  float64 `json:"system_temp"`
	DiskTemp    float64 `json:"disk_temp"`
	AmbientTemp float64 `json:"ambient_temp"`
	HasTempData bool    `json:"has_temp_data"`
}

// CollectTemperatureInfoWithContext gathers temperature data with context
func CollectTemperatureInfoWithContext(ctx context.Context) (TemperatureInfo, error) {
	const op = "CollectTemperatureInfo"

	select {
	case <-ctx.Done():
		return TemperatureInfo{}, ctx.Err()
	default:
	}

	result := TemperatureInfo{}

	temps, err := host.SensorsTemperaturesWithContext(ctx)
	if err != nil {
		return result, NewSensorError(op, "failed to get sensors temperatures", err)
	}

	if len(temps) > 0 {
		result.HasTempData = true

		for _, temp := range temps {
			sensorKey := strings.ToLower(temp.SensorKey)

			switch {
			case IsCPUTemp(sensorKey):
				result.CPUTemp = temp.Temperature
			case IsSystemTemp(sensorKey):
				result.SystemTemp = temp.Temperature
			case IsDiskTemp(sensorKey):
				result.DiskTemp = temp.Temperature
			case strings.Contains(sensorKey, "ambient"):
				result.AmbientTemp = temp.Temperature
			}
		}
	}

	return result, nil
}

// CollectTemperatureInfo uses background context
func CollectTemperatureInfo() (TemperatureInfo, error) {
	return CollectTemperatureInfoWithContext(context.Background())
}

// IsSystemTemp identifies system temperature sensors
func IsSystemTemp(sensor string) bool {
	sysSensors := []string{
		"system",
		"board",
		"mobo",
		"ambient",
	}
	for _, s := range sysSensors {
		if strings.Contains(strings.ToLower(sensor), s) {
			return true
		}
	}
	return false
}
