package usecase

import (
	"strings"

	"github.com/emgeorrk/pulse/internal/entity"
)

// Имена HID-сенсоров различаются между поколениями чипов (M1: "pACC MTR Temp
// Sensor…", новее: "PMU tdie…", Intel-SMC: "CPU die"), поэтому агрегируем по
// подстрокам, а не по точным именам.
var (
	cpuTempMarkers = []string{"tdie", "pacc", "eacc", "soc", "cpu"}
	gpuTempMarkers = []string{"gpu"}
)

// AggregateTemps считает CPU/GPU-агрегаты (среднее по совпавшим сенсорам)
// и самый горячий сенсор. Не совпало ничего — агрегат остаётся 0 и UI его
// не показывает.
func AggregateTemps(all []entity.Reading) entity.TempStats {
	stats := entity.TempStats{All: all}

	var cpuSum, gpuSum float64
	var cpuN, gpuN int
	for _, r := range all {
		name := strings.ToLower(r.Name)
		if r.Value > stats.Hottest.Value {
			stats.Hottest = r
		}
		if matchAny(name, gpuTempMarkers) {
			gpuSum += r.Value
			gpuN++
			continue // "GPU" не должен попадать в CPU-агрегат по маркеру "soc"
		}
		if matchAny(name, cpuTempMarkers) {
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

func matchAny(s string, markers []string) bool {
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}
