//go:build darwin

package tray

import (
	"testing"

	"github.com/emgeorrk/pulse/config"
)

// parseInterval owns the whole typed-input policy: reject non-numbers,
// clamp numbers into the config bounds.
func TestParseInterval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		want   int
		wantOK bool
	}{
		{name: "plain number", input: "2", want: 2, wantOK: true},
		{name: "surrounding spaces", input: " 10 ", want: 10, wantOK: true},
		{name: "exact min", input: "1", want: config.MinIntervalSeconds, wantOK: true},
		{name: "exact max", input: "60", want: config.MaxIntervalSeconds, wantOK: true},
		{name: "zero clamps to min", input: "0", want: config.MinIntervalSeconds, wantOK: true},
		{name: "negative clamps to min", input: "-5", want: config.MinIntervalSeconds, wantOK: true},
		{name: "above max clamps to max", input: "999", want: config.MaxIntervalSeconds, wantOK: true},
		{name: "explicit plus sign", input: "+5", want: 5, wantOK: true},
		{name: "empty rejected", input: "", wantOK: false},
		{name: "letters rejected", input: "abc", wantOK: false},
		{name: "decimal rejected", input: "2.5", wantOK: false},
		{name: "scientific rejected", input: "1e3", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseInterval(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("parseInterval(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}

			if ok && got != tt.want {
				t.Errorf("parseInterval(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
