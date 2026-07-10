// Package format форматирует значения метрик в стиле Vitals (values.js):
// проценты — целые с зажимом на 100, память — binary-единицы с одним знаком
// после запятой.
package format

import (
	"fmt"
	"math"
)

var binaryUnits = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}

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

// Bytes переводит байты в "24.3 GiB".
func Bytes(v uint64) string {
	val, unit := scale(v)
	if unit == "B" {
		return fmt.Sprintf("%d B", v)
	}
	return fmt.Sprintf("%.1f %s", val, unit)
}

// BytesShort — компактный вариант для menu bar: "24.3G".
func BytesShort(v uint64) string {
	val, unit := scale(v)
	if unit == "B" {
		return fmt.Sprintf("%dB", v)
	}
	return fmt.Sprintf("%.1f%s", val, unit[:1])
}

func scale(v uint64) (float64, string) {
	val := float64(v)
	exp := 0
	for val >= 1024 && exp < len(binaryUnits)-1 {
		val /= 1024
		exp++
	}
	// как в Vitals: 1023.96 KiB показываем как 1.0 MiB, а не "1024.0 KiB"
	if exp < len(binaryUnits)-1 && val >= 1024-0.05 {
		val /= 1024
		exp++
	}
	return val, binaryUnits[exp]
}
