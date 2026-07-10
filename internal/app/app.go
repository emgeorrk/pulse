//go:build darwin

// Package app связывает слои: sensors → usecase → tray.
package app

import (
	"context"
	"fmt"
	"strings"
	"time"

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

// Run запускает menu bar приложение; блокирует до выхода.
func Run() error {
	store := loadStore()
	hw := sensors.ReadHWInfo()

	mem, err := sensors.NewMem()
	if err != nil {
		return fmt.Errorf("memory sensor: %w", err)
	}
	mon := usecase.NewMonitor(sensors.NewCPU(), mem, store)

	tray.New(store, hw).Run(func(ctx context.Context) <-chan entity.Snapshot {
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

	cpu := sensors.NewCPU()
	mem, err := sensors.NewMem()
	if err != nil {
		return fmt.Errorf("memory sensor: %w", err)
	}

	prev, err := cpu.Ticks()
	if err != nil {
		return fmt.Errorf("cpu sensor: %w", err)
	}
	time.Sleep(cfg.Interval())
	cur, err := cpu.Ticks()
	if err != nil {
		return fmt.Errorf("cpu sensor: %w", err)
	}
	ms, err := mem.Read()
	if err != nil {
		return fmt.Errorf("memory sensor: %w", err)
	}

	st := usecase.CPUUsage(prev, cur)
	perCore := make([]string, len(st.Cores))
	for i, c := range st.Cores {
		perCore[i] = format.Percent(c)
	}

	dec := cfg.DecimalBytes
	fmt.Printf("%s · %s · macOS %s · %d cores · apple silicon: %v\n",
		hw.Chip, hw.Model, hw.OSVersion, hw.NumCores, hw.IsAppleSilicon)
	fmt.Printf("CPU total: %s (за %v)\n", format.Percent(st.Total), cfg.Interval())
	fmt.Printf("CPU cores: %s\n", strings.Join(perCore, " "))
	fmt.Printf("Mem: used %s / %s (%s), available %s, free %s\n",
		format.Bytes(ms.Used, dec), format.Bytes(ms.Total, dec), format.Percent(ms.UsedFraction()),
		format.Bytes(ms.Available, dec), format.Bytes(ms.Free, dec))
	fmt.Printf("Swap: used %s / %s\n", format.Bytes(ms.SwapUsed, dec), format.Bytes(ms.SwapTotal, dec))
	return nil
}
