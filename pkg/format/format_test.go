package format

import (
	"fmt"
	"testing"
)

func TestPercent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   float64
		want string
	}{
		{0, "0%"},
		{0.07, "7%"},
		{0.074, "7%"},
		{0.076, "8%"},
		{1, "100%"},
		{1.5, "100%"}, // clamped from above, like Vitals
		{-0.1, "0%"},  // garbage below zero isn't shown
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.in), func(t *testing.T) {
			t.Parallel()

			if got := Percent(tt.in); got != tt.want {
				t.Errorf("Percent(%v) = %q, want %q", tt.in, got, tt.want)
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
		want    string
	}{
		{0, false, "0 B"},
		{512, false, "512 B"},
		{1024, false, "1.0 KiB"},
		{26092059034, false, "24.3 GiB"}, // ≈24.3 GiB
		{48 * gib, false, "48.0 GiB"},
		{1048525, false, "1.0 MiB"}, // 1023.95 KiB → shown in the next unit up, not "1024.0 KiB"
		{48 * gib, true, "51.5 GB"}, // decimal mode
		{1000, true, "1.0 KB"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d decimal=%t", tt.in, tt.decimal), func(t *testing.T) {
			t.Parallel()

			if got := Bytes(tt.in, tt.decimal); got != tt.want {
				t.Errorf("Bytes(%d, %v) = %q, want %q", tt.in, tt.decimal, got, tt.want)
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

			if got := BytesShort(tt.in, false); got != tt.want {
				t.Errorf("BytesShort(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSpeed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   float64
		want string
	}{
		{0, "0 B/s"},
		{-5, "0 B/s"}, // a negative delta (counter reset) shouldn't alarm the user
		{1200, "1.2 KB/s"},
		{1340000, "1.3 MB/s"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.in), func(t *testing.T) {
			t.Parallel()

			if got := Speed(tt.in); got != tt.want {
				t.Errorf("Speed(%v) = %q, want %q", tt.in, got, tt.want)
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

			if got := SpeedShort(tt.in); got != tt.want {
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
		want       string
	}{
		{54.4, false, "54°C"},
		{100, true, "212°F"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			if got := Temp(tt.in, tt.fahrenheit); got != tt.want {
				t.Errorf("Temp(%v, %v) = %q, want %q", tt.in, tt.fahrenheit, got, tt.want)
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

			if got := TempShort(tt.in, tt.fahrenheit); got != tt.want {
				t.Errorf("TempShort(%v, %v) = %q, want %q", tt.in, tt.fahrenheit, got, tt.want)
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
