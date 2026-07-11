package sensors

import (
	"math"
	"testing"
)

func TestDecodeSMC(t *testing.T) {
	cases := []struct {
		typ  string
		b    []byte
		want float64
	}{
		// Apple Silicon: little-endian float32 (1850.0 RPM)
		{"flt ", []byte{0x00, 0x40, 0xe7, 0x44}, 1850},
		// Intel fan: big-endian fpe2 — 0x1CE8 >> 2 = 1850
		{"fpe2", []byte{0x1c, 0xe8}, 1850},
		// Intel temp: sp78 — 0x3A80 / 256 = 58.5°C
		{"sp78", []byte{0x3a, 0x80}, 58.5},
		// negative temperature in sp78
		{"sp78", []byte{0xff, 0x00}, -1},
		{"ui8 ", []byte{2}, 2},
		{"ui16", []byte{0x01, 0x00}, 256},
		{"ui32", []byte{0, 0, 0x01, 0x00}, 256},
	}
	for _, c := range cases {
		got, err := decodeSMC(c.typ, c.b)
		if err != nil {
			t.Errorf("decodeSMC(%q) error: %v", c.typ, err)
			continue
		}
		if math.Abs(got-c.want) > 1e-6 {
			t.Errorf("decodeSMC(%q) = %v, want %v", c.typ, got, c.want)
		}
	}
}

func TestDecodeSMCErrors(t *testing.T) {
	if _, err := decodeSMC("flt ", []byte{1, 2}); err == nil {
		t.Error("short flt should error")
	}
	if _, err := decodeSMC("hex_", []byte{1, 2, 3, 4}); err == nil {
		t.Error("unknown type should error")
	}
}
