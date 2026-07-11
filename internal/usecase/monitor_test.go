package usecase

import (
	"math"
	"testing"

	"github.com/emgeorrk/pulse/internal/entity"
)

func almostEqual(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestCPUUsage(t *testing.T) {
	prev := []entity.CoreTicks{
		{User: 100, System: 50, Idle: 850, Nice: 0},
		{User: 0, System: 0, Idle: 1000, Nice: 0},
	}
	cur := []entity.CoreTicks{
		{User: 150, System: 75, Idle: 875, Nice: 0}, // busy 75, idle 25 → 75%
		{User: 0, System: 0, Idle: 1100, Nice: 0},   // idle core → 0%
	}

	got := CPUUsage(prev, cur)

	if len(got.Cores) != 2 {
		t.Fatalf("len(Cores) = %d, want 2", len(got.Cores))
	}
	if !almostEqual(got.Cores[0], 0.75) {
		t.Errorf("Cores[0] = %v, want 0.75", got.Cores[0])
	}
	if !almostEqual(got.Cores[1], 0) {
		t.Errorf("Cores[1] = %v, want 0", got.Cores[1])
	}
	// overall: busy 75 out of 200 ticks
	if !almostEqual(got.Total, 0.375) {
		t.Errorf("Total = %v, want 0.375", got.Total)
	}
}

func TestCPUUsageTickWraparound(t *testing.T) {
	// the user counter wrapped past 2^32: the uint32 delta should stay 100
	prev := []entity.CoreTicks{{User: math.MaxUint32 - 49, Idle: 0}}
	cur := []entity.CoreTicks{{User: 50, Idle: 100}}

	got := CPUUsage(prev, cur)

	if !almostEqual(got.Cores[0], 0.5) {
		t.Errorf("Cores[0] = %v, want 0.5", got.Cores[0])
	}
}

func TestNetRates(t *testing.T) {
	prev := map[string]entity.NetCounters{
		"en0":  {Name: "en0", Rx: 1000, Tx: 500},
		"wrap": {Name: "wrap", Rx: math.MaxUint32 - 99, Tx: 0},
	}
	cur := []entity.NetCounters{
		{Name: "en0", Rx: 3000, Tx: 1500}, // +2000 rx, +1000 tx over 2s
		{Name: "wrap", Rx: 100, Tx: 0},    // overflow: delta 200
		{Name: "utun9", Rx: 999, Tx: 999}, // new interface — skipped until the next tick
	}

	got := NetRates(prev, cur, 2)

	if !almostEqual(got.Down, (2000+200)/2.0) {
		t.Errorf("Down = %v, want 1100", got.Down)
	}
	if !almostEqual(got.Up, 500) {
		t.Errorf("Up = %v, want 500", got.Up)
	}
	if len(got.Ifaces) != 2 {
		t.Errorf("Ifaces = %v, want en0+wrap only", got.Ifaces)
	}
}

func TestRate64(t *testing.T) {
	if got := rate64(100, 300, 2); !almostEqual(got, 100) {
		t.Errorf("rate64 = %v, want 100", got)
	}
	if got := rate64(300, 100, 2); got != 0 {
		t.Errorf("rate64 after reset = %v, want 0", got)
	}
}

func TestCPUUsageEmptyAndMismatched(t *testing.T) {
	if got := CPUUsage(nil, nil); got.Total != 0 || len(got.Cores) != 0 {
		t.Errorf("nil ticks: got %+v, want zero stats", got)
	}

	// mismatched lengths must not panic — take the common minimum
	prev := []entity.CoreTicks{{Idle: 0}}
	cur := []entity.CoreTicks{{Idle: 100}, {Idle: 100}}
	if got := CPUUsage(prev, cur); len(got.Cores) != 1 {
		t.Errorf("mismatched lengths: len(Cores) = %d, want 1", len(got.Cores))
	}
}
