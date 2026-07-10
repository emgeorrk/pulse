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

// buildGroups собирает реестр групп для этого железа: недоступные по caps
// группы не создаются вовсе. Порядок групп = порядок в дропдауне.
func buildGroups(hw entity.HWInfo, caps entity.Caps) []group {
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

	groups := []group{cpu, mem}
	if caps.Net {
		groups = append(groups, netGroup(caps.NetIfaces))
	}
	if caps.Disk {
		groups = append(groups, diskGroup())
	}
	return groups
}

func netGroup(ifaces []string) group {
	g := group{
		emoji: "📶",
		label: "Network",
		aggregate: func(s entity.Snapshot, c config.Config) string {
			if s.Net == nil {
				return "—"
			}
			return "↓" + format.SpeedShort(s.Net.Down) + " ↑" + format.SpeedShort(s.Net.Up)
		},
		metrics: []metric{
			{
				id:    "net.down",
				label: "Download",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Net == nil {
						return "—"
					}
					return format.Speed(s.Net.Down)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Net == nil {
						return "↓—"
					}
					return "↓" + format.SpeedShort(s.Net.Down)
				},
			},
			{
				id:    "net.up",
				label: "Upload",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Net == nil {
						return "—"
					}
					return format.Speed(s.Net.Up)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Net == nil {
						return "↑—"
					}
					return "↑" + format.SpeedShort(s.Net.Up)
				},
			},
			{
				id:    "net.session.down",
				label: "Session down",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Net == nil {
						return "—"
					}
					return format.Bytes(s.Net.SessionDown, c.DecimalBytes)
				},
			},
			{
				id:    "net.session.up",
				label: "Session up",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Net == nil {
						return "—"
					}
					return format.Bytes(s.Net.SessionUp, c.DecimalBytes)
				},
			},
		},
	}
	for _, name := range ifaces {
		name := name
		g.metrics = append(g.metrics, metric{
			id:    entity.MetricID("net.iface." + name),
			label: name,
			menu: func(s entity.Snapshot, c config.Config) string {
				if s.Net != nil {
					for _, i := range s.Net.Ifaces {
						if i.Name == name {
							return "↓" + format.SpeedShort(i.Down) + " ↑" + format.SpeedShort(i.Up)
						}
					}
				}
				return "idle"
			},
		})
	}
	return g
}

func diskGroup() group {
	return group{
		emoji: "💽",
		label: "Storage",
		aggregate: func(s entity.Snapshot, c config.Config) string {
			if s.Disk == nil {
				return "—"
			}
			return format.Percent(s.Disk.UsedFraction())
		},
		metrics: []metric{
			{
				id:    "disk.usage",
				label: "Usage",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}
					return format.Percent(s.Disk.UsedFraction())
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "DSK —"
					}
					return "DSK " + format.Percent(s.Disk.UsedFraction())
				},
			},
			{
				id:    "disk.used",
				label: "Used",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}
					return format.Bytes(s.Disk.Used, c.DecimalBytes)
				},
			},
			{
				id:    "disk.free",
				label: "Free",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}
					return format.Bytes(s.Disk.Available, c.DecimalBytes)
				},
			},
			{
				id:    "disk.total",
				label: "Total",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}
					return format.Bytes(s.Disk.Total, c.DecimalBytes)
				},
			},
			{
				id:    "disk.read",
				label: "Read rate",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}
					return format.Speed(s.Disk.ReadRate)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "R —"
					}
					return "R " + format.SpeedShort(s.Disk.ReadRate)
				},
			},
			{
				id:    "disk.write",
				label: "Write rate",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}
					return format.Speed(s.Disk.WriteRate)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "W —"
					}
					return "W " + format.SpeedShort(s.Disk.WriteRate)
				},
			},
			{
				id:    "disk.read.total",
				label: "Read total (boot)",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}
					return format.Bytes(s.Disk.ReadTotal, c.DecimalBytes)
				},
			},
			{
				id:    "disk.write.total",
				label: "Write total (boot)",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}
					return format.Bytes(s.Disk.WriteTotal, c.DecimalBytes)
				},
			},
		},
	}
}
