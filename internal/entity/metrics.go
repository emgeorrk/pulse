// Package entity описывает доменные типы метрик, независимые от источников
// данных и UI.
package entity

// MetricID — стабильный идентификатор метрики для пиннинга в menu bar и
// настроек: "cpu.total", "mem.used", "temp.cpu", "fan.1", "net.down", …
type MetricID string

// CoreTicks — накопительные тики загрузки одного ядра из Mach
// PROCESSOR_CPU_LOAD_INFO. В ядре это 32-битные счётчики: переполняются по
// модулю 2^32, поэтому дельты считаются в арифметике uint32.
type CoreTicks struct {
	User   uint32
	System uint32
	Idle   uint32
	Nice   uint32
}

// CPUStats — загрузка CPU за интервал между двумя сэмплами, доли 0..1.
type CPUStats struct {
	Total   float64
	Cores   []float64
	History []float64 // последние значения Total (старые → новые), для спарклайна
}

// MemStats — состояние физической памяти и свопа, в байтах.
type MemStats struct {
	Total     uint64 // объём физической памяти
	Used      uint64 // app memory + wired + compressed (как «Memory Used» в Activity Monitor)
	Available uint64 // free + inactive
	Free      uint64
	SwapTotal uint64
	SwapUsed  uint64
}

// UsedFraction — доля занятой памяти 0..1.
func (m MemStats) UsedFraction() float64 {
	if m.Total == 0 {
		return 0
	}
	return float64(m.Used) / float64(m.Total)
}

// HWInfo — сведения о железе; IsAppleSilicon — точка ветвления для будущих
// сенсорных слоёв (SMC на Intel, IOHIDEventSystemClient на Apple Silicon).
type HWInfo struct {
	Chip           string // machdep.cpu.brand_string, например "Apple M5 Pro"
	Model          string // hw.model, например "Mac17,8"
	OSVersion      string // kern.osproductversion, например "26.5.2"
	IsAppleSilicon bool
	NumCores       int
}

// Snapshot — один кадр всех метрик, отправляемый в UI.
type Snapshot struct {
	CPU CPUStats
	Mem MemStats
}
