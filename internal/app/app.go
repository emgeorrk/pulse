//go:build darwin

// Package app wires the layers together: sensors → usecase → tray. It also
// decides which sources are available on this hardware (Caps): a sensor that
// fails at startup disables its own group, not the whole app.
package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/emgeorrk/pulse/config"
	"github.com/emgeorrk/pulse/internal/controller/tray"
	"github.com/emgeorrk/pulse/internal/entity"
	"github.com/emgeorrk/pulse/internal/sensors"
	"github.com/emgeorrk/pulse/internal/usecase"
	"github.com/emgeorrk/pulse/pkg/format"
)

func loadStore() *config.Store {
	path, err := config.DefaultPath()
	if err != nil {
		path = "" // settings will live in memory only
	}

	return config.Load(path)
}

// probe collects the available sources and Caps for the UI. Each sensor is
// probed with a real read: if it doesn't respond, its group is disabled but
// the app keeps running.
func probe(hw entity.HWInfo) (sensors.Sources, entity.Caps, error) {
	mem, err := sensors.NewMem()
	if err != nil {
		return sensors.Sources{}, entity.Caps{}, fmt.Errorf("memory sensor: %w", err)
	}

	src := sensors.Sources{CPU: sensors.NewCPU(), Mem: mem}

	var caps entity.Caps

	net := sensors.NewNet()
	if counters, err := net.Counters(); err == nil {
		src.Net = net
		caps.Net = true

		for _, c := range counters {
			// menu only lists interfaces that already had traffic at startup
			if c.Rx > 0 || c.Tx > 0 {
				caps.NetIfaces = append(caps.NetIfaces, c.Name)
			}
		}
	}

	disk := sensors.NewDisk()
	if _, err := disk.Usage(); err == nil {
		src.Disk = disk
		caps.Disk = true
	}

	// ── temps and voltage: Intel and Apple Silicon paths differ ──
	if hw.IsAppleSilicon {
		// Apple Silicon: HID is primary for temperatures and voltage.
		if hid, err := sensors.NewHID(); err == nil {
			if temps, err := hid.Temps(); err == nil {
				src.Temp = hid

				caps.Temps = true
				for _, r := range temps {
					caps.TempSensors = append(caps.TempSensors, r.Name)
				}
			}

			if volts, err := hid.Voltages(); err == nil {
				src.Volt = hid

				caps.Volts = true
				for _, r := range volts {
					caps.VoltSensors = append(caps.VoltSensors, r.Name)
				}
			}
		}
	}

	// SMC: fans on both platforms; Intel temperatures and the narrow Apple
	// Silicon GPU fallback use the same connection.
	if smc, err := sensors.NewSMC(); err == nil {
		if !hw.IsAppleSilicon {
			if temps, err := smc.Temps(); err == nil {
				src.Temp = smc

				caps.Temps = true
				for _, r := range temps {
					caps.TempSensors = append(caps.TempSensors, r.Name)
				}
			}
		} else if names := smc.GPUTempSensors(); len(names) > 0 {
			// Apple Silicon: HID exposes no GPU-named temperature sensors on
			// some generations (M5: only "PMU tdie*"), but SMC does via Tg*
			// keys. Merge them so AggregateTemps fills the GPU slot.
			gpuSrc := sensors.TempFunc(smc.GPUTemps)
			if src.Temp != nil {
				src.Temp = sensors.NewMultiTemp(src.Temp, gpuSrc)
			} else {
				src.Temp = gpuSrc // HID unavailable but SMC GPU keys readable
				caps.Temps = true
			}

			caps.TempSensors = append(caps.TempSensors, names...)
		}

		if fans, err := smc.Fans(); err == nil {
			src.Fan = smc
			caps.Fans = true
			caps.FanCount = len(fans)
		}
	}

	batt := sensors.NewBattery()
	if _, err := batt.Battery(); err == nil {
		src.Battery = batt
		caps.Battery = true
	}

	gpu := sensors.NewGPU()
	if _, err := gpu.GPU(); err == nil {
		src.GPU = gpu
		caps.GPU = true
	}

	if ior, err := sensors.NewIOReport(); err == nil {
		src.Power = ior
		caps.Power = true

		if ior.HasFreq() {
			src.Freq = ior
			caps.Freq = true
			caps.FreqClusters = ior.FreqClusters()
		}
	}

	return src, caps, nil
}

// Run starts the menu bar app; blocks until it quits.
func Run() error {
	store := loadStore()
	hw := sensors.ReadHWInfo()

	src, caps, err := probe(hw)
	if err != nil {
		return err
	}

	mon := usecase.NewMonitor(src, store)

	tray.New(store, hw, caps).Run(func(ctx context.Context) <-chan entity.Snapshot {
		return mon.Start(ctx)
	})

	return nil
}

// RunOnce prints one metrics frame to stdout and exits — for checking
// sensors without the UI (pulse -once).
func RunOnce() error {
	store := loadStore()
	cfg := store.Get()
	hw := sensors.ReadHWInfo()

	src, _, err := probe(hw)
	if err != nil {
		return err
	}

	mon := usecase.NewMonitor(src, store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	snap, ok := <-mon.Start(ctx)
	if !ok {
		return fmt.Errorf("monitor stopped before first sample")
	}

	dec := cfg.DecimalBytes

	perCore := make([]string, len(snap.CPU.Cores))
	for i, c := range snap.CPU.Cores {
		perCore[i] = format.Percent(c)
	}

	fmt.Printf("%s · %s · macOS %s · %d cores · apple silicon: %v\n",
		hw.Chip, hw.Model, hw.OSVersion, hw.NumCores, hw.IsAppleSilicon)
	fmt.Printf("CPU total: %s (over %v)\n", format.Percent(snap.CPU.Total), cfg.Interval())
	fmt.Printf("CPU cores: %s\n", strings.Join(perCore, " "))

	if snap.Freq != nil {
		fmt.Printf("CPU freq: %s", format.Hertz(snap.Freq.Max))

		for _, r := range snap.Freq.Clusters {
			fmt.Printf(" · %s %s", r.Name, format.Hertz(r.Value))
		}

		fmt.Println()
	} else {
		fmt.Println("CPU freq: unavailable")
	}

	fmt.Printf("Mem: used %s / %s (%s), available %s, free %s\n",
		format.Bytes(snap.Mem.Used, dec), format.Bytes(snap.Mem.Total, dec),
		format.Percent(snap.Mem.UsedFraction()),
		format.Bytes(snap.Mem.Available, dec), format.Bytes(snap.Mem.Free, dec))
	fmt.Printf("Swap: used %s / %s\n",
		format.Bytes(snap.Mem.SwapUsed, dec), format.Bytes(snap.Mem.SwapTotal, dec))

	if snap.Net != nil {
		fmt.Printf("Net: ↓%s ↑%s", format.Speed(snap.Net.Down), format.Speed(snap.Net.Up))

		for _, i := range snap.Net.Ifaces {
			fmt.Printf(" · %s ↓%s ↑%s", i.Name, format.SpeedShort(i.Down), format.SpeedShort(i.Up))
		}

		fmt.Println()
	} else {
		fmt.Println("Net: unavailable")
	}

	if snap.Disk != nil {
		fmt.Printf("Disk: used %s / %s (%s), free %s · R %s W %s · boot R %s W %s\n",
			format.Bytes(snap.Disk.Used, dec), format.Bytes(snap.Disk.Total, dec),
			format.Percent(snap.Disk.UsedFraction()), format.Bytes(snap.Disk.Available, dec),
			format.Speed(snap.Disk.ReadRate), format.Speed(snap.Disk.WriteRate),
			format.Bytes(snap.Disk.ReadTotal, dec), format.Bytes(snap.Disk.WriteTotal, dec))
	} else {
		fmt.Println("Disk: unavailable")
	}

	f := cfg.TempUnit == config.Fahrenheit
	if snap.Temps != nil {
		fmt.Printf("Temp: CPU %s · GPU %s · hottest %s (%s) · %d sensors\n",
			format.Temp(snap.Temps.CPU, f), format.Temp(snap.Temps.GPU, f),
			format.Temp(snap.Temps.Hottest.Value, f), snap.Temps.Hottest.Name,
			len(snap.Temps.All))

		for _, r := range snap.Temps.All {
			fmt.Printf("  %-40s %s\n", r.Name, format.Temp(r.Value, f))
		}
	} else {
		fmt.Println("Temp: unavailable")
	}

	if len(snap.Fans) > 0 {
		for _, fan := range snap.Fans {
			fmt.Printf("%s: %s (min %s, max %s)\n",
				fan.Name, format.RPM(fan.RPM), format.RPM(fan.Min), format.RPM(fan.Max))
		}
	} else {
		fmt.Println("Fans: unavailable")
	}

	if len(snap.Volts) > 0 {
		fmt.Printf("Voltage: %d sensors\n", len(snap.Volts))

		for _, r := range snap.Volts {
			fmt.Printf("  %-40s %s\n", r.Name, format.Volts(r.Value))
		}
	} else {
		fmt.Println("Voltage: unavailable")
	}

	if snap.GPU != nil {
		fmt.Printf("GPU: %s\n", format.Percent(snap.GPU.Utilization))
	} else {
		fmt.Println("GPU: unavailable")
	}

	if snap.Power != nil {
		fmt.Printf("Power: total %s · CPU %s · GPU %s · ANE %s\n",
			format.Watts(snap.Power.Total), format.Watts(snap.Power.CPU),
			format.Watts(snap.Power.GPU), format.Watts(snap.Power.ANE))
	} else {
		fmt.Println("Power: unavailable")
	}

	if snap.Battery != nil {
		b := snap.Battery

		state := "discharging"
		if b.Charging {
			state = "charging"
		} else if b.External {
			state = "AC"
		}

		fmt.Printf("Battery: %s (%s) · health %s · %d cycles · %s · %s · %s\n",
			format.Percent(b.Percent), state, format.Percent(b.Health), b.Cycles,
			format.Temp(b.TempC, f), format.Volts(b.Volts), format.Watts(b.Watts))
	} else {
		fmt.Println("Battery: unavailable")
	}

	return nil
}
