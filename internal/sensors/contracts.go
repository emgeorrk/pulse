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
