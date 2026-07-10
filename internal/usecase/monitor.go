// Package usecase содержит логику мониторинга: цикл сэмплирования сенсоров
// и расчёт метрик по дельтам накопительных счётчиков.
package usecase

import (
	"context"
	"time"

	"github.com/emgeorrk/pulse/config"
	"github.com/emgeorrk/pulse/internal/entity"
	"github.com/emgeorrk/pulse/internal/sensors"
)

// historyLen — сколько последних значений CPU хранить для спарклайна.
const historyLen = 12

type Monitor struct {
	src   sensors.Sources
	store *config.Store

	// состояние между тиками
	prevTicks []entity.CoreTicks
	prevNet   map[string]entity.NetCounters
	sessDown  uint64
	sessUp    uint64
	prevRead  uint64
	prevWrite uint64
	haveDisk  bool
	lastTick  time.Time
	history   []float64
}

func NewMonitor(src sensors.Sources, store *config.Store) *Monitor {
	return &Monitor{src: src, store: store}
}

// Start запускает цикл сэмплирования в отдельной горутине (никогда не на
// UI-потоке). Первый кадр приходит через interval — сразу с осмысленными
// дельтами. Интервал перечитывается из настроек на каждом тике, так что
// смена в Settings подхватывается без рестарта. Канал закрывается при
// отмене ctx.
func (m *Monitor) Start(ctx context.Context) <-chan entity.Snapshot {
	out := make(chan entity.Snapshot, 1)
	go func() {
		defer close(out)

		m.prime()
		interval := m.store.Get().Interval()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			if cur := m.store.Get().Interval(); cur != interval {
				interval = cur
				ticker.Reset(interval)
			}

			snap := m.sample()

			// UI не успел забрать прошлый кадр — просто роняем его.
			select {
			case out <- snap:
			default:
			}
		}
	}()
	return out
}

// prime снимает первые значения счётчиков — точки отсчёта для дельт.
func (m *Monitor) prime() {
	m.prevTicks, _ = m.src.CPU.Ticks()
	if m.src.Net != nil {
		if counters, err := m.src.Net.Counters(); err == nil {
			m.prevNet = countersMap(counters)
		}
	}
	if m.src.Disk != nil {
		if r, w, err := m.src.Disk.IOTotals(); err == nil {
			m.prevRead, m.prevWrite, m.haveDisk = r, w, true
		}
	}
	m.lastTick = time.Now()
	m.history = make([]float64, 0, historyLen)
}

// sample снимает один кадр всех доступных метрик.
func (m *Monitor) sample() entity.Snapshot {
	now := time.Now()
	dwell := now.Sub(m.lastTick).Seconds()
	m.lastTick = now
	if dwell <= 0 {
		dwell = 1
	}

	var snap entity.Snapshot

	if cur, err := m.src.CPU.Ticks(); err == nil {
		if m.prevTicks != nil {
			snap.CPU = CPUUsage(m.prevTicks, cur)
		}
		m.prevTicks = cur

		if len(m.history) == historyLen {
			copy(m.history, m.history[1:])
			m.history = m.history[:historyLen-1]
		}
		m.history = append(m.history, snap.CPU.Total)
		snap.CPU.History = append([]float64(nil), m.history...)
	}

	if ms, err := m.src.Mem.Read(); err == nil {
		snap.Mem = ms
	}

	if m.src.Net != nil && m.prevNet != nil {
		if counters, err := m.src.Net.Counters(); err == nil {
			net := NetRates(m.prevNet, counters, dwell)
			m.prevNet = countersMap(counters)
			m.sessDown += uint64(net.Down * dwell)
			m.sessUp += uint64(net.Up * dwell)
			net.SessionDown, net.SessionUp = m.sessDown, m.sessUp
			snap.Net = &net
		}
	}

	if m.src.Disk != nil {
		if usage, err := m.src.Disk.Usage(); err == nil {
			disk := entity.DiskStats{DiskUsage: usage}
			if r, w, err := m.src.Disk.IOTotals(); err == nil && m.haveDisk {
				disk.ReadRate = rate64(m.prevRead, r, dwell)
				disk.WriteRate = rate64(m.prevWrite, w, dwell)
				disk.ReadTotal, disk.WriteTotal = r, w
				m.prevRead, m.prevWrite = r, w
			}
			snap.Disk = &disk
		}
	}

	if m.src.Temp != nil {
		if all, err := m.src.Temp.Temps(); err == nil {
			t := AggregateTemps(all)
			snap.Temps = &t
		}
	}
	if m.src.Volt != nil {
		if volts, err := m.src.Volt.Voltages(); err == nil {
			snap.Volts = volts
		}
	}
	if m.src.Fan != nil {
		if fans, err := m.src.Fan.Fans(); err == nil {
			snap.Fans = fans
		}
	}

	if m.src.Battery != nil {
		if b, err := m.src.Battery.Battery(); err == nil {
			snap.Battery = &b
		}
	}
	if m.src.GPU != nil {
		if g, err := m.src.GPU.GPU(); err == nil {
			snap.GPU = &g
		}
	}
	if m.src.Power != nil {
		if p, err := m.src.Power.Power(); err == nil {
			snap.Power = &p
		}
	}
	if m.src.Freq != nil {
		if f, err := m.src.Freq.Frequency(); err == nil {
			snap.Freq = &f
		}
	}

	return snap
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

// NetRates считает скорости по дельтам 32-битных счётчиков if_data
// (вычитание в uint32 корректно при переполнении). Интерфейсы без прошлого
// значения (появились между тиками) пропускаются до следующего тика.
func NetRates(prev map[string]entity.NetCounters, cur []entity.NetCounters, dwellSec float64) entity.NetStats {
	var stats entity.NetStats
	for _, c := range cur {
		p, ok := prev[c.Name]
		if !ok {
			continue
		}
		down := float64(c.Rx-p.Rx) / dwellSec
		up := float64(c.Tx-p.Tx) / dwellSec
		stats.Down += down
		stats.Up += up
		if down > 0 || up > 0 {
			stats.Ifaces = append(stats.Ifaces, entity.NetIface{Name: c.Name, Down: down, Up: up})
		}
	}
	return stats
}

func countersMap(counters []entity.NetCounters) map[string]entity.NetCounters {
	m := make(map[string]entity.NetCounters, len(counters))
	for _, c := range counters {
		m[c.Name] = c
	}
	return m
}

// rate64 — скорость по 64-битным счётчикам; сброс счётчика (cur < prev)
// не даёт отрицательных скоростей.
func rate64(prev, cur uint64, dwellSec float64) float64 {
	if cur < prev {
		return 0
	}
	return float64(cur-prev) / dwellSec
}
