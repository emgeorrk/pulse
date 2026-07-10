//go:build darwin

// Package app связывает слои: sensors → usecase → tray. Здесь же решается,
// какие источники доступны на этом железе (Caps): упавший при старте сенсор
// выключает свою группу, но не приложение.
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
		path = "" // настройки будут жить только в памяти
	}
	return config.Load(path)
}

// probe собирает доступные источники и Caps для UI.
func probe() (sensors.Sources, entity.Caps, error) {
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
			// в меню — только интерфейсы, у которых был трафик к моменту старта
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

	return src, caps, nil
}

// Run запускает menu bar приложение; блокирует до выхода.
func Run() error {
	store := loadStore()
	hw := sensors.ReadHWInfo()

	src, caps, err := probe()
	if err != nil {
		return err
	}
	mon := usecase.NewMonitor(src, store)

	tray.New(store, hw, caps).Run(func(ctx context.Context) <-chan entity.Snapshot {
		return mon.Start(ctx)
	})
	return nil
}

// RunOnce печатает один кадр метрик в stdout и выходит — для проверки
// сенсоров без UI (pulse -once).
func RunOnce() error {
	store := loadStore()
	cfg := store.Get()
	hw := sensors.ReadHWInfo()

	src, _, err := probe()
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
	fmt.Printf("CPU total: %s (за %v)\n", format.Percent(snap.CPU.Total), cfg.Interval())
	fmt.Printf("CPU cores: %s\n", strings.Join(perCore, " "))
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

	return nil
}
