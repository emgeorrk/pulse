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
	t.Parallel()

	tests := []struct {
		name string
		snap entity.Snapshot
	}{
		{name: "empty frame", snap: entity.Snapshot{}},
		{name: "sample frame", snap: sampleSnapshot()},
		{name: "fallback frame", snap: fallbackSnapshot()},
		{name: "charging frame", snap: chargingSnapshot()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hw := entity.HWInfo{NumCores: 2}
			cfg := config.Config{}

			for _, g := range buildGroups(hw, fullCaps()) {
				for _, m := range g.metrics {
					if got := m.barValue(tt.snap, cfg); got == "" {
						t.Errorf("%s: empty barValue", m.id)
					}

					if got := m.menu(tt.snap, cfg); got == "" {
						t.Errorf("%s: empty menu", m.id)
					}
				}
			}
		})
	}
}

// The header aggregates were never exercised (TestEveryMetricRendersInBar
// only drives the submenu metrics), so cover every group's aggregate across
// absent, present, and charging frames.
func TestGroupAggregates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		snap entity.Snapshot
	}{
		{name: "empty frame", snap: entity.Snapshot{}},
		{name: "sample frame", snap: sampleSnapshot()},
		{name: "fallback frame", snap: fallbackSnapshot()},
		{name: "charging frame", snap: chargingSnapshot()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.Config{}
			for _, g := range buildGroups(entity.HWInfo{NumCores: 2}, fullCaps()) {
				if g.aggregate == nil {
					continue
				}

				if got := g.aggregate(tt.snap, cfg); got == "" {
					t.Errorf("%s: empty aggregate", g.label)
				}
			}
		})
	}
}

// The bar prefix matrix: text mode uses the tag, emoji mode keeps the
// qualifier next to the group emoji, gnome mode swaps to icon keys and
// drops the qualifier when the icon itself is metric-specific.
func TestBarPartStyles(t *testing.T) {
	t.Parallel()

	tr := New(config.Load(""), entity.HWInfo{NumCores: 2}, fullCaps())
	snap := sampleSnapshot()

	text := config.Config{BarLabels: config.BarText}
	emoji := config.Config{BarLabels: config.BarVisual, VisualStyle: config.VisualEmoji}
	gnome := config.Config{BarLabels: config.BarVisual, VisualStyle: config.VisualGnome}
	classic := config.Config{BarLabels: config.BarVisual, VisualStyle: config.VisualClassic}

	tests := []struct {
		name     string
		id       entity.MetricID
		cfg      config.Config
		wantIcon string
		prefix   string // expected text before the bar value
	}{
		{"cpu.total text", "cpu.total", text, "", "CPU "},
		{"cpu.total emoji", "cpu.total", emoji, "", "⚙️ "},
		{"cpu.total gnome", "cpu.total", gnome, "gnome/cpu", " "},
		{"cpu.total classic", "cpu.total", classic, "classic/cpu", " "},
		{"net.down text", "net.down", text, "", "↓"},
		{"net.down emoji", "net.down", emoji, "", "📶 ↓"},
		{"net.down gnome", "net.down", gnome, "gnome/network-download", " "}, // own icon → no ↓
		{"net.down classic", "net.down", classic, "classic/network-download", " "},
		{"swap.used text", "swap.used", text, "", "SW "},
		{"swap.used emoji", "swap.used", emoji, "", "🧠 SW "},
		{"swap.used gnome", "swap.used", gnome, "gnome/memory", " SW "}, // group icon → keep SW
		{"swap.used classic", "swap.used", classic, "classic/memory", " SW "},
		{"mem.used text", "mem.used", text, "", ""},
		{"mem.used emoji", "mem.used", emoji, "", "🧠 "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m, ok := tr.bar[tt.id]
			if !ok {
				t.Fatalf("%s missing from the bar map", tt.id)
			}

			icon, got := m.barPart(snap, tt.cfg)
			if icon != tt.wantIcon {
				t.Errorf("%s: icon = %q, want %q", tt.name, icon, tt.wantIcon)
			}

			if want := tt.prefix + m.barValue(snap, tt.cfg); got != want {
				t.Errorf("%s: text = %q, want %q", tt.name, got, want)
			}
		})
	}
}

// New must register every metric in the bar map, including ones that used
// to be unpinnable (disk.free, temp.hottest, …). A single-construction
// invariant, so no table here.
func TestNewRegistersAllMetricsAsPinnable(t *testing.T) {
	t.Parallel()

	tr := New(config.Load(""), entity.HWInfo{NumCores: 2}, fullCaps())

	total := 0
	for _, g := range tr.groups {
		total += len(g.metrics)
	}

	if len(tr.bar) != total {
		t.Errorf("bar has %d metrics, groups have %d", len(tr.bar), total)
	}

	for _, id := range []entity.MetricID{"disk.free", "temp.hottest", "mem.available", "cpu.core.1", "temp.sensor.PMU tdie0", "batt.raw"} {
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
		Battery: &entity.BatteryStats{Percent: 0.87, RawPercent: 0.85, Health: 0.95, Cycles: 120, TempC: 32, Volts: 12.3, Watts: -8.4, MinutesLeft: 185},
		GPU:     &entity.GPUStats{Utilization: 0.33},
		Power:   &entity.PowerStats{CPU: 2.1, GPU: 1.2, ANE: 0.1, Total: 3.4},
		Freq: &entity.FreqStats{
			Clusters: []entity.Reading{{Name: "E-cores", Value: 2.1e9}},
			Max:      3.5e9,
		},
	}
}

// fallbackSnapshot drives the non-nil-but-alternate branches: temps with no
// CPU/GPU aggregate (hottest fallback), a fan with no rated max, and a
// battery on AC with unknown health/raw charge/time.
func fallbackSnapshot() entity.Snapshot {
	return entity.Snapshot{
		CPU:  entity.CPUStats{Total: 0.1, Cores: []float64{0.1, 0.1}},
		Mem:  entity.MemStats{Total: 16 << 30, Used: 4 << 30, Available: 12 << 30},
		Net:  &entity.NetStats{},
		Disk: &entity.DiskStats{DiskUsage: entity.DiskUsage{Total: 100 << 30, Used: 50 << 30, Available: 50 << 30}},
		Temps: &entity.TempStats{
			CPU: 0, GPU: 0, // no aggregate → aggregate falls back to hottest
			Hottest: entity.Reading{Name: "PMU tdie0", Value: 60},
			All:     []entity.Reading{{Name: "PMU tdie0", Value: 60}},
		},
		Volts:   []entity.Reading{{Name: "PMU vbus", Value: 12}},
		Fans:    []entity.Fan{{Name: "Fan 1", RPM: 1200, Max: 0}}, // Max 0 → no load percentage
		Battery: &entity.BatteryStats{Percent: 0.5, Health: 0, Cycles: 0, External: true, Charging: false, MinutesLeft: -1},
		GPU:     &entity.GPUStats{Utilization: 0.1},
		Power:   &entity.PowerStats{},
		Freq:    &entity.FreqStats{Clusters: []entity.Reading{{Name: "E-cores", Value: 1e9}}, Max: 1e9},
	}
}

// chargingSnapshot is the sample frame with the battery charging, to reach
// the "⚡"/"Charging" branches.
func chargingSnapshot() entity.Snapshot {
	s := sampleSnapshot()
	s.Battery.Charging = true

	return s
}

// The charging mark must match the icon set: a monochrome text glyph for the
// gnome (template-icon) style, the color emoji otherwise.
func TestChargeMark(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		style config.VisualStyle
		want  string
	}{
		{name: "gnome uses monochrome bolt", style: config.VisualGnome, want: chargeMarkMono},
		{name: "classic uses monochrome bolt", style: config.VisualClassic, want: chargeMarkMono},
		{name: "emoji uses color bolt", style: config.VisualEmoji, want: chargeMarkEmoji},
		{name: "empty style defaults to emoji", style: "", want: chargeMarkEmoji},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := chargeMark(tt.style); got != tt.want {
				t.Errorf("chargeMark(%q) = %q, want %q", tt.style, got, tt.want)
			}
		})
	}
}
