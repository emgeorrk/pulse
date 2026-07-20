package usecase

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/emgeorrk/pulse/internal/entity"
	"github.com/emgeorrk/pulse/internal/sensors"
	"github.com/emgeorrk/pulse/internal/sensors/mocks"
	"go.uber.org/mock/gomock"
)

func almostEqual(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

var errSensor = errors.New("sensor read failed")

func TestCPUUsage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		prev      []entity.CoreTicks
		cur       []entity.CoreTicks
		wantCores []float64
		wantTotal float64
	}{
		{
			name: "busy and idle cores",
			prev: []entity.CoreTicks{
				{User: 100, System: 50, Idle: 850, Nice: 0},
				{User: 0, System: 0, Idle: 1000, Nice: 0},
			},
			cur: []entity.CoreTicks{
				{User: 150, System: 75, Idle: 875, Nice: 0}, // busy 75, idle 25 → 75%
				{User: 0, System: 0, Idle: 1100, Nice: 0},   // idle core → 0%
			},
			wantCores: []float64{0.75, 0},
			wantTotal: 0.375, // overall: busy 75 out of 200 ticks
		},
		{
			// the user counter wrapped past 2^32: the uint32 delta should stay 100
			name:      "tick wraparound",
			prev:      []entity.CoreTicks{{User: math.MaxUint32 - 49, Idle: 0}},
			cur:       []entity.CoreTicks{{User: 50, Idle: 100}},
			wantCores: []float64{0.5},
			wantTotal: 0.5,
		},
		{
			name:      "nil ticks give zero stats",
			prev:      nil,
			cur:       nil,
			wantCores: []float64{},
			wantTotal: 0,
		},
		{
			// mismatched lengths must not panic — take the common minimum
			name:      "mismatched lengths",
			prev:      []entity.CoreTicks{{Idle: 0}},
			cur:       []entity.CoreTicks{{Idle: 100}, {Idle: 100}},
			wantCores: []float64{0},
			wantTotal: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := CPUUsage(tt.prev, tt.cur)

			if len(got.Cores) != len(tt.wantCores) {
				t.Fatalf("len(Cores) = %d, want %d", len(got.Cores), len(tt.wantCores))
			}

			for i, want := range tt.wantCores {
				if !almostEqual(got.Cores[i], want) {
					t.Errorf("Cores[%d] = %v, want %v", i, got.Cores[i], want)
				}
			}

			if !almostEqual(got.Total, tt.wantTotal) {
				t.Errorf("Total = %v, want %v", got.Total, tt.wantTotal)
			}
		})
	}
}

func TestNetRates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		prev       map[string]entity.NetCounters
		cur        []entity.NetCounters
		dwellSec   float64
		wantDown   float64
		wantUp     float64
		wantIfaces int
	}{
		{
			name:       "deltas divided by dwell",
			prev:       map[string]entity.NetCounters{"en0": {Name: "en0", Rx: 1000, Tx: 500}},
			cur:        []entity.NetCounters{{Name: "en0", Rx: 3000, Tx: 1500}}, // +2000 rx, +1000 tx over 2s
			dwellSec:   2,
			wantDown:   1000,
			wantUp:     500,
			wantIfaces: 1,
		},
		{
			// Rx wrapped past 2^32: the uint32 delta should stay 200
			name:       "counter wraparound",
			prev:       map[string]entity.NetCounters{"en0": {Name: "en0", Rx: math.MaxUint32 - 99, Tx: 0}},
			cur:        []entity.NetCounters{{Name: "en0", Rx: 100, Tx: 0}},
			dwellSec:   2,
			wantDown:   100,
			wantUp:     0,
			wantIfaces: 1,
		},
		{
			name:       "new interface skipped until the next tick",
			prev:       map[string]entity.NetCounters{},
			cur:        []entity.NetCounters{{Name: "utun9", Rx: 999, Tx: 999}},
			dwellSec:   2,
			wantDown:   0,
			wantUp:     0,
			wantIfaces: 0,
		},
		{
			name:       "idle interface not listed",
			prev:       map[string]entity.NetCounters{"en0": {Name: "en0", Rx: 1000, Tx: 500}},
			cur:        []entity.NetCounters{{Name: "en0", Rx: 1000, Tx: 500}},
			dwellSec:   2,
			wantDown:   0,
			wantUp:     0,
			wantIfaces: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NetRates(tt.prev, tt.cur, tt.dwellSec)

			if !almostEqual(got.Down, tt.wantDown) {
				t.Errorf("Down = %v, want %v", got.Down, tt.wantDown)
			}

			if !almostEqual(got.Up, tt.wantUp) {
				t.Errorf("Up = %v, want %v", got.Up, tt.wantUp)
			}

			if len(got.Ifaces) != tt.wantIfaces {
				t.Errorf("len(Ifaces) = %d (%v), want %d", len(got.Ifaces), got.Ifaces, tt.wantIfaces)
			}
		})
	}
}

func TestSample(t *testing.T) {
	t.Parallel()

	tests := []struct {
		build func(ctrl *gomock.Controller) *Monitor
		check func(t *testing.T, snap entity.Snapshot)
		name  string
	}{
		{
			name:  "all sources populate the snapshot",
			build: buildFullMonitor,
			check: func(t *testing.T, snap entity.Snapshot) {
				t.Helper()

				if snap.CPU.Total <= 0 || len(snap.CPU.History) == 0 {
					t.Errorf("CPU = %+v, want load > 0 with history", snap.CPU)
				}

				if snap.Mem.Total == 0 {
					t.Error("Mem not populated")
				}

				if snap.Net == nil || snap.Net.Down <= 0 {
					t.Errorf("Net = %+v, want download rate", snap.Net)
				}

				if snap.Disk == nil || snap.Disk.ReadRate <= 0 {
					t.Errorf("Disk = %+v, want read rate", snap.Disk)
				}

				if snap.Temps == nil || snap.Temps.CPU == 0 {
					t.Errorf("Temps = %+v, want CPU aggregate", snap.Temps)
				}

				for name, ptr := range map[string]any{
					"Fans": snap.Fans, "Battery": snap.Battery,
					"GPU": snap.GPU, "Power": snap.Power, "Freq": snap.Freq,
				} {
					if isNilPtrOrSlice(ptr) {
						t.Errorf("%s not populated", name)
					}
				}
			},
		},
		{
			name: "nil optional sources leave the snapshot empty",
			build: func(ctrl *gomock.Controller) *Monitor {
				cpu := mocks.NewMockCPUSource(ctrl)
				cpu.EXPECT().Ticks().Return([]entity.CoreTicks{{User: 200, Idle: 800}}, nil)

				mem := mocks.NewMockMemSource(ctrl)
				mem.EXPECT().Read().Return(entity.MemStats{Total: 16 << 30, Used: 8 << 30}, nil)

				return &Monitor{
					src:      &sensors.Sources{CPU: cpu, Mem: mem},
					lastTick: time.Now().Add(-time.Second),
				}
			},
			check: func(t *testing.T, snap entity.Snapshot) {
				t.Helper()

				if snap.Mem.Total == 0 {
					t.Error("Mem not populated")
				}

				if snap.Net != nil || snap.Disk != nil || snap.Temps != nil ||
					snap.Battery != nil || snap.GPU != nil || snap.Power != nil ||
					snap.Freq != nil || snap.Fans != nil {
					t.Errorf("optional fields should stay empty: %+v", snap)
				}
			},
		},
		{
			name: "source errors are skipped without panic",
			build: func(ctrl *gomock.Controller) *Monitor {
				cpu := mocks.NewMockCPUSource(ctrl)
				cpu.EXPECT().Ticks().Return(nil, errSensor)

				mem := mocks.NewMockMemSource(ctrl)
				mem.EXPECT().Read().Return(entity.MemStats{}, errSensor)

				net := mocks.NewMockNetSource(ctrl)
				net.EXPECT().Counters().Return(nil, errSensor)

				disk := mocks.NewMockDiskSource(ctrl)
				disk.EXPECT().Usage().Return(entity.DiskUsage{}, errSensor)

				temp := mocks.NewMockTempSource(ctrl)
				temp.EXPECT().Temps().Return(nil, errSensor)

				fan := mocks.NewMockFanSource(ctrl)
				fan.EXPECT().Fans().Return(nil, errSensor)

				batt := mocks.NewMockBatterySource(ctrl)
				batt.EXPECT().Battery().Return(entity.BatteryStats{}, errSensor)

				gpu := mocks.NewMockGPUSource(ctrl)
				gpu.EXPECT().GPU().Return(entity.GPUStats{}, errSensor)

				power := mocks.NewMockPowerSource(ctrl)
				power.EXPECT().Power().Return(entity.PowerStats{}, errSensor)

				freq := mocks.NewMockFreqSource(ctrl)
				freq.EXPECT().Frequency().Return(entity.FreqStats{}, errSensor)

				return &Monitor{
					src: &sensors.Sources{
						CPU: cpu, Mem: mem, Net: net, Disk: disk, Temp: temp,
						Fan: fan, Battery: batt, GPU: gpu, Power: power, Freq: freq,
					},
					prevNet:  map[string]entity.NetCounters{"en0": {Name: "en0"}},
					haveDisk: true,
					lastTick: time.Now().Add(-time.Second),
				}
			},
			check: func(t *testing.T, snap entity.Snapshot) {
				t.Helper()

				if snap.Net != nil || snap.Disk != nil || snap.Temps != nil ||
					snap.Battery != nil || snap.GPU != nil || snap.Power != nil || snap.Freq != nil {
					t.Errorf("failing sources should be skipped: %+v", snap)
				}
			},
		},
		{
			name: "non-positive dwell is clamped to one second",
			build: func(ctrl *gomock.Controller) *Monitor {
				cpu := mocks.NewMockCPUSource(ctrl)
				cpu.EXPECT().Ticks().Return([]entity.CoreTicks{{Idle: 100}}, nil)

				mem := mocks.NewMockMemSource(ctrl)
				mem.EXPECT().Read().Return(entity.MemStats{Total: 1}, nil)

				net := mocks.NewMockNetSource(ctrl)
				net.EXPECT().Counters().Return([]entity.NetCounters{{Name: "en0", Rx: 3000, Tx: 1000}}, nil)

				return &Monitor{
					src:      &sensors.Sources{CPU: cpu, Mem: mem, Net: net},
					prevNet:  map[string]entity.NetCounters{"en0": {Name: "en0", Rx: 1000, Tx: 500}},
					lastTick: time.Now().Add(time.Hour), // future tick → negative dwell
				}
			},
			check: func(t *testing.T, snap entity.Snapshot) {
				t.Helper()

				// dwell clamped to 1s, so rate == raw delta (2000 down, 500 up).
				if snap.Net == nil || !almostEqual(snap.Net.Down, 2000) || !almostEqual(snap.Net.Up, 500) {
					t.Errorf("Net = %+v, want Down 2000 Up 500 (dwell clamped)", snap.Net)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			m := tt.build(ctrl)
			tt.check(t, m.sample(context.Background()))
		})
	}
}

// buildFullMonitor wires every source with a mock and primes the delta
// baselines so one sample() produces meaningful rates.
func buildFullMonitor(ctrl *gomock.Controller) *Monitor {
	cpu := mocks.NewMockCPUSource(ctrl)
	cpu.EXPECT().Ticks().Return([]entity.CoreTicks{{User: 300, Idle: 900}}, nil)

	mem := mocks.NewMockMemSource(ctrl)
	mem.EXPECT().Read().Return(entity.MemStats{Total: 16 << 30, Used: 8 << 30}, nil)

	net := mocks.NewMockNetSource(ctrl)
	net.EXPECT().Counters().Return([]entity.NetCounters{{Name: "en0", Rx: 3000, Tx: 1500}}, nil)

	disk := mocks.NewMockDiskSource(ctrl)
	disk.EXPECT().Usage().Return(entity.DiskUsage{Total: 500, Used: 300}, nil)
	disk.EXPECT().IOTotals().Return(uint64(2000), uint64(1000), nil)

	temp := mocks.NewMockTempSource(ctrl)
	temp.EXPECT().Temps().Return([]entity.Reading{{Name: "PMU tdie0", Value: 55}}, nil)

	fan := mocks.NewMockFanSource(ctrl)
	fan.EXPECT().Fans().Return([]entity.Fan{{Name: "Fan 1", RPM: 1800, Max: 5000}}, nil)

	batt := mocks.NewMockBatterySource(ctrl)
	batt.EXPECT().Battery().Return(entity.BatteryStats{Percent: 0.9}, nil)

	gpu := mocks.NewMockGPUSource(ctrl)
	gpu.EXPECT().GPU().Return(entity.GPUStats{Utilization: 0.3}, nil)

	power := mocks.NewMockPowerSource(ctrl)
	power.EXPECT().Power().Return(entity.PowerStats{Total: 3.4}, nil)

	freq := mocks.NewMockFreqSource(ctrl)
	freq.EXPECT().Frequency().Return(entity.FreqStats{Max: 3.5e9}, nil)

	return &Monitor{
		src: &sensors.Sources{
			CPU: cpu, Mem: mem, Net: net, Disk: disk, Temp: temp,
			Fan: fan, Battery: batt, GPU: gpu, Power: power, Freq: freq,
		},
		prevTicks: []entity.CoreTicks{{User: 100, Idle: 700}},
		prevNet:   map[string]entity.NetCounters{"en0": {Name: "en0", Rx: 1000, Tx: 500}},
		prevRead:  1000,
		prevWrite: 500,
		haveDisk:  true,
		lastTick:  time.Now().Add(-time.Second),
	}
}

// isNilPtrOrSlice reports whether an interface holds a nil pointer or an
// empty slice — used to assert optional snapshot fields were populated.
func isNilPtrOrSlice(v any) bool {
	switch t := v.(type) {
	case []entity.Fan:
		return len(t) == 0
	case *entity.BatteryStats:
		return t == nil
	case *entity.GPUStats:
		return t == nil
	case *entity.PowerStats:
		return t == nil
	case *entity.FreqStats:
		return t == nil
	default:
		return true
	}
}

func TestCountersMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		counters []entity.NetCounters
		wantLen  int
	}{
		{
			name:     "indexes counters by interface name",
			counters: []entity.NetCounters{{Name: "en0", Rx: 1}, {Name: "en1", Rx: 2}},
			wantLen:  2,
		},
		{name: "empty input gives empty map", counters: nil, wantLen: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := countersMap(tt.counters)
			if len(got) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(got), tt.wantLen)
			}

			for _, c := range tt.counters {
				if got[c.Name] != c {
					t.Errorf("got[%q] = %+v, want %+v", c.Name, got[c.Name], c)
				}
			}
		})
	}
}

func TestRate64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		prev     uint64
		cur      uint64
		dwellSec float64
		want     float64
	}{
		{name: "delta divided by dwell", prev: 100, cur: 300, dwellSec: 2, want: 100},
		{name: "counter reset never goes negative", prev: 300, cur: 100, dwellSec: 2, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := rate64(tt.prev, tt.cur, tt.dwellSec); !almostEqual(got, tt.want) {
				t.Errorf("rate64(%d, %d, %v) = %v, want %v", tt.prev, tt.cur, tt.dwellSec, got, tt.want)
			}
		})
	}
}
