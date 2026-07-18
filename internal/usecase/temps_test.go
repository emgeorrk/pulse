package usecase

import (
	"testing"

	"github.com/emgeorrk/pulse/internal/entity"
)

func TestAggregateTemps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		all         []entity.Reading
		wantHottest entity.Reading
		wantCoolest entity.Reading
		wantCPU     float64
		wantGPU     float64
		wantAvg     float64
	}{
		{
			name: "apple silicon HID names",
			all: []entity.Reading{
				{Name: "PMU tdie0", Value: 50},
				{Name: "PMU tdie1", Value: 60},
				{Name: "GPU MTR Temp Sensor1", Value: 45},
				{Name: "NAND CH0 temp", Value: 40},
				{Name: "gas gauge battery", Value: 30},
			},
			wantCPU:     55, // average of tdie0+tdie1
			wantGPU:     45,
			wantAvg:     45, // average of all five
			wantHottest: entity.Reading{Name: "PMU tdie1", Value: 60},
			wantCoolest: entity.Reading{Name: "gas gauge battery", Value: 30},
		},
		{
			// M5-style set: HID exposes only generic "PMU tdie*" names, the GPU
			// reading is the averaged SMC Tg* value merged in by sensors.MultiTemp.
			name: "M5 SMC GPU fallback",
			all: []entity.Reading{
				{Name: "PMU tdie1", Value: 35},
				{Name: "PMU tdie2", Value: 36},
				{Name: "PMU tdie3", Value: 37},
				{Name: "GPU die", Value: 42.5},
				{Name: "NAND CH0 temp", Value: 31},
			},
			wantCPU:     36, // average of tdie1..3
			wantGPU:     42.5,
			wantAvg:     36.3,
			wantHottest: entity.Reading{Name: "GPU die", Value: 42.5},
			wantCoolest: entity.Reading{Name: "NAND CH0 temp", Value: 31},
		},
		{
			// no CPU/GPU matches: aggregates stay 0, hottest/coolest still tracked
			name:        "no matches",
			all:         []entity.Reading{{Name: "NAND CH0 temp", Value: 40}},
			wantCPU:     0,
			wantGPU:     0,
			wantAvg:     40,
			wantHottest: entity.Reading{Name: "NAND CH0 temp", Value: 40},
			wantCoolest: entity.Reading{Name: "NAND CH0 temp", Value: 40},
		},
		{
			// Intel path: labels from the curated SMC key list
			name: "intel labels",
			all: []entity.Reading{
				{Name: "CPU proximity", Value: 58},
				{Name: "GPU proximity", Value: 52},
			},
			wantCPU:     58,
			wantGPU:     52,
			wantAvg:     55,
			wantHottest: entity.Reading{Name: "CPU proximity", Value: 58},
			wantCoolest: entity.Reading{Name: "GPU proximity", Value: 52},
		},
		{
			// no sensors at all: everything stays zero
			name: "empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := AggregateTemps(tt.all)

			if !almostEqual(got.CPU, tt.wantCPU) {
				t.Errorf("CPU = %v, want %v", got.CPU, tt.wantCPU)
			}

			if !almostEqual(got.GPU, tt.wantGPU) {
				t.Errorf("GPU = %v, want %v", got.GPU, tt.wantGPU)
			}

			if !almostEqual(got.Avg, tt.wantAvg) {
				t.Errorf("Avg = %v, want %v", got.Avg, tt.wantAvg)
			}

			if got.Hottest.Name != tt.wantHottest.Name || !almostEqual(got.Hottest.Value, tt.wantHottest.Value) {
				t.Errorf("Hottest = %+v, want %+v", got.Hottest, tt.wantHottest)
			}

			if got.Coolest.Name != tt.wantCoolest.Name || !almostEqual(got.Coolest.Value, tt.wantCoolest.Value) {
				t.Errorf("Coolest = %+v, want %+v", got.Coolest, tt.wantCoolest)
			}
		})
	}
}
