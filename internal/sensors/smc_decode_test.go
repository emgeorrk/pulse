package sensors

import (
	"errors"
	"math"
	"testing"
)

func TestDecodeSMC(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		typ         string
		b           []byte
		want        float64
		wantErr     bool
		wantErrType error // if set, err must match this sentinel
	}{
		{
			// Apple Silicon: little-endian float32 (1850.0 RPM)
			name: "flt little-endian float32",
			typ:  "flt ",
			b:    []byte{0x00, 0x40, 0xe7, 0x44},
			want: 1850,
		},
		{
			// Intel fan: big-endian fpe2 — 0x1CE8 >> 2 = 1850
			name: "fpe2 big-endian fixed-point",
			typ:  "fpe2",
			b:    []byte{0x1c, 0xe8},
			want: 1850,
		},
		{
			// Intel temp: sp78 — 0x3A80 / 256 = 58.5°C
			name: "sp78 temperature",
			typ:  "sp78",
			b:    []byte{0x3a, 0x80},
			want: 58.5,
		},
		{
			name: "sp78 negative temperature",
			typ:  "sp78",
			b:    []byte{0xff, 0x00},
			want: -1,
		},
		{name: "ui8", typ: "ui8 ", b: []byte{2}, want: 2},
		{name: "ui16", typ: "ui16", b: []byte{0x01, 0x00}, want: 256},
		{name: "ui32", typ: "ui32", b: []byte{0, 0, 0x01, 0x00}, want: 256},
		{name: "short flt errors", typ: "flt ", b: []byte{1, 2}, wantErr: true, wantErrType: errSMCDataShort},
		{name: "short fpe2 errors", typ: "fpe2", b: []byte{1}, wantErr: true, wantErrType: errSMCDataShort},
		{name: "short sp78 errors", typ: "sp78", b: []byte{1}, wantErr: true, wantErrType: errSMCDataShort},
		{name: "short ui8 errors", typ: "ui8 ", b: []byte{}, wantErr: true, wantErrType: errSMCDataShort},
		{name: "short ui16 errors", typ: "ui16", b: []byte{1}, wantErr: true, wantErrType: errSMCDataShort},
		{name: "short ui32 errors", typ: "ui32", b: []byte{1, 2, 3}, wantErr: true, wantErrType: errSMCDataShort},
		{name: "unknown type errors", typ: "hex_", b: []byte{1, 2, 3, 4}, wantErr: true, wantErrType: errSMCDataType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := decodeSMC(tt.typ, tt.b)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("decodeSMC(%q, % x) = %v, want error", tt.typ, tt.b, got)
				}

				if tt.wantErrType != nil && !errors.Is(err, tt.wantErrType) {
					t.Errorf("decodeSMC(%q) error = %v, want %v", tt.typ, err, tt.wantErrType)
				}

				return
			}

			if err != nil {
				t.Fatalf("decodeSMC(%q) error: %v", tt.typ, err)
			}

			if math.Abs(got-tt.want) > 1e-6 {
				t.Errorf("decodeSMC(%q) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
}
