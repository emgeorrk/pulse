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
	if caps.Temps {
		groups = append(groups, tempGroup(caps.TempSensors))
	}
	if caps.Fans {
		groups = append(groups, fanGroup(caps.FanCount))
	}
	if caps.Volts {
		groups = append(groups, voltGroup(caps.VoltSensors))
	}
	if caps.Net {
		groups = append(groups, netGroup(caps.NetIfaces))
	}
	if caps.Disk {
		groups = append(groups, diskGroup())
	}
	if caps.GPU {
		groups = append(groups, gpuGroup())
	}
	if caps.Power {
		groups = append(groups, powerGroup())
	}
	if caps.Battery {
		groups = append(groups, batteryGroup())
	}
	return groups
}

func gpuGroup() group {
	return group{
		emoji: "🎮",
		label: "GPU",
		aggregate: func(s entity.Snapshot, c config.Config) string {
			if s.GPU == nil {
				return "—"
			}
			return format.Percent(s.GPU.Utilization)
		},
		metrics: []metric{
			{
				id:    "gpu.usage",
				label: "Usage",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.GPU == nil {
						return "—"
					}
					return format.Percent(s.GPU.Utilization)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.GPU == nil {
						return "GPU —"
					}
					return "GPU " + format.Percent(s.GPU.Utilization)
				},
			},
		},
	}
}

func powerGroup() group {
	watts := func(get func(*entity.PowerStats) float64) func(entity.Snapshot, config.Config) string {
		return func(s entity.Snapshot, c config.Config) string {
			if s.Power == nil {
				return "—"
			}
			return format.Watts(get(s.Power))
		}
	}
	return group{
		emoji: "🔌",
		label: "Power",
		aggregate: func(s entity.Snapshot, c config.Config) string {
			if s.Power == nil {
				return "—"
			}
			return format.Watts(s.Power.Total)
		},
		metrics: []metric{
			{
				id:    "power.total",
				label: "Total",
				menu:  watts(func(p *entity.PowerStats) float64 { return p.Total }),
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Power == nil {
						return "—W"
					}
					return format.Watts(s.Power.Total)
				},
			},
			{id: "power.cpu", label: "CPU", menu: watts(func(p *entity.PowerStats) float64 { return p.CPU })},
			{id: "power.gpu", label: "GPU", menu: watts(func(p *entity.PowerStats) float64 { return p.GPU })},
			{id: "power.ane", label: "ANE", menu: watts(func(p *entity.PowerStats) float64 { return p.ANE })},
		},
	}
}

func batteryGroup() group {
	return group{
		emoji: "🔋",
		label: "Battery",
		aggregate: func(s entity.Snapshot, c config.Config) string {
			if s.Battery == nil {
				return "—"
			}
			state := ""
			if s.Battery.Charging {
				state = " ⚡"
			}
			return format.Percent(s.Battery.Percent) + state
		},
		metrics: []metric{
			{
				id:    "batt.pct",
				label: "Charge",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Battery == nil {
						return "—"
					}
					return format.Percent(s.Battery.Percent)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Battery == nil {
						return "BAT —"
					}
					return "BAT " + format.Percent(s.Battery.Percent)
				},
			},
			{
				id:    "batt.state",
				label: "State",
				menu: func(s entity.Snapshot, c config.Config) string {
					switch {
					case s.Battery == nil:
						return "—"
					case s.Battery.Charging:
						return "Charging"
					case s.Battery.External:
						return "AC power"
					default:
						return "Discharging"
					}
				},
			},
			{
				id:    "batt.time",
				label: "Time left",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Battery == nil || s.Battery.MinutesLeft < 0 {
						return "—"
					}
					return fmt.Sprintf("%dh %02dm", s.Battery.MinutesLeft/60, s.Battery.MinutesLeft%60)
				},
			},
			{
				id:    "batt.power",
				label: "Power rate",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Battery == nil {
						return "—"
					}
					return format.Watts(s.Battery.Watts)
				},
			},
			{
				id:    "batt.health",
				label: "Health",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Battery == nil || s.Battery.Health == 0 {
						return "—"
					}
					return format.Percent(s.Battery.Health)
				},
			},
			{
				id:    "batt.cycles",
				label: "Cycles",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Battery == nil {
						return "—"
					}
					return fmt.Sprintf("%d", s.Battery.Cycles)
				},
			},
			{
				id:    "batt.temp",
				label: "Temperature",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Battery == nil {
						return "—"
					}
					return format.Temp(s.Battery.TempC, c.TempUnit == config.Fahrenheit)
				},
			},
			{
				id:    "batt.volts",
				label: "Voltage",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Battery == nil {
						return "—"
					}
					return format.Volts(s.Battery.Volts)
				},
			},
		},
	}
}

func tempGroup(sensorNames []string) group {
	g := group{
		emoji: "🌡",
		label: "Temp",
		aggregate: func(s entity.Snapshot, c config.Config) string {
			if s.Temps == nil {
				return "—"
			}
			if s.Temps.CPU > 0 {
				return format.Temp(s.Temps.CPU, c.TempUnit == config.Fahrenheit)
			}
			return format.Temp(s.Temps.Hottest.Value, c.TempUnit == config.Fahrenheit)
		},
		metrics: []metric{
			{
				id:    "temp.cpu",
				label: "CPU",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Temps == nil || s.Temps.CPU == 0 {
						return "—"
					}
					return format.Temp(s.Temps.CPU, c.TempUnit == config.Fahrenheit)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Temps == nil || s.Temps.CPU == 0 {
						return "—°"
					}
					return format.TempShort(s.Temps.CPU, c.TempUnit == config.Fahrenheit)
				},
			},
			{
				id:    "temp.gpu",
				label: "GPU",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Temps == nil || s.Temps.GPU == 0 {
						return "—"
					}
					return format.Temp(s.Temps.GPU, c.TempUnit == config.Fahrenheit)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Temps == nil || s.Temps.GPU == 0 {
						return "G—°"
					}
					return "G" + format.TempShort(s.Temps.GPU, c.TempUnit == config.Fahrenheit)
				},
			},
			{
				id:    "temp.hottest",
				label: "Hottest",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Temps == nil || s.Temps.Hottest.Name == "" {
						return "—"
					}
					return format.Temp(s.Temps.Hottest.Value, c.TempUnit == config.Fahrenheit) +
						" (" + s.Temps.Hottest.Name + ")"
				},
			},
		},
	}
	for _, name := range sensorNames {
		name := name
		g.metrics = append(g.metrics, metric{
			id:    entity.MetricID("temp.sensor." + name),
			label: name,
			menu: func(s entity.Snapshot, c config.Config) string {
				if s.Temps != nil {
					for _, r := range s.Temps.All {
						if r.Name == name {
							return format.Temp(r.Value, c.TempUnit == config.Fahrenheit)
						}
					}
				}
				return "—"
			},
		})
	}
	return g
}

func fanGroup(count int) group {
	g := group{
		emoji: "🌀",
		label: "Fans",
		aggregate: func(s entity.Snapshot, c config.Config) string {
			max := 0.0
			for _, f := range s.Fans {
				if f.RPM > max {
					max = f.RPM
				}
			}
			return format.RPM(max)
		},
	}
	for i := 0; i < count; i++ {
		idx := i
		g.metrics = append(g.metrics, metric{
			id:    entity.MetricID(fmt.Sprintf("fan.%d", idx+1)),
			label: fmt.Sprintf("Fan %d", idx+1),
			menu: func(s entity.Snapshot, c config.Config) string {
				if idx < len(s.Fans) {
					f := s.Fans[idx]
					if f.Max > 0 {
						return fmt.Sprintf("%s (%s)", format.RPM(f.RPM), format.Percent(f.RPM/f.Max))
					}
					return format.RPM(f.RPM)
				}
				return "—"
			},
			bar: func(s entity.Snapshot, c config.Config) string {
				if idx < len(s.Fans) {
					return fmt.Sprintf("%drpm", int(s.Fans[idx].RPM))
				}
				return "—rpm"
			},
		})
	}
	return g
}

func voltGroup(sensorNames []string) group {
	g := group{
		emoji: "⚡",
		label: "Voltage",
		aggregate: func(s entity.Snapshot, c config.Config) string {
			if len(s.Volts) == 0 {
				return "—"
			}
			return format.Volts(s.Volts[0].Value)
		},
	}
	for _, name := range sensorNames {
		name := name
		g.metrics = append(g.metrics, metric{
			id:    entity.MetricID("volt.sensor." + name),
			label: name,
			menu: func(s entity.Snapshot, c config.Config) string {
				for _, r := range s.Volts {
					if r.Name == name {
						return format.Volts(r.Value)
					}
				}
				return "—"
			},
		})
	}
	return g
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
