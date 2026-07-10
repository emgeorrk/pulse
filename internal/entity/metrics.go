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

// NetCounters — накопительные счётчики одного интерфейса из if_data
// (getifaddrs). В ядре они 32-битные и переполняются по модулю 2^32.
type NetCounters struct {
	Name string
	Rx   uint32
	Tx   uint32
}

// NetIface — скорость одного интерфейса за интервал.
type NetIface struct {
	Name string
	Down float64 // байт/с
	Up   float64
}

// NetStats — сеть: суммарные скорости, накопленный за сессию трафик и
// разбивка по интерфейсам.
type NetStats struct {
	Down        float64 // байт/с, сумма по интерфейсам
	Up          float64
	SessionDown uint64 // байты с запуска pulse (boot-тоталы ненадёжны: счётчики 32-битные)
	SessionUp   uint64
	Ifaces      []NetIface
}

// DiskUsage — заполненность корневого тома, в байтах.
type DiskUsage struct {
	Total     uint64
	Used      uint64
	Available uint64
}

func (d DiskUsage) UsedFraction() float64 {
	if d.Total == 0 {
		return 0
	}
	return float64(d.Used) / float64(d.Total)
}

// DiskStats — заполненность + скорости и суммарный I/O с загрузки системы.
type DiskStats struct {
	DiskUsage
	ReadRate   float64 // байт/с
	WriteRate  float64
	ReadTotal  uint64 // с загрузки системы (64-битные счётчики IOKit)
	WriteTotal uint64
}

// Reading — одно показание именованного сенсора (температура °C, вольты, …).
type Reading struct {
	Name  string
	Value float64
}

// TempStats — агрегаты + все температурные сенсоры.
type TempStats struct {
	CPU     float64 // среднее по CPU-сенсорам; 0 = не определили
	GPU     float64
	Hottest Reading
	All     []Reading
}

// Fan — один вентилятор: текущие обороты и паспортные пределы.
type Fan struct {
	Name string
	RPM  float64
	Min  float64
	Max  float64
}

// BatteryStats — состояние батареи из IORegistry (AppleSmartBattery).
type BatteryStats struct {
	Percent     float64 // 0..1
	Health      float64 // 0..1, фактическая ёмкость к паспортной
	Cycles      int
	TempC       float64
	Volts       float64
	Watts       float64 // >0 заряд, <0 разряд
	Charging    bool
	External    bool // питание от сети
	MinutesLeft int  // до разряда (или до полного заряда при зарядке); -1 = неизвестно
}

// GPUStats — загрузка GPU из IOAccelerator PerformanceStatistics.
type GPUStats struct {
	Utilization float64 // 0..1
}

// PowerStats — мощность по каналам IOReport Energy Model, Вт.
type PowerStats struct {
	CPU   float64
	GPU   float64
	ANE   float64
	Total float64
}

// FreqStats — средневзвешенная частота CPU по кластерам (IOReport perf
// states × таблицы частот из device tree).
type FreqStats struct {
	Clusters []Reading // "E-cores"/"P-cores", Гц
	Max      float64   // максимум по кластерам — текущая рабочая частота
}

// Caps — какие группы метрик реально доступны на этом железе; недоступные
// UI не показывает (правило CLAUDE.md: скрывать, а не падать).
type Caps struct {
	Net          bool
	NetIfaces    []string // интерфейсы с трафиком на момент старта
	Disk         bool
	Temps        bool
	TempSensors  []string // имена сенсоров на момент старта (строки меню)
	Volts        bool
	VoltSensors  []string
	Fans         bool
	FanCount     int
	Battery      bool
	GPU          bool
	Power        bool
	Freq         bool
	FreqClusters []string // имена кластерных каналов (MCPU0/PCPU/…)
}

// Snapshot — один кадр всех метрик, отправляемый в UI. Указатели — у групп,
// которых может не быть на данном железе или в данном кадре.
type Snapshot struct {
	CPU     CPUStats
	Mem     MemStats
	Net     *NetStats
	Disk    *DiskStats
	Temps   *TempStats
	Volts   []Reading
	Fans    []Fan
	Battery *BatteryStats
	GPU     *GPUStats
	Power   *PowerStats
	Freq    *FreqStats
}
