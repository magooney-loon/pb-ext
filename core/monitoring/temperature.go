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

	for _, temp := range temps {
		sensorKey := strings.ToLower(temp.SensorKey)

		// Ambient is matched before system: IsSystemTemp also accepts "ambient",
		// so testing it first would swallow every ambient sensor and leave
		// AmbientTemp permanently zero.
		//
		// Each category keeps the highest reading rather than whichever sensor
		// happened to come last — sensor order is not guaranteed, and a group
		// like coretemp reports a package sensor plus one per core.
		switch {
		case IsCPUTemp(sensorKey):
			result.CPUTemp = max(result.CPUTemp, temp.Temperature)
		case IsAmbientTemp(sensorKey):
			result.AmbientTemp = max(result.AmbientTemp, temp.Temperature)
		case IsSystemTemp(sensorKey):
			result.SystemTemp = max(result.SystemTemp, temp.Temperature)
		case IsDiskTemp(sensorKey):
			result.DiskTemp = max(result.DiskTemp, temp.Temperature)
		default:
			continue
		}

		// Only set once a sensor was actually recognised, so a host that
		// exposes nothing we understand doesn't advertise temperature data.
		result.HasTempData = true
	}

	return result, nil
}

// IsAmbientTemp identifies ambient/intake temperature sensors.
func IsAmbientTemp(sensor string) bool {
	return strings.Contains(strings.ToLower(sensor), "ambient")
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
