package usecase

import (
	"math"
	"testing"

	"github.com/emgeorrk/pulse/internal/entity"
)

func almostEqual(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

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
