//go:build darwin

// Package tray is the menu bar UI built on fyne.io/systray, modeled on
// Vitals: pinned metrics show inline in the title, the full list is in the
// dropdown grouped by category, pinning is a checkbox click, and settings
// live in the Settings submenu.
package tray

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"fyne.io/systray"

	"github.com/emgeorrk/pulse/config"
	"github.com/emgeorrk/pulse/internal/autostart"
	"github.com/emgeorrk/pulse/internal/entity"
	"github.com/emgeorrk/pulse/pkg/format"
)

type groupUI struct {
	group
	item *systray.MenuItem
	rows []*systray.MenuItem // parallel to group.metrics
}

type Tray struct {
	store *config.Store
	hw    entity.HWInfo

	groups []groupUI
	bar    map[entity.MetricID]metric // metrics by id, for rendering in the menu bar

	mu      sync.Mutex
	last    entity.Snapshot
	seen    bool // whether at least one frame has been received
	loading bool // startup heartbeat animation owns the title
}

func New(store *config.Store, hw entity.HWInfo, caps entity.Caps) *Tray {
	t := &Tray{store: store, hw: hw, bar: map[entity.MetricID]metric{}, loading: true}
	for _, g := range buildGroups(hw, caps) {
		t.groups = append(t.groups, groupUI{group: g})
		for _, m := range g.metrics {
			t.bar[m.id] = m
		}
	}
	return t
}

// Run blocks until the app quits. systray.Run must run on the main
// goroutine (the Cocoa main thread); start is called from onReady.
func (t *Tray) Run(start func(ctx context.Context) <-chan entity.Snapshot) {
	ctx, cancel := context.WithCancel(context.Background())
	systray.Run(func() {
		t.build()
		go t.animate(ctx)
		go t.consume(start(ctx))
	}, cancel)
}

func (t *Tray) build() {
	systray.SetTitle(restFrame)
	systray.SetTooltip("pulse — system monitor")

	cfg := t.store.Get()

	for gi := range t.groups {
		g := &t.groups[gi]
		g.item = systray.AddMenuItem(g.emoji+" "+g.label, "")
		for _, m := range g.metrics {
			it := g.item.AddSubMenuItemCheckbox(m.label+": —", "", cfg.IsPinned(m.id))
			it.KeepMenuOpen() // pinning several metrics in one menu open
			go t.watchPin(m.id, it)
			g.rows = append(g.rows, it)
		}
	}

	systray.AddSeparator()
	sys := systray.AddMenuItem("ℹ️ System", "")
	sys.AddSubMenuItem(t.hw.Chip, "")
	switch {
	case t.hw.ModelName != "":
		sys.AddSubMenuItem(prettyModel(t.hw.ModelName, t.hw.Chip), "")
	case t.hw.Model != "":
		sys.AddSubMenuItem("Model: "+t.hw.Model, "")
	}
	if t.hw.OSVersion != "" {
		sys.AddSubMenuItem("macOS "+t.hw.OSVersion, "")
	}

	t.buildSettings(cfg)

	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit pulse", "")
	go func() {
		<-quit.ClickedCh
		systray.Quit()
	}()
}

func (t *Tray) buildSettings(cfg config.Config) {
	s := systray.AddMenuItem("🛠️ Settings", "")

	// update interval — radio group
	intervals := []int{1, 2, 3, 5}
	var intervalItems []*systray.MenuItem
	for _, sec := range intervals {
		sec := sec
		label := formatSeconds(sec)
		it := s.AddSubMenuItemCheckbox("Update: "+label, "", cfg.IntervalSec == sec)
		it.KeepMenuOpen()
		intervalItems = append(intervalItems, it)
		go func(me *systray.MenuItem) {
			for range me.ClickedCh {
				t.store.Update(func(c *config.Config) { c.IntervalSec = sec })
				for _, other := range intervalItems {
					other.Uncheck()
				}
				me.Check()
			}
		}(it)
	}

	// temperature unit — radio group (relevant since the temps feature)
	tempC := s.AddSubMenuItemCheckbox("Temperature: °C", "", cfg.TempUnit == config.Celsius)
	tempF := s.AddSubMenuItemCheckbox("Temperature: °F", "", cfg.TempUnit == config.Fahrenheit)
	tempC.KeepMenuOpen()
	tempF.KeepMenuOpen()
	go t.watchRadio(tempC, tempF, func(c *config.Config) { c.TempUnit = config.Celsius })
	go t.watchRadio(tempF, tempC, func(c *config.Config) { c.TempUnit = config.Fahrenheit })

	// memory unit — radio group
	binU := s.AddSubMenuItemCheckbox("Memory: GiB (binary)", "", !cfg.DecimalBytes)
	decU := s.AddSubMenuItemCheckbox("Memory: GB (decimal)", "", cfg.DecimalBytes)
	binU.KeepMenuOpen()
	decU.KeepMenuOpen()
	go t.watchRadio(binU, decU, func(c *config.Config) { c.DecimalBytes = false })
	go t.watchRadio(decU, binU, func(c *config.Config) { c.DecimalBytes = true })

	// CPU sparkline in the menu bar
	spark := s.AddSubMenuItemCheckbox("CPU sparkline in bar", "", cfg.ShowSparkline)
	spark.KeepMenuOpen()
	go func() {
		for range spark.ClickedCh {
			var on bool
			t.store.Update(func(c *config.Config) { c.ShowSparkline = !c.ShowSparkline; on = c.ShowSparkline })
			setChecked(spark, on)
			t.refresh()
		}
	}()

	// start at login
	login := s.AddSubMenuItemCheckbox("Start at login", "", cfg.StartAtLogin)
	login.KeepMenuOpen()
	go func() {
		for range login.ClickedCh {
			var on bool
			t.store.Update(func(c *config.Config) { c.StartAtLogin = !c.StartAtLogin; on = c.StartAtLogin })
			var err error
			if on {
				err = autostart.Enable()
			} else {
				err = autostart.Disable()
			}
			if err != nil { // rollback: couldn't write the LaunchAgent — don't lie with the checkbox
				t.store.Update(func(c *config.Config) { c.StartAtLogin = !on })
				on = !on
			}
			setChecked(login, on)
		}
	}()
}

// watchRadio: clicking me checks it, unchecks other, and applies apply.
func (t *Tray) watchRadio(me, other *systray.MenuItem, apply func(*config.Config)) {
	for range me.ClickedCh {
		t.store.Update(apply)
		me.Check()
		other.Uncheck()
		t.refresh()
	}
}

func (t *Tray) watchPin(id entity.MetricID, item *systray.MenuItem) {
	for range item.ClickedCh {
		setChecked(item, t.store.TogglePin(id))
		t.refresh()
	}
}

// The startup heartbeat: a "lub-dub" cycle of monochrome text glyphs
// (VS15 on the filled heart keeps it from rendering as color emoji).
// restFrame is also the initial title so the bar is never blank.
const restFrame = "♡"

var heartbeat = []struct {
	frame string
	d     time.Duration
}{
	{"♥︎", 180 * time.Millisecond}, // lub
	{restFrame, 120 * time.Millisecond},
	{"♥︎", 180 * time.Millisecond},      // dub
	{restFrame, 620 * time.Millisecond}, // diastole
}

// animate beats the heart in the title until the first snapshot arrives.
// It only checks for that at the end of a full cycle, so a beat is never
// cut short; render skips the title while loading is set.
func (t *Tray) animate(ctx context.Context) {
	for {
		for _, f := range heartbeat {
			systray.SetTitle(f.frame)
			select {
			case <-ctx.Done():
				return
			case <-time.After(f.d):
			}
		}
		t.mu.Lock()
		done := t.seen
		t.loading = !done
		t.mu.Unlock()
		if done {
			t.refresh()
			return
		}
	}
}

func (t *Tray) consume(ch <-chan entity.Snapshot) {
	for snap := range ch {
		t.mu.Lock()
		t.last = snap
		t.seen = true
		t.mu.Unlock()
		t.render(snap)
	}
}

// refresh redraws the UI from the last frame — for an instant reaction to
// pinning/settings changes, without waiting for the next tick.
func (t *Tray) refresh() {
	t.mu.Lock()
	snap, ok := t.last, t.seen
	t.mu.Unlock()
	if ok {
		t.render(snap)
	}
}

func (t *Tray) render(s entity.Snapshot) {
	cfg := t.store.Get()
	t.mu.Lock()
	loading := t.loading
	t.mu.Unlock()
	if !loading { // while loading, the heartbeat animation owns the title
		systray.SetTitle(t.title(s, cfg))
	}

	for gi := range t.groups {
		g := &t.groups[gi]
		if g.aggregate != nil {
			g.item.SetTitle(g.emoji + " " + g.label + " · " + g.aggregate(s, cfg))
		}
		for i, m := range g.metrics {
			g.rows[i].SetTitle(m.label + ": " + m.menu(s, cfg))
		}
	}
}

func (t *Tray) title(s entity.Snapshot, cfg config.Config) string {
	var parts []string
	if cfg.ShowSparkline && len(s.CPU.History) > 0 {
		parts = append(parts, format.Sparkline(s.CPU.History))
	}
	for _, id := range cfg.Pinned {
		if m, ok := t.bar[id]; ok {
			parts = append(parts, m.barText(s, cfg))
		}
	}
	if len(parts) == 0 {
		return "pulse"
	}
	return strings.Join(parts, "  ")
}

func setChecked(item *systray.MenuItem, on bool) {
	if on {
		item.Check()
	} else {
		item.Uncheck()
	}
}

func formatSeconds(sec int) string {
	return fmt.Sprintf("%d s", sec)
}

// prettyModel strips the parentheses and the duplicate chip name from
// product-name — it's already shown on its own line: "MacBook Pro
// (16-inch, M5 Pro)" → "MacBook Pro 16-inch".
func prettyModel(name, chip string) string {
	base, rest, ok := strings.Cut(name, "(")
	if !ok {
		return name
	}
	parts := []string{strings.TrimSpace(base)}
	rest = strings.TrimSuffix(strings.TrimSpace(rest), ")")
	for _, tok := range strings.Split(rest, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" || strings.Contains(chip, tok) {
			continue
		}
		parts = append(parts, tok)
	}
	return strings.Join(parts, " ")
}
