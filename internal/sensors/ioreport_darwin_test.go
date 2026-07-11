//go:build darwin

package sensors

import (
	"math"
	"testing"
)

func TestWeightedFreq(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		deltas []uint64
		table  []float64
		want   float64
	}{
		{
			name:   "residency-weighted average",
			deltas: []uint64{10, 20},
			table:  []float64{1e9, 2e9},
			want:   (10*1e9 + 20*2e9) / 30, // ≈ 1.667 GHz
		},
		{
			name:   "no residency yields zero",
			deltas: []uint64{0, 0},
			table:  []float64{1e9, 2e9},
			want:   0,
		},
		{
			name:   "table shorter than deltas is clamped to the minimum",
			deltas: []uint64{10, 20, 30},
			table:  []float64{1e9}, // only the first state is weighted
			want:   1e9,
		},
		{
			name:   "empty deltas yield zero",
			deltas: nil,
			table:  []float64{1e9},
			want:   0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := weightedFreq(tt.deltas, tt.table); math.Abs(got-tt.want) > 1 {
				t.Errorf("weightedFreq(%v, %v) = %v, want %v", tt.deltas, tt.table, got, tt.want)
			}
		})
	}
}

func TestJoulesDivisor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		unit string
		want float64
	}{
		{name: "nanojoules", unit: unitNanojoules, want: nanojoulesPerJoule},
		{name: "microjoules ascii", unit: unitMicrojoules, want: microjoulesPerJoule},
		{name: "microjoules micro sign", unit: unitMicroSign, want: microjoulesPerJoule},
		{name: "millijoules", unit: unitMillijoules, want: millijoulesPerJoule},
		{name: "joules", unit: "J", want: 1},
		{name: "surrounding whitespace trimmed", unit: "  " + unitNanojoules + "  ", want: nanojoulesPerJoule},
		{name: "unknown unit defaults to joules", unit: "kWh", want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := joulesDivisor(tt.unit); got != tt.want {
				t.Errorf("joulesDivisor(%q) = %v, want %v", tt.unit, got, tt.want)
			}
		})
	}
}

func TestTableFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		states int
		want   []float64
	}{
		{name: "matches by state count (short)", states: 3, want: []float64{1e9, 2e9, 3e9}},
		{name: "matches by state count (long)", states: 5, want: []float64{1, 2, 3, 4, 5}},
		{name: "no matching table returns nil", states: 7, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &IOReport{tables: [][]float64{{1e9, 2e9, 3e9}, {1, 2, 3, 4, 5}}}

			got := r.tableFor(tt.states)
			if len(got) != len(tt.want) {
				t.Fatalf("tableFor(%d) len = %d, want %d", tt.states, len(got), len(tt.want))
			}

			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("tableFor(%d)[%d] = %v, want %v", tt.states, i, got[i], tt.want[i])
				}
			}
		})
	}
}
