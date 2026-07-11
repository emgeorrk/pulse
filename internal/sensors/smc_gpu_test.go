package sensors

import (
	"strings"
	"testing"
)

func TestIsGPUTempKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key  string
		want bool
	}{
		{"Tg0d", true},
		{"Tg1c", true},
		{"Tg0U", true},
		{"TC0P", false}, // CPU proximity
		{"Ts0P", false}, // palm rest
		{"TG0D", false}, // Intel GPU die — curated Intel path, not this one
		{"Tg0", false},  // not a 4-char key
		{"#KEY", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()

			if got := isGPUTempKey(tt.key); got != tt.want {
				t.Errorf("isGPUTempKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestSMCKeyString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key  uint32
		want string
	}{
		{0x54673064, "Tg0d"},
		{0x54433050, "TC0P"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			if got := smcKeyString(tt.key); got != tt.want {
				t.Errorf("smcKeyString(%#x) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestGPUTempSensorName(t *testing.T) {
	t.Parallel()

	// AggregateTemps matches GPU sensors by the lowercase "gpu" substring —
	// the label must keep satisfying that. A single invariant on a constant,
	// so no table here.
	if got := strings.ToLower(gpuTempSensorName); !strings.Contains(got, "gpu") {
		t.Errorf("gpuTempSensorName = %q, must contain \"gpu\" (case-insensitive)", gpuTempSensorName)
	}
}
