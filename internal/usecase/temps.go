package usecase

import (
	"strings"

	"github.com/emgeorrk/pulse/internal/entity"
)

// HID sensor names differ across chip generations (M1: "pACC MTR Temp
// Sensor…", newer: "PMU tdie…", Intel SMC: "CPU die"), so we aggregate by
// substring rather than exact name.

// AggregateTemps computes CPU/GPU aggregates (average across matching
// sensors), the overall average and the hottest/coolest sensors. If nothing
// matches, the aggregate stays 0 and the UI doesn't show it.
func AggregateTemps(all []entity.Reading) entity.TempStats {
	stats := entity.TempStats{All: all}

	var (
		sum, cpuSum, gpuSum float64
		cpuN, gpuN          int
	)

	for i, r := range all {
		sum += r.Value

		if r.Value > stats.Hottest.Value {
			stats.Hottest = r
		}

		// Coolest starts from the first reading, not from the zero value.
		if i == 0 || r.Value < stats.Coolest.Value {
			stats.Coolest = r
		}

		if IsGPUTempSensor(r.Name) {
			gpuSum += r.Value
			gpuN++

			continue // "GPU" must not land in the CPU aggregate via the "soc" marker
		}

		if IsCPUTempSensor(r.Name) {
			cpuSum += r.Value
			cpuN++
		}
	}

	if len(all) > 0 {
		stats.Avg = sum / float64(len(all))
	}

	if cpuN > 0 {
		stats.CPU = cpuSum / float64(cpuN)
	}

	if gpuN > 0 {
		stats.GPU = gpuSum / float64(gpuN)
	}

	return stats
}

// IsGPUTempSensor reports whether this sensor feeds the GPU aggregate.
func IsGPUTempSensor(name string) bool {
	return matchAny(strings.ToLower(name), gpuTemperatureMarkers())
}

// IsCPUTempSensor reports whether this sensor feeds the CPU aggregate. A GPU
// sensor never does, even when it matches a CPU marker (e.g. "soc").
func IsCPUTempSensor(name string) bool {
	return !IsGPUTempSensor(name) && matchAny(strings.ToLower(name), cpuTemperatureMarkers())
}

func cpuTemperatureMarkers() []string {
	return []string{"tdie", "pacc", "eacc", "soc", "cpu"}
}

func gpuTemperatureMarkers() []string {
	return []string{"gpu"}
}

func matchAny(s string, markers []string) bool {
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}

	return false
}
