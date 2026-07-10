package format

import "testing"

func TestPercent(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0%"},
		{0.07, "7%"},
		{0.074, "7%"},
		{0.076, "8%"},
		{1, "100%"},
		{1.5, "100%"}, // зажим сверху, как в Vitals
		{-0.1, "0%"},  // мусор снизу не показываем
	}
	for _, c := range cases {
		if got := Percent(c.in); got != c.want {
			t.Errorf("Percent(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBytes(t *testing.T) {
	const gib = 1024 * 1024 * 1024
	cases := []struct {
		in      uint64
		decimal bool
		want    string
	}{
		{0, false, "0 B"},
		{512, false, "512 B"},
		{1024, false, "1.0 KiB"},
		{26092059034, false, "24.3 GiB"}, // ≈24.3 GiB
		{48 * gib, false, "48.0 GiB"},
		{1048525, false, "1.0 MiB"}, // 1023.95 KiB → показываем в следующей единице, а не "1024.0 KiB"
		{48 * gib, true, "51.5 GB"}, // decimal-режим
		{1000, true, "1.0 KB"},
	}
	for _, c := range cases {
		if got := Bytes(c.in, c.decimal); got != c.want {
			t.Errorf("Bytes(%d, %v) = %q, want %q", c.in, c.decimal, got, c.want)
		}
	}
}

func TestBytesShort(t *testing.T) {
	cases := []struct {
		in   uint64
		want string
	}{
		{0, "0B"},
		{26092059034, "24.3G"}, // ≈24.3 GiB
		{2 * 1024 * 1024, "2.0M"},
	}
	for _, c := range cases {
		if got := BytesShort(c.in, false); got != c.want {
			t.Errorf("BytesShort(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSpeed(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0 B/s"},
		{-5, "0 B/s"}, // отрицательная дельта (сброс счётчика) не пугает пользователя
		{1200, "1.2 KB/s"},
		{1340000, "1.3 MB/s"},
	}
	for _, c := range cases {
		if got := Speed(c.in); got != c.want {
			t.Errorf("Speed(%v) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := SpeedShort(1340000); got != "1.3M/s" {
		t.Errorf("SpeedShort = %q, want 1.3M/s", got)
	}
}

func TestTemp(t *testing.T) {
	if got := Temp(54.4, false); got != "54°C" {
		t.Errorf("Temp C = %q", got)
	}
	if got := Temp(100, true); got != "212°F" {
		t.Errorf("Temp F = %q", got)
	}
	if got := TempShort(54.4, false); got != "54°" {
		t.Errorf("TempShort = %q", got)
	}
}

func TestHertz(t *testing.T) {
	if got := Hertz(3500000000); got != "3.5 GHz" {
		t.Errorf("Hertz = %q", got)
	}
	if got := Hertz(0); got != "0 Hz" {
		t.Errorf("Hertz(0) = %q", got)
	}
}

func TestSparkline(t *testing.T) {
	if got := Sparkline(nil); got != "" {
		t.Errorf("Sparkline(nil) = %q, want empty", got)
	}
	// 0 → нижний блок, 1 → верхний, выбросы зажимаются
	if got := Sparkline([]float64{0, 0.5, 1, 2, -1}); got != "▁▅██▁" {
		t.Errorf("Sparkline = %q, want ▁▅██▁", got)
	}
}
