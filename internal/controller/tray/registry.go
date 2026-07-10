//go:build darwin

package tray

import (
	"fmt"

	"github.com/emgeorrk/pulse/config"
	"github.com/emgeorrk/pulse/internal/entity"
	"github.com/emgeorrk/pulse/pkg/format"
)

// metric — одна строка в дропдауне. bar == nil означает, что метрику нельзя
// пиннить в menu bar (нет компактного представления).
type metric struct {
	id    entity.MetricID
	label string
	menu  func(s entity.Snapshot, c config.Config) string
	bar   func(s entity.Snapshot, c config.Config) string
}

// group — группа метрик в стиле Vitals: живой агрегат в заголовке,
// метрики в подменю.
type group struct {
	emoji     string
	label     string
	aggregate func(s entity.Snapshot, c config.Config) string
	metrics   []metric
}

// buildGroups собирает реестр групп для этого железа. Порядок групп =
// порядок в дропдауне. Этапы 2–5 добавляют сюда сеть/диск/температуры/…
func buildGroups(hw entity.HWInfo) []group {
	cpu := group{
		emoji: "⚙",
		label: "CPU",
		aggregate: func(s entity.Snapshot, c config.Config) string {
			return format.Percent(s.CPU.Total)
		},
		metrics: []metric{
			{
				id:    "cpu.total",
				label: "Usage",
				menu: func(s entity.Snapshot, c config.Config) string {
					return format.Percent(s.CPU.Total)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					return "CPU " + format.Percent(s.CPU.Total)
				},
			},
		},
	}
	for i := 0; i < hw.NumCores; i++ {
		core := i // капчурим индекс
		cpu.metrics = append(cpu.metrics, metric{
			id:    entity.MetricID(fmt.Sprintf("cpu.core.%d", core+1)),
			label: fmt.Sprintf("Core %d", core+1),
			menu: func(s entity.Snapshot, c config.Config) string {
				if core < len(s.CPU.Cores) {
					return format.Percent(s.CPU.Cores[core])
				}
				return "—"
			},
		})
	}

	mem := group{
		emoji: "🧠",
		label: "Memory",
		aggregate: func(s entity.Snapshot, c config.Config) string {
			return format.Percent(s.Mem.UsedFraction())
		},
		metrics: []metric{
			{
				id:    "mem.usage",
				label: "Usage",
				menu: func(s entity.Snapshot, c config.Config) string {
					return format.Percent(s.Mem.UsedFraction())
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					return "MEM " + format.Percent(s.Mem.UsedFraction())
				},
			},
			{
				id:    "mem.used",
				label: "Used",
				menu: func(s entity.Snapshot, c config.Config) string {
					return format.Bytes(s.Mem.Used, c.DecimalBytes)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					return format.BytesShort(s.Mem.Used, c.DecimalBytes)
				},
			},
			{
				id:    "mem.available",
				label: "Available",
				menu: func(s entity.Snapshot, c config.Config) string {
					return format.Bytes(s.Mem.Available, c.DecimalBytes)
				},
			},
			{
				id:    "mem.physical",
				label: "Physical",
				menu: func(s entity.Snapshot, c config.Config) string {
					return format.Bytes(s.Mem.Total, c.DecimalBytes)
				},
			},
			{
				id:    "swap.used",
				label: "Swap",
				menu: func(s entity.Snapshot, c config.Config) string {
					return format.Bytes(s.Mem.SwapUsed, c.DecimalBytes)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					return "SW " + format.BytesShort(s.Mem.SwapUsed, c.DecimalBytes)
				},
			},
		},
	}

	return []group{cpu, mem}
}
