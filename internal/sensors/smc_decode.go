package sensors

import (
	"encoding/binary"
	"fmt"
	"math"
)

// decodeSMC переводит сырые байты SMC-ключа в число по fourcc-типу данных.
// Intel-SMC отдаёт big-endian форматы с фиксированной точкой (fpe2, sp78),
// Apple Silicon — little-endian IEEE-754 float ("flt ").
func decodeSMC(typ string, b []byte) (float64, error) {
	switch typ {
	case "flt ":
		if len(b) < 4 {
			return 0, fmt.Errorf("flt: short data (%d bytes)", len(b))
		}
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(b))), nil
	case "fpe2":
		if len(b) < 2 {
			return 0, fmt.Errorf("fpe2: short data (%d bytes)", len(b))
		}
		return float64(binary.BigEndian.Uint16(b)) / 4, nil
	case "sp78":
		if len(b) < 2 {
			return 0, fmt.Errorf("sp78: short data (%d bytes)", len(b))
		}
		return float64(int16(binary.BigEndian.Uint16(b))) / 256, nil
	case "ui8 ":
		if len(b) < 1 {
			return 0, fmt.Errorf("ui8: no data")
		}
		return float64(b[0]), nil
	case "ui16":
		if len(b) < 2 {
			return 0, fmt.Errorf("ui16: short data (%d bytes)", len(b))
		}
		return float64(binary.BigEndian.Uint16(b)), nil
	case "ui32":
		if len(b) < 4 {
			return 0, fmt.Errorf("ui32: short data (%d bytes)", len(b))
		}
		return float64(binary.BigEndian.Uint32(b)), nil
	default:
		return 0, fmt.Errorf("unsupported SMC data type %q", typ)
	}
}
