//go:build darwin

// Package tray — menu bar UI на fyne.io/systray: выбранные метрики инлайн в
// title, полный список — в дропдауне по группам (как в Vitals).
package tray

import (
	"context"
	"fmt"
	"strings"

	"fyne.io/systray"

	"github.com/emgeorrk/pulse/config"
	"github.com/emgeorrk/pulse/internal/entity"
	"github.com/emgeorrk/pulse/pkg/format"
)

type Tray struct {
	cfg config.Config
	hw  entity.HWInfo

	cpuUsage *systray.MenuItem
	cores    []*systray.MenuItem
	memUsage *systray.MenuItem
	memUsed  *systray.MenuItem
	memAvail *systray.MenuItem
	memPhys  *systray.MenuItem
	memSwap  *systray.MenuItem
}

func New(cfg config.Config, hw entity.HWInfo) *Tray {
	return &Tray{cfg: cfg, hw: hw}
}

// Run блокирует до выхода из приложения. systray.Run обязан выполняться на
// главной горутине (Cocoa main thread); start вызывается уже из onReady.
func (t *Tray) Run(start func(ctx context.Context) <-chan entity.Snapshot) {
	ctx, cancel := context.WithCancel(context.Background())
	systray.Run(func() {
		t.build()
		go t.consume(start(ctx))
	}, cancel)
}

func (t *Tray) build() {
	systray.SetTitle("pulse …")
	systray.SetTooltip("pulse — system monitor")

	cpu := systray.AddMenuItem("Processor", "")
	t.cpuUsage = cpu.AddSubMenuItem("Usage: —", "")
	for i := 0; i < t.hw.NumCores; i++ {
		t.cores = append(t.cores, cpu.AddSubMenuItem(fmt.Sprintf("Core %d: —", i+1), ""))
	}

	mem := systray.AddMenuItem("Memory", "")
	t.memUsage = mem.AddSubMenuItem("Usage: —", "")
	t.memUsed = mem.AddSubMenuItem("Used: —", "")
	t.memAvail = mem.AddSubMenuItem("Available: —", "")
	t.memPhys = mem.AddSubMenuItem("Physical: —", "")
	t.memSwap = mem.AddSubMenuItem("Swap: —", "")

	systray.AddSeparator()
	sys := systray.AddMenuItem("System", "")
	sys.AddSubMenuItem(t.hw.Chip, "")
	if t.hw.Model != "" {
		sys.AddSubMenuItem("Model: "+t.hw.Model, "")
	}
	if t.hw.OSVersion != "" {
		sys.AddSubMenuItem("macOS "+t.hw.OSVersion, "")
	}

	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit pulse", "")
	go func() {
		<-quit.ClickedCh
		systray.Quit()
	}()
}

func (t *Tray) consume(ch <-chan entity.Snapshot) {
	for snap := range ch {
		t.update(snap)
	}
}

func (t *Tray) update(s entity.Snapshot) {
	systray.SetTitle(t.title(s))

	t.cpuUsage.SetTitle("Usage: " + format.Percent(s.CPU.Total))
	for i, item := range t.cores {
		if i < len(s.CPU.Cores) {
			item.SetTitle(fmt.Sprintf("Core %d: %s", i+1, format.Percent(s.CPU.Cores[i])))
		}
	}

	t.memUsage.SetTitle("Usage: " + format.Percent(s.Mem.UsedFraction()))
	t.memUsed.SetTitle("Used: " + format.Bytes(s.Mem.Used))
	t.memAvail.SetTitle("Available: " + format.Bytes(s.Mem.Available))
	t.memPhys.SetTitle("Physical: " + format.Bytes(s.Mem.Total))
	t.memSwap.SetTitle("Swap: " + format.Bytes(s.Mem.SwapUsed))
}

func (t *Tray) title(s entity.Snapshot) string {
	var parts []string
	if t.cfg.ShowCPUInBar {
		parts = append(parts, "CPU "+format.Percent(s.CPU.Total))
	}
	if t.cfg.ShowMemInBar {
		parts = append(parts, format.BytesShort(s.Mem.Used))
	}
	if len(parts) == 0 {
		return "pulse"
	}
	return strings.Join(parts, "  ")
}
