// Package sensors — слой источников данных (аналог repo в go-clean-template).
// Интерфейсы ниже — точка ветвления для платформенных реализаций: Mach API
// (этот инкремент, одинаков на Intel и Apple Silicon), дальше SMC на Intel и
// IOHIDEventSystemClient/IOReport на Apple Silicon.
package sensors

import "github.com/emgeorrk/pulse/internal/entity"

// CPUSource отдаёт накопительные тики загрузки по ядрам; загрузку считает
// usecase по дельте двух чтений.
type CPUSource interface {
	Ticks() ([]entity.CoreTicks, error)
}

// MemSource отдаёт текущее состояние памяти и свопа.
type MemSource interface {
	Read() (entity.MemStats, error)
}

// NetSource отдаёт накопительные счётчики трафика по интерфейсам;
// скорости считает usecase по дельте.
type NetSource interface {
	Counters() ([]entity.NetCounters, error)
}

// DiskSource отдаёт заполненность корневого тома и накопительные счётчики
// чтения/записи (IOKit, с загрузки системы).
type DiskSource interface {
	Usage() (entity.DiskUsage, error)
	IOTotals() (read, write uint64, err error)
}

// TempSource отдаёт показания всех температурных сенсоров в °C.
// Реализации: HID sensor hub на Apple Silicon, SMC-ключи на Intel.
type TempSource interface {
	Temps() ([]entity.Reading, error)
}

// VoltSource отдаёт показания сенсоров напряжения в вольтах.
type VoltSource interface {
	Voltages() ([]entity.Reading, error)
}

// FanSource отдаёт обороты вентиляторов (SMC на обеих платформах).
type FanSource interface {
	Fans() ([]entity.Fan, error)
}

// BatterySource отдаёт состояние батареи (IORegistry AppleSmartBattery).
type BatterySource interface {
	Battery() (entity.BatteryStats, error)
}

// GPUSource отдаёт загрузку GPU (IOAccelerator PerformanceStatistics).
type GPUSource interface {
	GPU() (entity.GPUStats, error)
}

// PowerSource отдаёт мощность CPU/GPU/ANE в ваттах, усреднённую с прошлого
// вызова (IOReport Energy Model — накопительные счётчики энергии).
type PowerSource interface {
	Power() (entity.PowerStats, error)
}

// Sources — собранные при старте источники; nil = недоступен на этом железе
// (группа скрывается). CPU и Mem обязательны.
type Sources struct {
	CPU     CPUSource
	Mem     MemSource
	Net     NetSource
	Disk    DiskSource
	Temp    TempSource
	Volt    VoltSource
	Fan     FanSource
	Battery BatterySource
	GPU     GPUSource
	Power   PowerSource
}
