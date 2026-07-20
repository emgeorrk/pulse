//go:build darwin

// Package app wires the layers together: sensors → usecase → tray. It also
// decides which sources are available on this hardware (Caps): a sensor that
// fails at startup disables its own group, not the whole app.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/emgeorrk/pulse/config"
	"github.com/emgeorrk/pulse/internal/controller/tray"
	"github.com/emgeorrk/pulse/internal/entity"
	"github.com/emgeorrk/pulse/internal/sensors"
	"github.com/emgeorrk/pulse/internal/usecase"
	"github.com/emgeorrk/pulse/pkg/format"
	"github.com/emgeorrk/pulse/pkg/pprof"
)

var errMonitorStopped = errors.New("monitor stopped before first sample")

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
func probe(hw entity.HWInfo) (sensors.Sources, entity.Caps, error) { //nolint:cyclop,funlen,gocognit,gocyclo // Hardware probing deliberately isolates optional sensors.
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
		// The lookup itself only runs when the user enables the metric —
		// no network request happens at startup.
		src.PublicIP = sensors.NewPublicIP()

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

	sys := sensors.NewSystem()
	if _, err := sys.System(); err == nil {
		src.System = sys
		caps.System = true
	}

	// ── temps: Intel and Apple Silicon paths differ ──
	if hw.IsAppleSilicon {
		// Apple Silicon: HID is primary for temperatures.
		if hid, err := sensors.NewHID(); err == nil {
			if temps, err := hid.Temps(); err == nil {
				src.Temp = hid

				caps.Temps = true
				for _, r := range temps {
					caps.TempSensors = append(caps.TempSensors, r.Name)
				}
			}
		}
	}

	// SMC: fans on both platforms; Intel temperatures and the narrow Apple
	// Silicon GPU fallback use the same connection.
	if smc, err := sensors.NewSMC(); err == nil { //nolint:nestif // One SMC connection probes several independent capabilities.
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
	pprof.Start() // no-op unless built with -tags debug

	store := loadStore()
	hw := sensors.ReadHWInfo()

	src, caps, err := probe(hw)
	if err != nil {
		return err
	}

	mon := usecase.NewMonitor(&src, store)

	tray.New(store, hw, caps).Run(func(ctx context.Context) <-chan entity.Snapshot {
		return mon.Start(ctx)
	})

	return nil
}

// RunOnce prints one metrics frame to stdout and exits — for checking
// sensors without the UI (pulse -once).
func RunOnce() error { //nolint:cyclop,funlen,gocognit,gocyclo // The diagnostic output intentionally handles each optional metric independently.
	store := loadStore()
	cfg := store.Get()
	hw := sensors.ReadHWInfo()

	src, _, err := probe(hw)
	if err != nil {
		return err
	}

	mon := usecase.NewMonitor(&src, store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	snap, ok := <-mon.Start(ctx)
	if !ok {
		return errMonitorStopped
	}

	dec := cfg.DecimalBytes
	prec := cfg.HigherPrecision
	out := os.Stdout

	perCore := make([]string, len(snap.CPU.Cores))
	for i, c := range snap.CPU.Cores {
		perCore[i] = format.Percent(c, prec)
	}

	fmt.Fprintf(out, "%s · %s · macOS %s · %d cores · apple silicon: %v\n",
		hw.Chip, hw.Model, hw.OSVersion, hw.NumCores, hw.IsAppleSilicon)
	fmt.Fprintf(out, "CPU total: %s (over %v)\n", format.Percent(snap.CPU.Total, prec), cfg.Interval())
	fmt.Fprintf(out, "CPU cores: %s\n", strings.Join(perCore, " "))

	if snap.Freq != nil {
		fmt.Fprintf(out, "CPU freq: %s", format.Hertz(snap.Freq.Max))

		for _, r := range snap.Freq.Clusters {
			fmt.Fprintf(out, " · %s %s", r.Name, format.Hertz(r.Value))
		}

		fmt.Fprintln(out)
	} else {
		fmt.Fprintln(out, "CPU freq: unavailable")
	}

	fmt.Fprintf(out, "Mem: used %s / %s (%s), available %s, free %s, cached %s\n",
		format.Bytes(snap.Mem.Used, dec, prec), format.Bytes(snap.Mem.Total, dec, prec),
		format.Percent(snap.Mem.UsedFraction(), prec),
		format.Bytes(snap.Mem.Available, dec, prec), format.Bytes(snap.Mem.Free, dec, prec),
		format.Bytes(snap.Mem.Cached, dec, prec))
	fmt.Fprintf(out, "Swap: used %s / %s\n",
		format.Bytes(snap.Mem.SwapUsed, dec, prec), format.Bytes(snap.Mem.SwapTotal, dec, prec))

	if snap.System != nil {
		fmt.Fprintf(out, "System: load %s %s %s · up %s · %d procs · %d open files\n",
			format.Load(snap.System.Load1), format.Load(snap.System.Load5),
			format.Load(snap.System.Load15), format.Uptime(snap.System.UptimeSec),
			snap.System.Procs, snap.System.OpenFiles)
	} else {
		fmt.Fprintln(out, "System: unavailable")
	}

	if snap.Net != nil {
		fmt.Fprintf(out, "Net: ↓%s ↑%s", format.Speed(snap.Net.Down, prec), format.Speed(snap.Net.Up, prec))

		for _, i := range snap.Net.Ifaces {
			fmt.Fprintf(out, " · %s ↓%s ↑%s", i.Name, format.SpeedShort(i.Down, prec), format.SpeedShort(i.Up, prec))
		}

		if snap.Net.PublicIP != "" {
			fmt.Fprintf(out, " · public IP %s", format.WithFlag(snap.Net.PublicIP, snap.Net.IPCountry))
		}

		fmt.Fprintln(out)
	} else {
		fmt.Fprintln(out, "Net: unavailable")
	}

	if snap.Disk != nil {
		fmt.Fprintf(out, "Disk: used %s / %s (%s), free %s · R %s W %s · boot R %s W %s\n",
			format.Bytes(snap.Disk.Used, dec, prec), format.Bytes(snap.Disk.Total, dec, prec),
			format.Percent(snap.Disk.UsedFraction(), prec), format.Bytes(snap.Disk.Available, dec, prec),
			format.Speed(snap.Disk.ReadRate, prec), format.Speed(snap.Disk.WriteRate, prec),
			format.Bytes(snap.Disk.ReadTotal, dec, prec), format.Bytes(snap.Disk.WriteTotal, dec, prec))
	} else {
		fmt.Fprintln(out, "Disk: unavailable")
	}

	f := cfg.TempUnit == config.Fahrenheit
	if snap.Temps != nil {
		fmt.Fprintf(out, "Temp: CPU %s · GPU %s · avg %s · hottest %s (%s) · coolest %s (%s) · %d sensors\n",
			format.Temp(snap.Temps.CPU, f, prec), format.Temp(snap.Temps.GPU, f, prec),
			format.Temp(snap.Temps.Avg, f, prec),
			format.Temp(snap.Temps.Hottest.Value, f, prec), snap.Temps.Hottest.Name,
			format.Temp(snap.Temps.Coolest.Value, f, prec), snap.Temps.Coolest.Name,
			len(snap.Temps.All))

		for _, r := range snap.Temps.All {
			fmt.Fprintf(out, "  %-40s %s\n", r.Name, format.Temp(r.Value, f, prec))
		}
	} else {
		fmt.Fprintln(out, "Temp: unavailable")
	}

	if len(snap.Fans) > 0 {
		for _, fan := range snap.Fans {
			fmt.Fprintf(out, "%s: %s (min %s, max %s)\n",
				fan.Name, format.RPM(fan.RPM), format.RPM(fan.Min), format.RPM(fan.Max))
		}
	} else {
		fmt.Fprintln(out, "Fans: unavailable")
	}

	if snap.GPU != nil {
		fmt.Fprintf(out, "GPU: %s\n", format.Percent(snap.GPU.Utilization, prec))
	} else {
		fmt.Fprintln(out, "GPU: unavailable")
	}

	if snap.Power != nil {
		fmt.Fprintf(out, "Power: total %s · CPU %s · GPU %s · ANE %s\n",
			format.Watts(snap.Power.Total), format.Watts(snap.Power.CPU),
			format.Watts(snap.Power.GPU), format.Watts(snap.Power.ANE))
	} else {
		fmt.Fprintln(out, "Power: unavailable")
	}

	if snap.Battery != nil {
		b := snap.Battery

		state := "discharging"
		if b.Charging {
			state = "charging"
		} else if b.External {
			state = "AC"
		}

		fmt.Fprintf(out, "Battery: %s (raw %s) (%s) · health %s · %d cycles · %s · %s · %s\n",
			format.Percent(b.Percent, prec), format.Percent(b.RawPercent, prec), state,
			format.Percent(b.Health, prec), b.Cycles,
			format.Temp(b.TempC, f, prec), format.Volts(b.Volts), format.Watts(b.Watts))
	} else {
		fmt.Fprintln(out, "Battery: unavailable")
	}

	return nil
}
