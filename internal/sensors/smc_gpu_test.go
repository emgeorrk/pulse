package sensors

import (
	"strings"
	"testing"
)

func TestIsGPUTempKey(t *testing.T) {
	cases := []struct {
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
	for _, c := range cases {
		if got := isGPUTempKey(c.key); got != c.want {
			t.Errorf("isGPUTempKey(%q) = %v, want %v", c.key, got, c.want)
		}
	}
}

func TestSMCKeyString(t *testing.T) {
	if got := smcKeyString(0x54673064); got != "Tg0d" {
		t.Errorf("smcKeyString(0x54673064) = %q, want \"Tg0d\"", got)
	}
}

func TestGPUTempSensorName(t *testing.T) {
	// AggregateTemps matches GPU sensors by the lowercase "gpu" substring —
	// the label must keep satisfying that.
	if got := strings.ToLower(gpuTempSensorName); !strings.Contains(got, "gpu") {
		t.Errorf("gpuTempSensorName = %q, must contain \"gpu\" (case-insensitive)", gpuTempSensorName)
	}
}
