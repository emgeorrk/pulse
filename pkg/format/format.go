// Package format форматирует значения метрик в стиле Vitals (values.js):
// проценты — целые с зажимом на 100, память — единицы с одним знаком после
// запятой (binary или decimal по настройке), скорость сети — десятичные
// единицы, температура — °C/°F.
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

// Percent переводит долю 0..1 в "7%"; значения вне диапазона зажимаются.
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

// Bytes переводит байты в "24.3 GiB" (или "26.1 GB" при decimal).
func Bytes(v uint64, decimal bool) string {
	val, unit := scaleBytes(v, decimal)
	if unit == "B" {
		return fmt.Sprintf("%d B", v)
	}
	return fmt.Sprintf("%.1f %s", val, unit)
}

// BytesShort — компактный вариант для menu bar: "24.3G".
func BytesShort(v uint64, decimal bool) string {
	val, unit := scaleBytes(v, decimal)
	if unit == "B" {
		return fmt.Sprintf("%dB", v)
	}
	return fmt.Sprintf("%.1f%s", val, unit[:1])
}

// Speed — байты/с в десятичных единицах, как Vitals: "1.2 MB/s".
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

// SpeedShort — для menu bar: "1.2M/s".
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

// Temp — градусы Цельсия в "54°C" или "129°F".
func Temp(celsius float64, fahrenheit bool) string {
	unit := "C"
	v := celsius
	if fahrenheit {
		v = celsius*9/5 + 32
		unit = "F"
	}
	return fmt.Sprintf("%d°%s", int(math.Round(v)), unit)
}

// TempShort — для menu bar: "54°".
func TempShort(celsius float64, fahrenheit bool) string {
	v := celsius
	if fahrenheit {
		v = celsius*9/5 + 32
	}
	return fmt.Sprintf("%d°", int(math.Round(v)))
}

// RPM — обороты вентилятора: "1850 RPM".
func RPM(v float64) string {
	return fmt.Sprintf("%d RPM", int(math.Round(v)))
}

// Watts — мощность: "8.4 W".
func Watts(v float64) string {
	return fmt.Sprintf("%.1f W", v)
}

// Volts — напряжение: "13.08 V".
func Volts(v float64) string {
	return fmt.Sprintf("%.2f V", v)
}

// Hertz — частота: "3.5 GHz".
func Hertz(hz float64) string {
	if hz <= 0 {
		return "0 Hz"
	}
	val, unit := scale(hz, 1000, hertzUnits)
	return fmt.Sprintf("%.1f %s", val, unit)
}

// Sparkline рисует историю долей 0..1 юникод-блоками: "▂▃▆▄".
// Пустая история → пустая строка.
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
	// как в Vitals: 1023.96 KiB показываем как 1.0 MiB, а не "1024.0 KiB"
	if exp < len(units)-1 && v >= unit-0.05 {
		v /= unit
		exp++
	}
	return v, units[exp]
}
