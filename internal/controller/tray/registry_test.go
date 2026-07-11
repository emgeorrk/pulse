//go:build darwin

package tray

import (
	"testing"

	"github.com/emgeorrk/pulse/config"
	"github.com/emgeorrk/pulse/internal/entity"
)

func fullCaps() entity.Caps {
	return entity.Caps{
		Net:          true,
		NetIfaces:    []string{"en0"},
		Disk:         true,
		Temps:        true,
		TempSensors:  []string{"PMU tdie0"},
		Volts:        true,
		VoltSensors:  []string{"PMU vbus"},
		Fans:         true,
		FanCount:     1,
		Battery:      true,
		GPU:          true,
		Power:        true,
		Freq:         true,
		FreqClusters: []string{"E-cores"},
	}
}

// Every metric in the registry must be pinnable and render in the menu bar
// — even on an empty frame, before any sensor data has arrived (regression:
// entries without a bar func used to ignore clicks).
func TestEveryMetricRendersInBar(t *testing.T) {
	hw := entity.HWInfo{NumCores: 2}
	cfg := config.Config{}

	for _, snap := range []entity.Snapshot{{}, sampleSnapshot()} {
		for _, g := range buildGroups(hw, fullCaps()) {
			for _, m := range g.metrics {
				if got := m.barText(snap, cfg); got == "" {
					t.Errorf("%s: empty barText", m.id)
				}
				if got := m.menu(snap, cfg); got == "" {
					t.Errorf("%s: empty menu", m.id)
				}
			}
		}
	}
}

// New must register every metric in the bar map, including ones that used
// to be unpinnable (disk.free, temp.hottest, …).
func TestNewRegistersAllMetricsAsPinnable(t *testing.T) {
	tr := New(config.Load(""), entity.HWInfo{NumCores: 2}, fullCaps())

	total := 0
	for _, g := range tr.groups {
		total += len(g.metrics)
	}
	if len(tr.bar) != total {
		t.Errorf("bar has %d metrics, groups have %d", len(tr.bar), total)
	}
	for _, id := range []entity.MetricID{"disk.free", "temp.hottest", "mem.available", "cpu.core.1", "temp.sensor.PMU tdie0"} {
		if _, ok := tr.bar[id]; !ok {
			t.Errorf("%s missing from the bar map", id)
		}
	}
}

func sampleSnapshot() entity.Snapshot {
	return entity.Snapshot{
		CPU: entity.CPUStats{Total: 0.42, Cores: []float64{0.1, 0.9}},
		Mem: entity.MemStats{Total: 16 << 30, Used: 8 << 30, Available: 8 << 30, SwapUsed: 1 << 30},
		Net: &entity.NetStats{
			Down: 1200, Up: 300, SessionDown: 5 << 20, SessionUp: 1 << 20,
			Ifaces: []entity.NetIface{{Name: "en0", Down: 1200, Up: 300}},
		},
		Disk: &entity.DiskStats{
			DiskUsage: entity.DiskUsage{Total: 500 << 30, Used: 300 << 30, Available: 200 << 30},
			ReadRate:  1 << 20, WriteRate: 1 << 19, ReadTotal: 10 << 30, WriteTotal: 5 << 30,
		},
		Temps: &entity.TempStats{
			CPU: 54, GPU: 48,
			Hottest: entity.Reading{Name: "PMU tdie0", Value: 61},
			All:     []entity.Reading{{Name: "PMU tdie0", Value: 61}},
		},
		Volts:   []entity.Reading{{Name: "PMU vbus", Value: 13.08}},
		Fans:    []entity.Fan{{Name: "Fan 1", RPM: 1850, Max: 5000}},
		Battery: &entity.BatteryStats{Percent: 0.87, Health: 0.95, Cycles: 120, TempC: 32, Volts: 12.3, Watts: -8.4, MinutesLeft: 185},
		GPU:     &entity.GPUStats{Utilization: 0.33},
		Power:   &entity.PowerStats{CPU: 2.1, GPU: 1.2, ANE: 0.1, Total: 3.4},
		Freq: &entity.FreqStats{
			Clusters: []entity.Reading{{Name: "E-cores", Value: 2.1e9}},
			Max:      3.5e9,
		},
	}
}
