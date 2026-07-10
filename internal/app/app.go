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

// Run запускает menu bar приложение; блокирует до выхода.
func Run(cfg config.Config) error {
	hw := sensors.ReadHWInfo()

	mem, err := sensors.NewMem()
	if err != nil {
		return fmt.Errorf("memory sensor: %w", err)
	}
	mon := usecase.NewMonitor(sensors.NewCPU(), mem, cfg.Interval)

	tray.New(cfg, hw).Run(func(ctx context.Context) <-chan entity.Snapshot {
		return mon.Start(ctx)
	})
	return nil
}

// RunOnce печатает один кадр метрик в stdout и выходит — для проверки
// сенсоров без UI (pulse -once).
func RunOnce(cfg config.Config) error {
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
	time.Sleep(cfg.Interval)
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

	fmt.Printf("%s · %s · macOS %s · %d cores · apple silicon: %v\n",
		hw.Chip, hw.Model, hw.OSVersion, hw.NumCores, hw.IsAppleSilicon)
	fmt.Printf("CPU total: %s (за %v)\n", format.Percent(st.Total), cfg.Interval)
	fmt.Printf("CPU cores: %s\n", strings.Join(perCore, " "))
	fmt.Printf("Mem: used %s / %s (%s), available %s, free %s\n",
		format.Bytes(ms.Used), format.Bytes(ms.Total), format.Percent(ms.UsedFraction()),
		format.Bytes(ms.Available), format.Bytes(ms.Free))
	fmt.Printf("Swap: used %s / %s\n", format.Bytes(ms.SwapUsed), format.Bytes(ms.SwapTotal))
	return nil
}
