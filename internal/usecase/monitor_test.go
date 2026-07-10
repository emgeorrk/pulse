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
		{User: 0, System: 0, Idle: 1100, Nice: 0},   // idle-ядро → 0%
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
	// суммарно: busy 75 из 200 тиков
	if !almostEqual(got.Total, 0.375) {
		t.Errorf("Total = %v, want 0.375", got.Total)
	}
}

func TestCPUUsageTickWraparound(t *testing.T) {
	// счётчик user перевалил через 2^32: дельта в uint32 должна остаться 100
	prev := []entity.CoreTicks{{User: math.MaxUint32 - 49, Idle: 0}}
	cur := []entity.CoreTicks{{User: 50, Idle: 100}}

	got := CPUUsage(prev, cur)

	if !almostEqual(got.Cores[0], 0.5) {
		t.Errorf("Cores[0] = %v, want 0.5", got.Cores[0])
	}
}

func TestCPUUsageEmptyAndMismatched(t *testing.T) {
	if got := CPUUsage(nil, nil); got.Total != 0 || len(got.Cores) != 0 {
		t.Errorf("nil ticks: got %+v, want zero stats", got)
	}

	// разная длина не должна паниковать — берём общий минимум
	prev := []entity.CoreTicks{{Idle: 0}}
	cur := []entity.CoreTicks{{Idle: 100}, {Idle: 100}}
	if got := CPUUsage(prev, cur); len(got.Cores) != 1 {
		t.Errorf("mismatched lengths: len(Cores) = %d, want 1", len(got.Cores))
	}
}
