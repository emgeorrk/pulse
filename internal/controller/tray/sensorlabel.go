//go:build darwin

package tray

import (
	"strings"

	"github.com/emgeorrk/pulse/internal/usecase"
)

// tempSensorRow pairs a sensor's raw hardware name (the stable MetricID key,
// so existing pins survive relabeling) with the label shown in the menu.
type tempSensorRow struct {
	raw   string
	label string
}

// hiddenTempSensor reports whether a sensor's row is omitted from the Temp
// group: sensors feeding the CPU or GPU aggregate (e.g. the 14 near-identical
// "PMU tdie*" points on an M5) only repeat what the CPU/GPU/Hottest rows
// already show.
func hiddenTempSensor(name string) bool {
	return usecase.IsCPUTempSensor(name) || usecase.IsGPUTempSensor(name)
}

// tempSensorLabel maps a raw sensor name ("gas gauge battery", "NAND CH0
// temp") to a user-facing label; unknown sensors keep their raw name rather
// than disappear.
func tempSensorLabel(name string) string {
	lower := strings.ToLower(name)

	switch {
	case usecase.IsGPUTempSensor(name):
		return labelGPU
	case usecase.IsCPUTempSensor(name):
		return labelCPU
	case strings.Contains(lower, "nand"):
		return "SSD"
	case strings.Contains(lower, "gas gauge"):
		return labelBattery
	default:
		return name
	}
}

// visibleTempSensors filters out aggregate-feeding sensors and resolves the
// display labels. When two sensors map to the same label (say, two NAND
// channels), all of them fall back to raw names so the rows stay
// distinguishable.
func visibleTempSensors(names []string) []tempSensorRow {
	rows := make([]tempSensorRow, 0, len(names))
	seen := map[string]int{}

	for _, name := range names {
		if hiddenTempSensor(name) {
			continue
		}

		label := tempSensorLabel(name)
		seen[label]++

		rows = append(rows, tempSensorRow{raw: name, label: label})
	}

	for i, row := range rows {
		if seen[row.label] > 1 {
			rows[i].label = row.raw
		}
	}

	return rows
}
