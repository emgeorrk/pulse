package format

import (
	"fmt"
	"testing"
)

func TestPercent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in      float64
		precise bool
		want    string
	}{
		{0, false, "0%"},
		{0.07, false, "7%"},
		{0.074, false, "7%"},
		{0.076, false, "8%"},
		{1, false, "100%"},
		{1.5, false, "100%"}, // clamped from above, like Vitals
		{-0.1, false, "0%"},  // garbage below zero isn't shown
		{0.074, true, "7.4%"},
		{1.5, true, "100.0%"},
		{-0.1, true, "0.0%"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v precise=%t", tt.in, tt.precise), func(t *testing.T) {
			t.Parallel()

			if got := Percent(tt.in, tt.precise); got != tt.want {
				t.Errorf("Percent(%v, %t) = %q, want %q", tt.in, tt.precise, got, tt.want)
			}
		})
	}
}

func TestBytes(t *testing.T) {
	t.Parallel()

	const gib = 1024 * 1024 * 1024

	tests := []struct {
		in      uint64
		decimal bool
		precise bool
		want    string
	}{
		{0, false, false, "0 B"},
		{512, false, false, "512 B"},
		{1024, false, false, "1.0 KiB"},
		{26092059034, false, false, "24.3 GiB"}, // ≈24.3 GiB
		{48 * gib, false, false, "48.0 GiB"},
		{1048525, false, false, "1.0 MiB"}, // 1023.95 KiB → shown in the next unit up, not "1024.0 KiB"
		{48 * gib, true, false, "51.5 GB"}, // decimal mode
		{1000, true, false, "1.0 KB"},
		{26092059034, false, true, "24.30 GiB"}, // higher precision adds a digit
		{512, false, true, "512 B"},             // raw bytes stay integer
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d decimal=%t precise=%t", tt.in, tt.decimal, tt.precise), func(t *testing.T) {
			t.Parallel()

			if got := Bytes(tt.in, tt.decimal, tt.precise); got != tt.want {
				t.Errorf("Bytes(%d, %v, %v) = %q, want %q", tt.in, tt.decimal, tt.precise, got, tt.want)
			}
		})
	}
}

func TestBytesShort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   uint64
		want string
	}{
		{0, "0B"},
		{26092059034, "24.3G"}, // ≈24.3 GiB
		{2 * 1024 * 1024, "2.0M"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.in), func(t *testing.T) {
			t.Parallel()

			if got := BytesShort(tt.in, false, false); got != tt.want {
				t.Errorf("BytesShort(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSpeed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in      float64
		precise bool
		want    string
	}{
		{0, false, "0 B/s"},
		{-5, false, "0 B/s"}, // a negative delta (counter reset) shouldn't alarm the user
		{1200, false, "1.2 KB/s"},
		{1340000, false, "1.3 MB/s"},
		{1340000, true, "1.34 MB/s"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v precise=%t", tt.in, tt.precise), func(t *testing.T) {
			t.Parallel()

			if got := Speed(tt.in, tt.precise); got != tt.want {
				t.Errorf("Speed(%v, %t) = %q, want %q", tt.in, tt.precise, got, tt.want)
			}
		})
	}
}

func TestSpeedShort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   float64
		want string
	}{
		{0, "0B/s"},
		{1340000, "1.3M/s"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.in), func(t *testing.T) {
			t.Parallel()

			if got := SpeedShort(tt.in, false); got != tt.want {
				t.Errorf("SpeedShort(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestTemp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in         float64
		fahrenheit bool
		precise    bool
		want       string
	}{
		{54.4, false, false, "54°C"},
		{100, true, false, "212°F"},
		{54.42, false, true, "54.4°C"},
		{100, true, true, "212.0°F"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			if got := Temp(tt.in, tt.fahrenheit, tt.precise); got != tt.want {
				t.Errorf("Temp(%v, %v, %v) = %q, want %q", tt.in, tt.fahrenheit, tt.precise, got, tt.want)
			}
		})
	}
}

func TestTempShort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in         float64
		fahrenheit bool
		want       string
	}{
		{54.4, false, "54°"},
		{100, true, "212°"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v fahrenheit=%t", tt.in, tt.fahrenheit), func(t *testing.T) {
			t.Parallel()

			if got := TempShort(tt.in, tt.fahrenheit, false); got != tt.want {
				t.Errorf("TempShort(%v, %v) = %q, want %q", tt.in, tt.fahrenheit, got, tt.want)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   float64
		want string
	}{
		{0, "0.00"},
		{1.4249, "1.42"},
		{12.5, "12.50"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.in), func(t *testing.T) {
			t.Parallel()

			if got := Load(tt.in); got != tt.want {
				t.Errorf("Load(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{"NL", "🇳🇱"},
		{"us", "🇺🇸"},
		{"", ""},
		{"X", ""},
		{"USA", ""},
		{"1A", ""},
		{"р", ""}, // one two-byte rune, len 2 but not ASCII letters
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.in), func(t *testing.T) {
			t.Parallel()

			if got := Flag(tt.in); got != tt.want {
				t.Errorf("Flag(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestWithFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		v    string
		cc   string
		want string
	}{
		{"203.0.113.7", "NL", "203.0.113.7 🇳🇱"},
		{"203.0.113.7", "", "203.0.113.7"},
		{"203.0.113.7", "XX?", "203.0.113.7"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s+%q", tt.v, tt.cc), func(t *testing.T) {
			t.Parallel()

			if got := WithFlag(tt.v, tt.cc); got != tt.want {
				t.Errorf("WithFlag(%q, %q) = %q, want %q", tt.v, tt.cc, got, tt.want)
			}
		})
	}
}

func TestUptime(t *testing.T) {
	t.Parallel()

	const (
		minute = 60
		hour   = 60 * minute
		day    = 24 * hour
	)

	tests := []struct {
		in   uint64
		want string
	}{
		{0, "0m"},
		{59, "0m"}, // sub-minute uptime rounds down
		{12 * minute, "12m"},
		{4*hour + 12*minute, "4h 12m"},
		{3*day + 4*hour + 12*minute, "3d 4h 12m"},
		{3 * day, "3d 0h 0m"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			if got := Uptime(tt.in); got != tt.want {
				t.Errorf("Uptime(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestHertz(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   float64
		want string
	}{
		{3500000000, "3.5 GHz"},
		{0, "0 Hz"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.in), func(t *testing.T) {
			t.Parallel()

			if got := Hertz(tt.in); got != tt.want {
				t.Errorf("Hertz(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSparkline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []float64
		want string
	}{
		{name: "empty history", in: nil, want: ""},
		// 0 → bottom block, 1 → top block, outliers are clamped
		{name: "clamped values", in: []float64{0, 0.5, 1, 2, -1}, want: "▁▅██▁"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := Sparkline(tt.in); got != tt.want {
				t.Errorf("Sparkline(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
