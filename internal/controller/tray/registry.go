//go:build darwin

package tray

import (
	"fmt"
	"strings"

	"github.com/emgeorrk/pulse/config"
	"github.com/emgeorrk/pulse/internal/controller/tray/icons"
	"github.com/emgeorrk/pulse/internal/entity"
	"github.com/emgeorrk/pulse/pkg/format"
)

// metric is one row in the dropdown. bar defines a compact value for the
// menu bar; when bar == nil, the menu value is shown in the bar. What
// precedes the value there depends on the bar-label style: tag in text
// mode, the visual (emoji or icon) plus sym in visual mode. sym keeps
// otherwise ambiguous metrics apart when the visual is group-level (net
// ↓/↑, disk R/W, swap). emoji, icon and iconQual are filled from the group
// in fill(); a metric-level icon (network arrows) already carries the
// qualifier, so its iconQual stays empty.
type metric struct {
	menu     func(s entity.Snapshot, c config.Config) string
	bar      func(s entity.Snapshot, c config.Config) string
	id       entity.MetricID
	label    string
	tag      string
	sym      string
	icon     string
	emoji    string
	iconQual string
}

// barValue is the compact value for the menu bar; without an explicit bar
// func, the menu value is used.
func (m metric) barValue(s entity.Snapshot, c config.Config) string {
	if m.bar != nil {
		return m.bar(s, c)
	}

	return m.menu(s, c)
}

// barPart renders the metric for the menu bar per the current settings:
// an optional icon key (drawn by the systray layer) plus the text.
func (m metric) barPart(s entity.Snapshot, c config.Config) (iconKey, text string) {
	val := m.barValue(s, c)
	if c.BarLabels != config.BarVisual {
		return "", m.tag + val
	}

	if c.VisualStyle == config.VisualGnome {
		return m.icon, " " + m.iconQual + val
	}

	return "", m.emoji + " " + m.sym + val
}

// fill completes the metric with group-level visuals.
func (m metric) fill(g group) metric {
	if m.icon == "" {
		m.icon = g.icon
		m.iconQual = m.sym
	}

	if m.emoji == "" {
		m.emoji = g.emoji
	}

	return m
}

// group is a Vitals-style metric group: a live aggregate in the header,
// metrics in the submenu.
type group struct {
	emoji     string
	icon      string // key into the icons package, for the gnome style
	label     string
	aggregate func(s entity.Snapshot, c config.Config) string
	metrics   []metric
}

// buildGroups builds the group registry for this hardware: groups
// unavailable per caps aren't created at all. Group order = dropdown order.
func buildGroups(hw entity.HWInfo, caps entity.Caps) []group {
	cpu := group{
		emoji: "⚙️",
		icon:  icons.CPU,
		label: "CPU",
		aggregate: func(s entity.Snapshot, c config.Config) string {
			return format.Percent(s.CPU.Total)
		},
		metrics: []metric{
			{
				id:    "cpu.total",
				label: "Usage",
				tag:   "CPU ",
				menu: func(s entity.Snapshot, c config.Config) string {
					return format.Percent(s.CPU.Total)
				},
			},
		},
	}
	if caps.Freq {
		cpu.metrics = append(cpu.metrics,
			metric{
				id:    "cpu.freq",
				label: "Frequency",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Freq == nil {
						return "—"
					}

					return format.Hertz(s.Freq.Max)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Freq == nil {
						return "—GHz"
					}

					return strings.ReplaceAll(format.Hertz(s.Freq.Max), " ", "")
				},
			})

		for _, cl := range caps.FreqClusters {
			cpu.metrics = append(cpu.metrics, metric{
				id:    entity.MetricID("cpu.freq." + cl),
				label: cl,
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Freq != nil {
						for _, r := range s.Freq.Clusters {
							if r.Name == cl {
								return format.Hertz(r.Value)
							}
						}
					}

					return "—"
				},
			})
		}
	}

	for i := 0; i < hw.NumCores; i++ {
		core := i // capture the index
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
		icon:  icons.Memory,
		label: "Memory",
		aggregate: func(s entity.Snapshot, c config.Config) string {
			return format.Percent(s.Mem.UsedFraction())
		},
		metrics: []metric{
			{
				id:    "mem.usage",
				label: "Usage",
				tag:   "MEM ",
				menu: func(s entity.Snapshot, c config.Config) string {
					return format.Percent(s.Mem.UsedFraction())
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
				bar: func(s entity.Snapshot, c config.Config) string {
					return format.BytesShort(s.Mem.Available, c.DecimalBytes)
				},
			},
			{
				id:    "mem.physical",
				label: "Physical",
				menu: func(s entity.Snapshot, c config.Config) string {
					return format.Bytes(s.Mem.Total, c.DecimalBytes)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					return format.BytesShort(s.Mem.Total, c.DecimalBytes)
				},
			},
			{
				id:    "swap.used",
				label: "Swap",
				tag:   "SW ",
				sym:   "SW ",
				menu: func(s entity.Snapshot, c config.Config) string {
					return format.Bytes(s.Mem.SwapUsed, c.DecimalBytes)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					return format.BytesShort(s.Mem.SwapUsed, c.DecimalBytes)
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
		icon:  icons.GPU,
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
				tag:   "GPU ",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.GPU == nil {
						return "—"
					}

					return format.Percent(s.GPU.Utilization)
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
		icon:  icons.Voltage, // the set has no dedicated power icon; the bolt fits watts
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
		icon:  icons.Battery,
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
				tag:   "BAT ",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Battery == nil {
						return "—"
					}

					return format.Percent(s.Battery.Percent)
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
		emoji: "🌡️",
		icon:  icons.Temperature,
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
				tag:   "G",
				sym:   "G",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Temps == nil || s.Temps.GPU == 0 {
						return "—"
					}

					return format.Temp(s.Temps.GPU, c.TempUnit == config.Fahrenheit)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Temps == nil || s.Temps.GPU == 0 {
						return "—°"
					}

					return format.TempShort(s.Temps.GPU, c.TempUnit == config.Fahrenheit)
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
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Temps == nil || s.Temps.Hottest.Name == "" {
						return "—°"
					}

					return format.TempShort(s.Temps.Hottest.Value, c.TempUnit == config.Fahrenheit)
				},
			},
		},
	}

	for _, name := range sensorNames {
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
		icon:  icons.Fan,
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
		icon:  icons.Voltage,
		label: "Voltage",
		aggregate: func(s entity.Snapshot, c config.Config) string {
			if len(s.Volts) == 0 {
				return "—"
			}

			return format.Volts(s.Volts[0].Value)
		},
	}

	for _, name := range sensorNames {
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
		icon:  icons.Network,
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
				tag:   "↓",
				sym:   "↓",
				icon:  icons.NetworkDownload,
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Net == nil {
						return "—"
					}

					return format.Speed(s.Net.Down)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Net == nil {
						return "—"
					}

					return format.SpeedShort(s.Net.Down)
				},
			},
			{
				id:    "net.up",
				label: "Upload",
				tag:   "↑",
				sym:   "↑",
				icon:  icons.NetworkUpload,
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Net == nil {
						return "—"
					}

					return format.Speed(s.Net.Up)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Net == nil {
						return "—"
					}

					return format.SpeedShort(s.Net.Up)
				},
			},
			{
				id:    "net.session.down",
				label: "Session down",
				tag:   "↓",
				sym:   "↓",
				icon:  icons.NetworkDownload,
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Net == nil {
						return "—"
					}

					return format.Bytes(s.Net.SessionDown, c.DecimalBytes)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Net == nil {
						return "—"
					}

					return format.BytesShort(s.Net.SessionDown, c.DecimalBytes)
				},
			},
			{
				id:    "net.session.up",
				label: "Session up",
				tag:   "↑",
				sym:   "↑",
				icon:  icons.NetworkUpload,
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Net == nil {
						return "—"
					}

					return format.Bytes(s.Net.SessionUp, c.DecimalBytes)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Net == nil {
						return "—"
					}

					return format.BytesShort(s.Net.SessionUp, c.DecimalBytes)
				},
			},
		},
	}

	for _, name := range ifaces {
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
		icon:  icons.Storage,
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
				tag:   "DSK ",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}

					return format.Percent(s.Disk.UsedFraction())
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
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}

					return format.BytesShort(s.Disk.Used, c.DecimalBytes)
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
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}

					return format.BytesShort(s.Disk.Available, c.DecimalBytes)
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
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}

					return format.BytesShort(s.Disk.Total, c.DecimalBytes)
				},
			},
			{
				id:    "disk.read",
				label: "Read rate",
				tag:   "R ",
				sym:   "R ",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}

					return format.Speed(s.Disk.ReadRate)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}

					return format.SpeedShort(s.Disk.ReadRate)
				},
			},
			{
				id:    "disk.write",
				label: "Write rate",
				tag:   "W ",
				sym:   "W ",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}

					return format.Speed(s.Disk.WriteRate)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}

					return format.SpeedShort(s.Disk.WriteRate)
				},
			},
			{
				id:    "disk.read.total",
				label: "Read total (boot)",
				tag:   "R ",
				sym:   "R ",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}

					return format.Bytes(s.Disk.ReadTotal, c.DecimalBytes)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}

					return format.BytesShort(s.Disk.ReadTotal, c.DecimalBytes)
				},
			},
			{
				id:    "disk.write.total",
				label: "Write total (boot)",
				tag:   "W ",
				sym:   "W ",
				menu: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}

					return format.Bytes(s.Disk.WriteTotal, c.DecimalBytes)
				},
				bar: func(s entity.Snapshot, c config.Config) string {
					if s.Disk == nil {
						return "—"
					}

					return format.BytesShort(s.Disk.WriteTotal, c.DecimalBytes)
				},
			},
		},
	}
}
