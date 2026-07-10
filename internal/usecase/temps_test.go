package usecase

import (
	"testing"

	"github.com/emgeorrk/pulse/internal/entity"
)

func TestAggregateTemps(t *testing.T) {
	all := []entity.Reading{
		{Name: "PMU tdie0", Value: 50},
		{Name: "PMU tdie1", Value: 60},
		{Name: "GPU MTR Temp Sensor1", Value: 45},
		{Name: "NAND CH0 temp", Value: 40},
		{Name: "gas gauge battery", Value: 30},
	}

	got := AggregateTemps(all)

	if !almostEqual(got.CPU, 55) {
		t.Errorf("CPU = %v, want 55 (среднее tdie0+tdie1)", got.CPU)
	}
	if !almostEqual(got.GPU, 45) {
		t.Errorf("GPU = %v, want 45", got.GPU)
	}
	if got.Hottest.Name != "PMU tdie1" || !almostEqual(got.Hottest.Value, 60) {
		t.Errorf("Hottest = %+v, want tdie1/60", got.Hottest)
	}
}

func TestAggregateTempsNoMatches(t *testing.T) {
	got := AggregateTemps([]entity.Reading{{Name: "NAND CH0 temp", Value: 40}})
	if got.CPU != 0 || got.GPU != 0 {
		t.Errorf("агрегаты должны остаться 0: %+v", got)
	}
	if got.Hottest.Value != 40 {
		t.Errorf("Hottest = %+v", got.Hottest)
	}
}

func TestAggregateTempsIntelLabels(t *testing.T) {
	// Intel-путь: метки из курируемого списка SMC-ключей
	got := AggregateTemps([]entity.Reading{
		{Name: "CPU proximity", Value: 58},
		{Name: "GPU proximity", Value: 52},
	})
	if !almostEqual(got.CPU, 58) || !almostEqual(got.GPU, 52) {
		t.Errorf("Intel labels: %+v", got)
	}
}
