package usecase

import (
	"strings"

	"github.com/emgeorrk/pulse/internal/entity"
)

// HID sensor names differ across chip generations (M1: "pACC MTR Temp
// Sensor…", newer: "PMU tdie…", Intel SMC: "CPU die"), so we aggregate by
// substring rather than exact name.

// AggregateTemps computes CPU/GPU aggregates (average across matching
// sensors) and the hottest sensor. If nothing matches, the aggregate stays
// 0 and the UI doesn't show it.
func AggregateTemps(all []entity.Reading) entity.TempStats {
	stats := entity.TempStats{All: all}

	var (
		cpuSum, gpuSum float64
		cpuN, gpuN     int
	)

	for _, r := range all {
		name := strings.ToLower(r.Name)
		if r.Value > stats.Hottest.Value {
			stats.Hottest = r
		}

		if matchAny(name, gpuTemperatureMarkers()) {
			gpuSum += r.Value
			gpuN++

			continue // "GPU" must not land in the CPU aggregate via the "soc" marker
		}

		if matchAny(name, cpuTemperatureMarkers()) {
			cpuSum += r.Value
			cpuN++
		}
	}

	if cpuN > 0 {
		stats.CPU = cpuSum / float64(cpuN)
	}

	if gpuN > 0 {
		stats.GPU = gpuSum / float64(gpuN)
	}

	return stats
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
