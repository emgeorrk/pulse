// Package usecase contains the monitoring logic: the sensor sampling loop
// and metric computation from cumulative-counter deltas.
package usecase

import (
	"context"
	"time"

	"github.com/emgeorrk/pulse/config"
	"github.com/emgeorrk/pulse/internal/entity"
	"github.com/emgeorrk/pulse/internal/sensors"
)

// historyLen is how many recent CPU values to keep for the sparkline.
const historyLen = 8

// Public IP lookup schedule: a fresh value is kept for publicIPRefresh; a
// failed lookup is retried after publicIPRetry.
const (
	publicIPRefresh = 15 * time.Minute
	publicIPRetry   = time.Minute
)

// ipResult is one finished public IP lookup.
type ipResult struct {
	info entity.PublicIPInfo
	ok   bool
}

type Monitor struct {
	lastTick    time.Time
	ipFetchedAt time.Time
	store       *config.Store
	prevNet     map[string]entity.NetCounters
	src         *sensors.Sources
	ipCh        chan ipResult
	ip          string
	ipCountry   string
	prevTicks   []entity.CoreTicks
	history     []float64
	sessDown    uint64
	sessUp      uint64
	prevRead    uint64
	prevWrite   uint64
	haveDisk    bool
	ipInFlight  bool
	ipOK        bool
}

func NewMonitor(src *sensors.Sources, store *config.Store) *Monitor {
	return &Monitor{src: src, store: store, ipCh: make(chan ipResult, 1)}
}

// Start launches the sampling loop in its own goroutine (never on the UI
// thread). The first frame arrives after `interval` — with meaningful
// deltas right away. The interval is re-read from settings on every tick,
// so changing it in Settings takes effect without a restart. The channel
// closes when ctx is canceled.
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

			snap := m.sample(ctx)

			// UI hasn't picked up the previous frame yet — just drop it.
			select {
			case out <- snap:
			default:
			}
		}
	}()

	return out
}

// prime takes the first counter readings — the baseline for deltas.
func (m *Monitor) prime() {
	if ticks, err := m.src.CPU.Ticks(); err == nil {
		m.prevTicks = ticks
	}

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

// sample takes one frame of all available metrics.
func (m *Monitor) sample(ctx context.Context) entity.Snapshot { //nolint:cyclop,funlen,gocognit,gocyclo // Each optional source is sampled independently by design.
	m.pollPublicIP(ctx)

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
			net.PublicIP, net.IPCountry = m.ip, m.ipCountry
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
			snap.Temps = new(AggregateTemps(all))
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

	if m.src.System != nil {
		if st, err := m.src.System.System(); err == nil {
			snap.System = &st
		}
	}

	return snap
}

// pollPublicIP drains a finished lookup and starts a new one when the metric
// is enabled and the current value is stale. The HTTP request runs in its
// own goroutine, so a slow provider never blocks the sampling tick.
func (m *Monitor) pollPublicIP(ctx context.Context) {
	if m.src.PublicIP == nil {
		return
	}

	select {
	case res := <-m.ipCh:
		m.ipInFlight = false
		m.ipFetchedAt = time.Now()
		m.ipOK = res.ok

		if res.ok {
			m.ip, m.ipCountry = res.info.IP, res.info.Country
		}
	default:
	}

	if !m.store.Get().ShowPublicIP {
		// Drop the value so re-enabling starts from a fresh lookup.
		m.ip, m.ipCountry = "", ""
		m.ipFetchedAt = time.Time{}

		return
	}

	if m.ipInFlight || !publicIPDue(m.ipFetchedAt, m.ipOK, time.Now()) {
		return
	}

	m.ipInFlight = true

	go func() {
		info, err := m.src.PublicIP.Fetch(ctx)
		m.ipCh <- ipResult{info: info, ok: err == nil}
	}()
}

// publicIPDue reports whether a new lookup should start: immediately when
// none has finished yet, after publicIPRefresh on success, and after
// publicIPRetry on failure.
func publicIPDue(fetchedAt time.Time, lastOK bool, now time.Time) bool {
	if fetchedAt.IsZero() {
		return true
	}

	wait := publicIPRefresh
	if !lastOK {
		wait = publicIPRetry
	}

	return now.Sub(fetchedAt) >= wait
}

// CPUUsage computes load from the delta of cumulative ticks. Ticks are
// 32-bit and wrap around; subtraction in uint32 is correct modulo 2^32.
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

// NetRates computes throughput from the deltas of 32-bit if_data counters
// (subtraction in uint32 is correct on overflow). Interfaces with no
// previous value (they appeared between ticks) are skipped until the next tick.
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

// rate64 computes a rate from 64-bit counters; a counter reset (cur < prev)
// never produces a negative rate.
func rate64(prev, cur uint64, dwellSec float64) float64 {
	if cur < prev {
		return 0
	}

	return float64(cur-prev) / dwellSec
}
