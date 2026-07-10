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
		{1.5, "100%"},  // зажим сверху, как в Vitals
		{-0.1, "0%"},   // мусор снизу не показываем
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
		in   uint64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{26092059034, "24.3 GiB"}, // ≈24.3 GiB
		{48 * gib, "48.0 GiB"},
		{1048525, "1.0 MiB"}, // 1023.95 KiB → показываем в следующей единице, а не "1024.0 KiB"
	}
	for _, c := range cases {
		if got := Bytes(c.in); got != c.want {
			t.Errorf("Bytes(%d) = %q, want %q", c.in, got, c.want)
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
		if got := BytesShort(c.in); got != c.want {
			t.Errorf("BytesShort(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
