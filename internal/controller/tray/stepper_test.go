//go:build darwin

package tray

import (
	"testing"

	"github.com/emgeorrk/pulse/config"
)

// next owns the clamping, so the table walks both bounds from both sides.
func TestStepperNext(t *testing.T) {
	t.Parallel()

	sp := intervalStepper()

	tests := []struct {
		name    string
		current int
		delta   int
		want    int
	}{
		{name: "increment", current: 2, delta: 1, want: 3},
		{name: "decrement", current: 2, delta: -1, want: 1},
		{name: "decrement at min stays min", current: sp.min, delta: -1, want: sp.min},
		{name: "increment at max stays max", current: sp.max, delta: 1, want: sp.max},
		{name: "large negative delta clamps to min", current: 2, delta: -100, want: sp.min},
		{name: "large positive delta clamps to max", current: 2, delta: 100, want: sp.max},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := sp.next(tt.current, tt.delta); got != tt.want {
				t.Errorf("next(%d, %d) = %d, want %d", tt.current, tt.delta, got, tt.want)
			}
		})
	}
}

// The interval stepper spec owns the whole wiring of the Update interval
// submenu: config bounds, IntervalSec round-trip, and the leaf labels.
func TestIntervalStepper(t *testing.T) {
	t.Parallel()

	sp := intervalStepper()

	if sp.min != config.MinIntervalSeconds || sp.max != config.MaxIntervalSeconds || sp.step != config.IntervalStepSeconds {
		t.Errorf("bounds = %d/%d/%d, want %d/%d/%d", sp.min, sp.max, sp.step,
			config.MinIntervalSeconds, config.MaxIntervalSeconds, config.IntervalStepSeconds)
	}

	var c config.Config

	sp.set(&c, 7)

	if got := sp.get(c); got != 7 || c.IntervalSec != 7 {
		t.Errorf("set/get round-trip: get = %d, IntervalSec = %d, want 7/7", got, c.IntervalSec)
	}

	if got := sp.format(2); got != "2 s" {
		t.Errorf("format(2) = %q, want %q", got, "2 s")
	}

	if inc, dec := stepLabels(sp); inc != "+1 s" || dec != "−1 s" {
		t.Errorf("stepLabels = %q/%q, want %q/%q", inc, dec, "+1 s", "−1 s")
	}
}
