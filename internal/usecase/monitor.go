// Package usecase содержит логику мониторинга: цикл сэмплирования сенсоров
// и расчёт загрузки CPU по дельте тиков.
package usecase

import (
	"context"
	"time"

	"github.com/emgeorrk/pulse/internal/entity"
	"github.com/emgeorrk/pulse/internal/sensors"
)

type Monitor struct {
	cpu      sensors.CPUSource
	mem      sensors.MemSource
	interval time.Duration
}

func NewMonitor(cpu sensors.CPUSource, mem sensors.MemSource, interval time.Duration) *Monitor {
	return &Monitor{cpu: cpu, mem: mem, interval: interval}
}

// Start запускает цикл сэмплирования в отдельной горутине (никогда не на
// UI-потоке). Первый кадр приходит через interval — сразу с осмысленной
// дельтой CPU. Канал закрывается при отмене ctx.
func (m *Monitor) Start(ctx context.Context) <-chan entity.Snapshot {
	out := make(chan entity.Snapshot, 1)
	go func() {
		defer close(out)

		prev, _ := m.cpu.Ticks() // точка отсчёта для первой дельты
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			var snap entity.Snapshot
			if cur, err := m.cpu.Ticks(); err == nil {
				if prev != nil {
					snap.CPU = CPUUsage(prev, cur)
				}
				prev = cur
			}
			if ms, err := m.mem.Read(); err == nil {
				snap.Mem = ms
			}

			// UI не успел забрать прошлый кадр — просто роняем его.
			select {
			case out <- snap:
			default:
			}
		}
	}()
	return out
}

// CPUUsage считает загрузку по дельте накопительных тиков. Тики 32-битные и
// переполняются; вычитание в uint32 корректно по модулю 2^32.
func CPUUsage(prev, cur []entity.CoreTicks) entity.CPUStats {
	n := min(len(prev), len(cur))
	stats := entity.CPUStats{Cores: make([]float64, n)}

	var busyAll, totalAll uint64
	for i := 0; i < n; i++ {
		busy := uint64(cur[i].User-prev[i].User) +
			uint64(cur[i].System-prev[i].System) +
			uint64(cur[i].Nice-prev[i].Nice)
		total := busy + uint64(cur[i].Idle-prev[i].Idle)
		if total > 0 {
			stats.Cores[i] = float64(busy) / float64(total)
		}
		busyAll += busy
		totalAll += total
	}
	if totalAll > 0 {
		stats.Total = float64(busyAll) / float64(totalAll)
	}
	return stats
}
