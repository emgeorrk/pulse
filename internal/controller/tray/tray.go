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
	"github.com/emgeorrk/pulse/internal/controller/tray/icons"
	"github.com/emgeorrk/pulse/internal/entity"
	"github.com/emgeorrk/pulse/pkg/format"
)

type groupUI struct {
	group
	item *systray.MenuItem
	rows []*systray.MenuItem // parallel to group.metrics
}

type Tray struct { //nolint:govet // Field order follows UI lifecycle and state ownership.
	last         entity.Snapshot
	store        *config.Store
	bar          map[entity.MetricID]metric
	sys          *systray.MenuItem
	settings     *systray.MenuItem
	appliedStyle config.VisualStyle
	groups       []groupUI
	hw           entity.HWInfo
	mu           sync.Mutex
	seen         bool
	loading      bool
}

func New(store *config.Store, hw entity.HWInfo, caps entity.Caps) *Tray {
	t := &Tray{store: store, hw: hw, bar: map[entity.MetricID]metric{}, loading: true}

	groups := buildGroups(hw, caps)
	for groupIndex := range groups {
		g := &groups[groupIndex]
		for metricIndex := range g.metrics {
			g.metrics[metricIndex] = g.metrics[metricIndex].fill(*g)
		}

		t.groups = append(t.groups, groupUI{group: *g})
		for metricIndex := range g.metrics {
			m := &g.metrics[metricIndex]
			t.bar[m.id] = *m
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
	systray.SetTooltip("Pulse — system monitor")

	for _, key := range icons.Keys() {
		systray.RegisterTitleIcon(key, icons.PNG(key))
	}

	cfg := t.store.Get()

	for gi := range t.groups {
		g := &t.groups[gi]

		g.item = systray.AddMenuItem(g.headerTitle(cfg, ""), "")
		for metricIndex := range g.metrics {
			m := &g.metrics[metricIndex]
			it := g.item.AddSubMenuItemCheckbox(m.label+": —", "", cfg.IsPinned(m.id))

			it.KeepMenuOpen() // pinning several metrics in one menu open
			go t.watchPin(m.id, it)

			g.rows = append(g.rows, it)
		}
	}

	systray.AddSeparator()

	t.sys = systray.AddMenuItem(sysTitle(cfg), "")
	t.sys.AddSubMenuItem(t.hw.Chip, "")

	switch {
	case t.hw.ModelName != "":
		t.sys.AddSubMenuItem(prettyModel(t.hw.ModelName, t.hw.Chip), "")
	case t.hw.Model != "":
		t.sys.AddSubMenuItem("Model: "+t.hw.Model, "")
	}

	if t.hw.OSVersion != "" {
		t.sys.AddSubMenuItem("macOS "+t.hw.OSVersion, "")
	}

	t.buildSettings(cfg)

	t.applyVisualStyle(cfg) // after buildSettings so the Settings item exists

	systray.AddSeparator()

	quit := systray.AddMenuItem("Quit Pulse", "")
	go func() {
		<-quit.ClickedCh
		systray.Quit()
	}()
}

// headerTitle is the group header text for the current visual style; in the
// gnome style the emoji is replaced by a template icon on the item itself.
func (g *groupUI) headerTitle(cfg config.Config, aggregate string) string {
	title := g.label
	if cfg.VisualStyle != config.VisualGnome {
		title = g.emoji + " " + title
	}

	if aggregate != "" {
		title += " · " + aggregate
	}

	return title
}

func sysTitle(cfg config.Config) string {
	if cfg.VisualStyle == config.VisualGnome {
		return "System"
	}

	return "ℹ️ System"
}

func settingsTitle(cfg config.Config) string {
	if cfg.VisualStyle == config.VisualGnome {
		return "Settings"
	}

	return "🛠️ Settings"
}

// applyVisualStyle sets or clears the dropdown icons. Called from build and
// whenever the style setting changes — not on every frame, so the images
// aren't re-decoded each tick.
func (t *Tray) applyVisualStyle(cfg config.Config) {
	gnome := cfg.VisualStyle == config.VisualGnome

	for gi := range t.groups {
		g := &t.groups[gi]
		if gnome {
			setTemplateIcon(g.item, g.icon)
		} else {
			g.item.ClearIcon()
		}
	}

	if t.sys != nil {
		t.sys.SetTitle(sysTitle(cfg))

		if gnome {
			setTemplateIcon(t.sys, icons.System)
		} else {
			t.sys.ClearIcon()
		}
	}

	if t.settings != nil {
		t.settings.SetTitle(settingsTitle(cfg))

		if gnome {
			setTemplateIcon(t.settings, icons.Settings)
		} else {
			t.settings.ClearIcon()
		}
	}

	t.mu.Lock()
	t.appliedStyle = cfg.VisualStyle
	t.mu.Unlock()
}

// setTemplateIcon guards against missing assets: no icon beats a panic on
// empty bytes inside the systray fork.
func setTemplateIcon(item *systray.MenuItem, key string) {
	if png := icons.PNG(key); len(png) > 0 {
		item.SetTemplateIcon(png, nil)
	}
}

func (t *Tray) buildSettings(cfg config.Config) { //nolint:funlen,gocognit // Settings construction mirrors the menu hierarchy.
	s := systray.AddMenuItem(settingsTitle(cfg), "")
	t.settings = s

	// update interval — radio group
	intervals := []int{1, 2, 3, 5}

	var intervalItems []*systray.MenuItem

	for _, sec := range intervals {
		label := formatSeconds(sec)
		it := s.AddSubMenuItemCheckbox("Update: "+label, "", cfg.IntervalSec == sec)
		it.KeepMenuOpen()

		intervalItems = append(intervalItems, it)
		go func(me *systray.MenuItem) {
			for range me.ClickedCh {
				t.updateConfig(func(c *config.Config) { c.IntervalSec = sec })

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
	binU := s.AddSubMenuItemCheckbox("Memory: GiB", "", !cfg.DecimalBytes)
	decU := s.AddSubMenuItemCheckbox("Memory: GB", "", cfg.DecimalBytes)

	binU.KeepMenuOpen()

	decU.KeepMenuOpen()
	go t.watchRadio(binU, decU, func(c *config.Config) { c.DecimalBytes = false })
	go t.watchRadio(decU, binU, func(c *config.Config) { c.DecimalBytes = true })

	// visual style — radio group; render picks up the change and restyles
	// the dropdown via applyVisualStyle
	visEmoji := s.AddSubMenuItemCheckbox("Icons: Emoji", "", cfg.VisualStyle == config.VisualEmoji)
	visGnome := s.AddSubMenuItemCheckbox("Icons: GNOME", "", cfg.VisualStyle == config.VisualGnome)

	visEmoji.KeepMenuOpen()

	visGnome.KeepMenuOpen()
	go t.watchRadio(visEmoji, visGnome, func(c *config.Config) { c.VisualStyle = config.VisualEmoji })
	go t.watchRadio(visGnome, visEmoji, func(c *config.Config) { c.VisualStyle = config.VisualGnome })

	// bar label style — radio group
	barText := s.AddSubMenuItemCheckbox("Bar labels: Text", "", cfg.BarLabels == config.BarText)
	barVis := s.AddSubMenuItemCheckbox("Bar labels: Icons", "", cfg.BarLabels == config.BarVisual)

	barText.KeepMenuOpen()

	barVis.KeepMenuOpen()
	go t.watchRadio(barText, barVis, func(c *config.Config) { c.BarLabels = config.BarText })
	go t.watchRadio(barVis, barText, func(c *config.Config) { c.BarLabels = config.BarVisual })

	// CPU sparkline in the menu bar
	spark := s.AddSubMenuItemCheckbox("CPU sparkline in bar", "", cfg.ShowSparkline)

	spark.KeepMenuOpen()
	go func() {
		for range spark.ClickedCh {
			var on bool

			t.updateConfig(func(c *config.Config) { c.ShowSparkline = !c.ShowSparkline; on = c.ShowSparkline })
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

			t.updateConfig(func(c *config.Config) { c.StartAtLogin = !c.StartAtLogin; on = c.StartAtLogin })

			var err error
			if on {
				err = autostart.Enable()
			} else {
				err = autostart.Disable()
			}

			if err != nil { // rollback: couldn't write the LaunchAgent — don't lie with the checkbox
				t.updateConfig(func(c *config.Config) { c.StartAtLogin = !on })

				on = !on
			}

			setChecked(login, on)
		}
	}()
}

// watchRadio: clicking me checks it, unchecks other, and applies apply.
func (t *Tray) watchRadio(me, other *systray.MenuItem, apply func(*config.Config)) {
	for range me.ClickedCh {
		t.updateConfig(apply)
		me.Check()
		other.Uncheck()
		t.refresh()
	}
}

func (t *Tray) updateConfig(apply func(*config.Config)) {
	if err := t.store.Update(apply); err != nil {
		return // Settings remain updated in memory when persistence fails.
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
const (
	restFrame  = "♡"
	pulseFrame = "♥︎"
)

type heartbeatFrame struct {
	frame string
	d     time.Duration
}

func heartbeatFrames() []heartbeatFrame {
	return []heartbeatFrame{
		{pulseFrame, 180 * time.Millisecond}, // lub
		{restFrame, 120 * time.Millisecond},
		{pulseFrame, 180 * time.Millisecond}, // dub
		{restFrame, 620 * time.Millisecond},  // diastole
	}
}

// animate beats the heart in the title until the first snapshot arrives.
// It only checks for that at the end of a full cycle, so a beat is never
// cut short; render skips the title while loading is set.
func (t *Tray) animate(ctx context.Context) {
	for {
		for _, f := range heartbeatFrames() {
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

func (t *Tray) render(s entity.Snapshot) { //nolint:gocritic // Snapshots are immutable frames shared across render helpers.
	cfg := t.store.Get()
	t.mu.Lock()
	loading := t.loading
	styleStale := t.appliedStyle != cfg.VisualStyle
	t.mu.Unlock()

	if styleStale { // the style radio changed — restyle the dropdown once
		t.applyVisualStyle(cfg)
	}

	if !loading { // while loading, the heartbeat animation owns the title
		t.setTitle(s, cfg)
	}

	for gi := range t.groups {
		g := &t.groups[gi]
		if g.aggregate != nil {
			g.item.SetTitle(g.headerTitle(cfg, g.aggregate(s, cfg)))
		}

		for i := range g.metrics {
			m := &g.metrics[i]
			g.rows[i].SetTitle(m.label + ": " + m.menu(s, cfg))
		}
	}
}

// setTitle renders the pinned metrics into the status item. Only the
// gnome+visual combination needs the attributed-title path; everything
// else stays a plain string.
func (t *Tray) setTitle(s entity.Snapshot, cfg config.Config) { //nolint:cyclop,gocritic,gocyclo // Title modes are intentionally explicit.
	var parts []systray.TitlePart
	if cfg.ShowSparkline && len(s.CPU.History) > 0 {
		parts = append(parts, systray.TitlePart{Text: format.Sparkline(s.CPU.History)})
	}

	for _, id := range cfg.Pinned {
		if m, ok := t.bar[id]; ok {
			iconKey, text := m.barPart(s, cfg)
			parts = append(parts, systray.TitlePart{Icon: iconKey, Text: text})
		}
	}

	if len(parts) == 0 {
		systray.SetTitle("Pulse")

		return
	}

	if cfg.BarLabels == config.BarVisual && cfg.VisualStyle == config.VisualGnome {
		// icon parts already begin with a space; a single-space separator
		// keeps the gaps even with the text modes' double space
		sep := make([]systray.TitlePart, 0, len(parts)*2-1)
		for i, p := range parts {
			if i > 0 {
				sep = append(sep, systray.TitlePart{Text: " "})
			}

			sep = append(sep, p)
		}

		systray.SetTitleParts(sep)

		return
	}

	texts := make([]string, len(parts))
	for i, p := range parts {
		texts[i] = p.Text
	}

	systray.SetTitle(strings.Join(texts, "  "))
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
