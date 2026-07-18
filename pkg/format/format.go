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

const (
	percentScale     = 100
	decimalScale     = 1000
	binaryScale      = 1024
	fahrenheitFactor = 9.0 / 5.0
	fahrenheitOffset = 32
	zeroHertz        = "0 Hz"
	sparklineGlyphs  = "▁▂▃▄▅▆▇█"
)

// Percent converts a 0..1 fraction to "7%" ("7.3%" when precise);
// out-of-range values are clamped.
func Percent(v float64, precise bool) string {
	p := v * percentScale
	if p > percentScale {
		p = percentScale
	}

	if p < 0 {
		p = 0
	}

	if precise {
		return fmt.Sprintf("%.1f%%", p)
	}

	return fmt.Sprintf("%d%%", int(math.Round(p)))
}

// Bytes converts bytes to "24.3 GiB" (or "26.1 GB" when decimal); precise
// adds one more fraction digit.
func Bytes(v uint64, decimal, precise bool) string {
	val, unit := scaleBytes(v, decimal)
	if unit == "B" {
		return fmt.Sprintf("%d B", v)
	}

	return fmt.Sprintf("%.*f %s", fractionDigits(precise), val, unit)
}

// BytesShort is a compact variant for the menu bar: "24.3G".
func BytesShort(v uint64, decimal, precise bool) string {
	val, unit := scaleBytes(v, decimal)
	if unit == "B" {
		return fmt.Sprintf("%dB", v)
	}

	return fmt.Sprintf("%.*f%s", fractionDigits(precise), val, unit[:1])
}

// Speed formats bytes/s in decimal units, like Vitals: "1.2 MB/s".
func Speed(bytesPerSec float64, precise bool) string {
	if bytesPerSec < 0 {
		bytesPerSec = 0
	}

	val, unit := scale(bytesPerSec, decimalScale, decimalUnits())
	if unit == "B" {
		return fmt.Sprintf("%.0f B/s", val)
	}

	return fmt.Sprintf("%.*f %s/s", fractionDigits(precise), val, unit)
}

// SpeedShort is for the menu bar: "1.2M/s".
func SpeedShort(bytesPerSec float64, precise bool) string {
	if bytesPerSec < 0 {
		bytesPerSec = 0
	}

	val, unit := scale(bytesPerSec, decimalScale, decimalUnits())
	if unit == "B" {
		return fmt.Sprintf("%.0fB/s", val)
	}

	return fmt.Sprintf("%.*f%s/s", fractionDigits(precise), val, unit[:1])
}

// Temp converts Celsius degrees to "54°C" or "129°F" ("54.3°C" when precise).
func Temp(celsius float64, fahrenheit, precise bool) string {
	unit := "C"

	v := celsius
	if fahrenheit {
		v = celsius*fahrenheitFactor + fahrenheitOffset
		unit = "F"
	}

	if precise {
		return fmt.Sprintf("%.1f°%s", v, unit)
	}

	return fmt.Sprintf("%d°%s", int(math.Round(v)), unit)
}

// TempShort is for the menu bar: "54°".
func TempShort(celsius float64, fahrenheit, precise bool) string {
	v := celsius
	if fahrenheit {
		v = celsius*fahrenheitFactor + fahrenheitOffset
	}

	if precise {
		return fmt.Sprintf("%.1f°", v)
	}

	return fmt.Sprintf("%d°", int(math.Round(v)))
}

// Fraction digits for scaled values: one normally, two in higher-precision
// mode.
const (
	fractionDigitsDefault = 1
	fractionDigitsPrecise = 2
)

func fractionDigits(precise bool) int {
	if precise {
		return fractionDigitsPrecise
	}

	return fractionDigitsDefault
}

// Load formats a load average: "1.42".
func Load(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

// Uptime formats seconds of uptime: "3d 4h 12m", "4h 12m" or "12m".
func Uptime(sec uint64) string {
	const (
		secondsPerMinute = 60
		minutesPerHour   = 60
		hoursPerDay      = 24
	)

	minutes := sec / secondsPerMinute
	hours := minutes / minutesPerHour
	days := hours / hoursPerDay

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh %dm", days, hours%hoursPerDay, minutes%minutesPerHour)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes%minutesPerHour)
	default:
		return fmt.Sprintf("%dm", minutes)
	}
}

// Flag renders an ISO 3166-1 alpha-2 country code as its emoji flag
// ("NL" → 🇳🇱, case-insensitive) by mapping each letter to a regional
// indicator symbol. Anything that isn't two ASCII letters yields "".
func Flag(cc string) string {
	const (
		countryCodeLen        = 2
		regionalIndicatorBase = 0x1F1E6 // U+1F1E6 REGIONAL INDICATOR SYMBOL LETTER A
	)

	if len(cc) != countryCodeLen {
		return ""
	}

	flag := make([]rune, 0, countryCodeLen)

	for _, c := range strings.ToUpper(cc) {
		if c < 'A' || c > 'Z' {
			return ""
		}

		flag = append(flag, regionalIndicatorBase+c-'A')
	}

	return string(flag)
}

// WithFlag appends a country's emoji flag to a value ("1.2.3.4", "NL" →
// "1.2.3.4 🇳🇱"); an unknown code leaves the value unchanged.
func WithFlag(v, cc string) string {
	if flag := Flag(cc); flag != "" {
		return v + " " + flag
	}

	return v
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
		return zeroHertz
	}

	val, unit := scale(hz, decimalScale, hertzUnits())

	return fmt.Sprintf("%.1f %s", val, unit)
}

// Sparkline renders a history of 0..1 fractions as unicode blocks: "▂▃▆▄".
// An empty history yields an empty string.
func Sparkline(vals []float64) string {
	var b strings.Builder

	sparkRunes := []rune(sparklineGlyphs)

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

func scaleBytes(v uint64, decimal bool) (value float64, label string) {
	if decimal {
		return scale(float64(v), decimalScale, decimalUnits())
	}

	return scale(float64(v), binaryScale, binaryUnits())
}

func scale(v, unit float64, units []string) (value float64, label string) {
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

func binaryUnits() []string {
	return []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}
}

func decimalUnits() []string {
	return []string{"B", "KB", "MB", "GB", "TB", "PB"}
}

func hertzUnits() []string {
	return []string{"Hz", "KHz", "MHz", "GHz", "THz"}
}
