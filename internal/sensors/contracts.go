// Package sensors is the data-source layer (analogous to repo in
// go-clean-template). The interfaces below are the branch point for
// platform implementations: the Mach API (this increment, identical on
// Intel and Apple Silicon), then SMC on Intel and
// IOHIDEventSystemClient/IOReport on Apple Silicon.
package sensors

//go:generate go tool mockgen -source=contracts.go -destination=mocks/sensors.go -package=mocks

import (
	"context"

	"github.com/emgeorrk/pulse/internal/entity"
)

// CPUSource returns cumulative per-core load ticks; usecase computes load
// from the delta between two reads.
type CPUSource interface {
	Ticks() ([]entity.CoreTicks, error)
}

// MemSource returns the current memory and swap state.
type MemSource interface {
	Read() (entity.MemStats, error)
}

// NetSource returns cumulative per-interface traffic counters; usecase
// computes throughput from the delta.
type NetSource interface {
	Counters() ([]entity.NetCounters, error)
}

// DiskSource returns root-volume usage and cumulative read/write counters
// (IOKit, since boot).
type DiskSource interface {
	Usage() (entity.DiskUsage, error)
	IOTotals() (read, write uint64, err error)
}

// TempSource returns readings for every temperature sensor, in °C.
// Implementations: HID sensor hub on Apple Silicon, SMC keys on Intel.
type TempSource interface {
	Temps() ([]entity.Reading, error)
}

// FanSource returns fan RPMs (SMC on both platforms).
type FanSource interface {
	Fans() ([]entity.Fan, error)
}

// SystemSource returns system-wide state: load averages, uptime, process
// and open-file counts (sysctl + libproc).
type SystemSource interface {
	System() (entity.SystemStats, error)
}

// PublicIPSource looks up the machine's public IP address (HTTPS providers).
// Unlike the hardware sources it is queried on its own slow schedule, and
// only when the user enables the metric.
type PublicIPSource interface {
	Fetch(ctx context.Context) (entity.PublicIPInfo, error)
}

// BatterySource returns battery state (IORegistry AppleSmartBattery).
type BatterySource interface {
	Battery() (entity.BatteryStats, error)
}

// GPUSource returns GPU utilization (IOAccelerator PerformanceStatistics).
type GPUSource interface {
	GPU() (entity.GPUStats, error)
}

// PowerSource returns CPU/GPU/ANE power in watts, averaged since the last
// call (IOReport Energy Model — cumulative energy counters).
type PowerSource interface {
	Power() (entity.PowerStats, error)
}

// FreqSource returns the weighted-average CPU frequency per cluster
// (IOReport performance states — Apple Silicon only, best effort).
type FreqSource interface {
	Frequency() (entity.FreqStats, error)
}

// Sources holds the sources collected at startup; nil means unavailable on
// this hardware (its group is hidden). CPU and Mem are mandatory.
type Sources struct {
	CPU      CPUSource
	Mem      MemSource
	Net      NetSource
	Disk     DiskSource
	Temp     TempSource
	Fan      FanSource
	Battery  BatterySource
	GPU      GPUSource
	Power    PowerSource
	Freq     FreqSource
	System   SystemSource
	PublicIP PublicIPSource
}
