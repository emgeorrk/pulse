package sensors

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	smcTypeFloat = "flt "
	smcTypeFPE2  = "fpe2"
	smcTypeSP78  = "sp78"
	smcTypeUI8   = "ui8 "
	smcTypeUI16  = "ui16"
	smcTypeUI32  = "ui32"
	wordBytes    = 2
	dwordBytes   = 4
	fpe2Divisor  = 4
	sp78Divisor  = 256
)

// decodeSMC converts an SMC key's raw bytes to a number based on its fourcc
// data type. Intel SMC returns big-endian fixed-point formats (fpe2, sp78),
// Apple Silicon returns little-endian IEEE-754 float ("flt ").
func decodeSMC(typ string, b []byte) (float64, error) { //nolint:cyclop,gocyclo // Each case decodes a distinct SMC wire format.
	switch typ {
	case smcTypeFloat:
		if len(b) < dwordBytes {
			return 0, fmt.Errorf("%s: %w (%d bytes)", smcTypeFloat, errSMCDataShort, len(b))
		}

		return float64(math.Float32frombits(binary.LittleEndian.Uint32(b))), nil
	case smcTypeFPE2:
		if len(b) < wordBytes {
			return 0, fmt.Errorf("%s: %w (%d bytes)", smcTypeFPE2, errSMCDataShort, len(b))
		}

		return float64(binary.BigEndian.Uint16(b)) / fpe2Divisor, nil
	case smcTypeSP78:
		if len(b) < wordBytes {
			return 0, fmt.Errorf("%s: %w (%d bytes)", smcTypeSP78, errSMCDataShort, len(b))
		}

		raw := binary.BigEndian.Uint16(b)
		signed := int16(raw) //nolint:gosec // SMC sp78 is a signed 16-bit fixed-point bit pattern.

		return float64(signed) / sp78Divisor, nil
	case smcTypeUI8:
		if len(b) < 1 {
			return 0, fmt.Errorf("%s: %w", smcTypeUI8, errSMCDataShort)
		}

		return float64(b[0]), nil
	case smcTypeUI16:
		if len(b) < wordBytes {
			return 0, fmt.Errorf("%s: %w (%d bytes)", smcTypeUI16, errSMCDataShort, len(b))
		}

		return float64(binary.BigEndian.Uint16(b)), nil
	case smcTypeUI32:
		if len(b) < dwordBytes {
			return 0, fmt.Errorf("%s: %w (%d bytes)", smcTypeUI32, errSMCDataShort, len(b))
		}

		return float64(binary.BigEndian.Uint32(b)), nil
	default:
		return 0, fmt.Errorf("%w %q", errSMCDataType, typ)
	}
}
