// Package format formats metric values Vitals-style (values.js): percentages
// are integers clamped at 100, memory uses one decimal digit (binary or
// decimal units per setting), network speed uses decimal units, temperature
// is °C/°F.
package format

import (
	"fmt"
	"math"
	"strings"
)

var (
	binaryUnits  = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}
	decimalUnits = []string{"B", "KB", "MB", "GB", "TB", "PB"}
	hertzUnits   = []string{"Hz", "KHz", "MHz", "GHz", "THz"}
	sparkRunes   = []rune("▁▂▃▄▅▆▇█")
)

// Percent converts a 0..1 fraction to "7%"; out-of-range values are clamped.
func Percent(v float64) string {
	p := math.Round(v * 100)
	if p > 100 {
		p = 100
	}
	if p < 0 {
		p = 0
	}
	return fmt.Sprintf("%d%%", int(p))
}

// Bytes converts bytes to "24.3 GiB" (or "26.1 GB" when decimal).
func Bytes(v uint64, decimal bool) string {
	val, unit := scaleBytes(v, decimal)
	if unit == "B" {
		return fmt.Sprintf("%d B", v)
	}
	return fmt.Sprintf("%.1f %s", val, unit)
}

// BytesShort is a compact variant for the menu bar: "24.3G".
func BytesShort(v uint64, decimal bool) string {
	val, unit := scaleBytes(v, decimal)
	if unit == "B" {
		return fmt.Sprintf("%dB", v)
	}
	return fmt.Sprintf("%.1f%s", val, unit[:1])
}

// Speed formats bytes/s in decimal units, like Vitals: "1.2 MB/s".
func Speed(bytesPerSec float64) string {
	if bytesPerSec < 0 {
		bytesPerSec = 0
	}
	val, unit := scale(bytesPerSec, 1000, decimalUnits)
	if unit == "B" {
		return fmt.Sprintf("%.0f B/s", val)
	}
	return fmt.Sprintf("%.1f %s/s", val, unit)
}

// SpeedShort is for the menu bar: "1.2M/s".
func SpeedShort(bytesPerSec float64) string {
	if bytesPerSec < 0 {
		bytesPerSec = 0
	}
	val, unit := scale(bytesPerSec, 1000, decimalUnits)
	if unit == "B" {
		return fmt.Sprintf("%.0fB/s", val)
	}
	return fmt.Sprintf("%.1f%s/s", val, unit[:1])
}

// Temp converts Celsius degrees to "54°C" or "129°F".
func Temp(celsius float64, fahrenheit bool) string {
	unit := "C"
	v := celsius
	if fahrenheit {
		v = celsius*9/5 + 32
		unit = "F"
	}
	return fmt.Sprintf("%d°%s", int(math.Round(v)), unit)
}

// TempShort is for the menu bar: "54°".
func TempShort(celsius float64, fahrenheit bool) string {
	v := celsius
	if fahrenheit {
		v = celsius*9/5 + 32
	}
	return fmt.Sprintf("%d°", int(math.Round(v)))
}

// RPM formats fan speed: "1850 RPM".
func RPM(v float64) string {
	return fmt.Sprintf("%d RPM", int(math.Round(v)))
}

// Watts formats power: "8.4 W".
func Watts(v float64) string {
	return fmt.Sprintf("%.1f W", v)
}

// Volts formats voltage: "13.08 V".
func Volts(v float64) string {
	return fmt.Sprintf("%.2f V", v)
}

// Hertz formats frequency: "3.5 GHz".
func Hertz(hz float64) string {
	if hz <= 0 {
		return "0 Hz"
	}
	val, unit := scale(hz, 1000, hertzUnits)
	return fmt.Sprintf("%.1f %s", val, unit)
}

// Sparkline renders a history of 0..1 fractions as unicode blocks: "▂▃▆▄".
// An empty history yields an empty string.
func Sparkline(vals []float64) string {
	var b strings.Builder
	for _, v := range vals {
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		idx := int(v * float64(len(sparkRunes)))
		if idx >= len(sparkRunes) {
			idx = len(sparkRunes) - 1
		}
		b.WriteRune(sparkRunes[idx])
	}
	return b.String()
}

func scaleBytes(v uint64, decimal bool) (float64, string) {
	if decimal {
		return scale(float64(v), 1000, decimalUnits)
	}
	return scale(float64(v), 1024, binaryUnits)
}

func scale(v, unit float64, units []string) (float64, string) {
	exp := 0
	for v >= unit && exp < len(units)-1 {
		v /= unit
		exp++
	}
	// as in Vitals: show 1023.96 KiB as 1.0 MiB, not "1024.0 KiB"
	if exp < len(units)-1 && v >= unit-0.05 {
		v /= unit
		exp++
	}
	return v, units[exp]
}
