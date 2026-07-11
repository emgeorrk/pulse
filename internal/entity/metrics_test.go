package entity

import (
	"math"
	"testing"
)

func TestMemStatsUsedFraction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mem  MemStats
		want float64
	}{
		{name: "half used", mem: MemStats{Total: 16 << 30, Used: 8 << 30}, want: 0.5},
		{name: "fully used", mem: MemStats{Total: 16 << 30, Used: 16 << 30}, want: 1},
		{name: "zero total avoids divide by zero", mem: MemStats{Total: 0, Used: 8 << 30}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.mem.UsedFraction(); math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("UsedFraction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiskUsageUsedFraction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		disk DiskUsage
		want float64
	}{
		{name: "60 percent used", disk: DiskUsage{Total: 500 << 30, Used: 300 << 30}, want: 0.6},
		{name: "empty volume", disk: DiskUsage{Total: 500 << 30, Used: 0}, want: 0},
		{name: "zero total avoids divide by zero", disk: DiskUsage{Total: 0, Used: 300 << 30}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.disk.UsedFraction(); math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("UsedFraction() = %v, want %v", got, tt.want)
			}
		})
	}
}
