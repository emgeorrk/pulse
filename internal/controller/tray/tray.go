//go:build darwin

// Package tray is the menu bar UI built on fyne.io/systray, modeled on
// Vitals: pinned metrics show inline in the title, the full list is in the
// dropdown grouped by category, pinning is a checkbox click, and settings
// live in the Settings submenu.
package tray

import (
	"context"
	"fmt"
	"os/exec"
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
	actMon       *systray.MenuItem
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

	for _, st := range icons.ImageStyles() {
		for _, key := range icons.MetricKeys() {
			systray.RegisterTitleIcon(icons.TitleKey(st, key), icons.PNG(st, key))
		}
	}

	cfg := t.store.Get()

	for gi := range t.groups {
		g := &t.groups[gi]

		g.item = systray.AddMenuItem(g.headerTitle(""), "")
		for metricIndex := range g.metrics {
			m := &g.metrics[metricIndex]
			it := g.item.AddSubMenuItemCheckbox(m.label+": —", "", cfg.IsPinned(m.id))

			it.KeepMenuOpen() // pinning several metrics in one menu open
			go t.watchPin(m.id, it)

			g.rows = append(g.rows, it)
		}
	}

	systray.AddSeparator()

	t.sys = systray.AddMenuItem("About", "")
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

	// Vitals' "System Monitor" button; the macOS equivalent is Activity Monitor.
	t.actMon = systray.AddMenuItem("Activity Monitor", "")
	go func() {
		for range t.actMon.ClickedCh {
			openActivityMonitor()
		}
	}()

	t.buildSettings(cfg)

	t.applyVisualStyle(cfg) // after buildSettings so the Settings item exists

	systray.AddSeparator()

	quit := systray.AddMenuItem("Quit Pulse", "")
	go func() {
		<-quit.ClickedCh
		systray.Quit()
	}()
}

// headerTitle is the group header text plus the live aggregate; the group's
// visual (emoji or template icon) lives in the item's icon slot, never in
// the text.
func (g *groupUI) headerTitle(aggregate string) string {
	title := g.label
	if aggregate != "" {
		title += " · " + aggregate
	}

	return title
}

// Emoji for the About, Activity Monitor and Settings items in the emoji
// style; the groups carry their own emoji in the registry.
const (
	sysEmoji      = "ℹ️"
	actMonEmoji   = "📈"
	settingsEmoji = "🛠️"
)

// openActivityMonitor launches Activity Monitor via open(1). A failure is
// deliberately ignored: there is nowhere to surface it from the menu, and
// the app itself is unaffected.
func openActivityMonitor() {
	_ = exec.CommandContext(context.Background(), "open", "-a", "Activity Monitor").Start() //nolint:errcheck // Best-effort launch, see above.
}

// applyVisualStyle swaps the dropdown icons between the emoji and the icon
// packs (gnome, classic). Every style puts an image on the item, so a live
// switch only replaces image contents — an open menu never gains or loses its
// icon column, which AppKit fails to re-lay out (rows keep a stale indent).
// Called from build and on a style change — not on every frame, so the images
// aren't re-decoded each tick.
func (t *Tray) applyVisualStyle(cfg config.Config) {
	style := cfg.VisualStyle

	for gi := range t.groups {
		g := &t.groups[gi]
		applyItemStyle(g.item, style, g.icon, g.emoji)
	}

	applyItemStyle(t.sys, style, icons.About, sysEmoji)
	applyItemStyle(t.actMon, style, icons.ActivityMonitor, actMonEmoji)
	applyItemStyle(t.settings, style, icons.Settings, settingsEmoji)

	t.mu.Lock()
	t.appliedStyle = style
	t.mu.Unlock()
}

// applyItemStyle puts the style's visual on one menu item: the template icon
// for the icon packs, the emoji otherwise. Not-yet-built items are skipped.
func applyItemStyle(item *systray.MenuItem, style config.VisualStyle, key, emoji string) {
	if item == nil {
		return
	}

	if style.UsesTemplateIcons() {
		setTemplateIcon(item, style, key)
	} else {
		item.SetEmojiIcon(emoji)
	}
}

// setTemplateIcon guards against missing assets: no icon beats a panic on
// empty bytes inside the systray fork.
func setTemplateIcon(item *systray.MenuItem, style config.VisualStyle, key string) {
	if png := icons.PNG(string(style), key); len(png) > 0 {
		item.SetTemplateIcon(png, nil)
	}
}

func (t *Tray) buildSettings(cfg config.Config) { //nolint:funlen,gocognit // Settings construction mirrors the menu hierarchy.
	s := systray.AddMenuItem("Settings", "")
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

	// visual style — radio group; render picks up the change and swaps the
	// dropdown icons via applyVisualStyle
	visEmoji := s.AddSubMenuItemCheckbox("Icons: Emoji", "", cfg.VisualStyle == config.VisualEmoji)
	visGnome := s.AddSubMenuItemCheckbox("Icons: GNOME", "", cfg.VisualStyle == config.VisualGnome)
	visClassic := s.AddSubMenuItemCheckbox("Icons: Classic", "", cfg.VisualStyle == config.VisualClassic)

	visEmoji.KeepMenuOpen()

	visGnome.KeepMenuOpen()

	visClassic.KeepMenuOpen()
	go t.watchRadioN(visEmoji, []*systray.MenuItem{visGnome, visClassic}, func(c *config.Config) { c.VisualStyle = config.VisualEmoji })
	go t.watchRadioN(visGnome, []*systray.MenuItem{visEmoji, visClassic}, func(c *config.Config) { c.VisualStyle = config.VisualGnome })
	go t.watchRadioN(visClassic, []*systray.MenuItem{visEmoji, visGnome}, func(c *config.Config) { c.VisualStyle = config.VisualClassic })

	// bar label style — radio group
	barText := s.AddSubMenuItemCheckbox("Bar labels: Text", "", cfg.BarLabels == config.BarText)
	barVis := s.AddSubMenuItemCheckbox("Bar labels: Icons", "", cfg.BarLabels == config.BarVisual)

	barText.KeepMenuOpen()

	barVis.KeepMenuOpen()
	go t.watchRadio(barText, barVis, func(c *config.Config) { c.BarLabels = config.BarText })
	go t.watchRadio(barVis, barText, func(c *config.Config) { c.BarLabels = config.BarVisual })

	// public IP lookup (an outbound HTTPS request every 15 min) — opt-in
	pubIP := s.AddSubMenuItemCheckbox("Show public IP", "", cfg.ShowPublicIP)

	pubIP.KeepMenuOpen()
	go func() {
		for range pubIP.ClickedCh {
			var on bool

			t.updateConfig(func(c *config.Config) { c.ShowPublicIP = !c.ShowPublicIP; on = c.ShowPublicIP })
			setChecked(pubIP, on)
			t.refresh()
		}
	}()

	// one extra fraction digit on values (Vitals' "use higher precision")
	precise := s.AddSubMenuItemCheckbox("Higher precision", "", cfg.HigherPrecision)

	precise.KeepMenuOpen()
	go func() {
		for range precise.ClickedCh {
			var on bool

			t.updateConfig(func(c *config.Config) { c.HigherPrecision = !c.HigherPrecision; on = c.HigherPrecision })
			setChecked(precise, on)
			t.refresh()
		}
	}()

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

// watchRadioN is watchRadio for a group of more than two options: clicking me
// checks it, unchecks every other item, and applies apply.
func (t *Tray) watchRadioN(me *systray.MenuItem, others []*systray.MenuItem, apply func(*config.Config)) {
	for range me.ClickedCh {
		t.updateConfig(apply)
		me.Check()

		for _, other := range others {
			other.Uncheck()
		}

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

// animate beats the heart in the title until the first snapshot arrives and
// at least minBeats full cycles have played. It only checks at the end of a
// full cycle, so a beat is never cut short; render skips the title while
// loading is set.
func (t *Tray) animate(ctx context.Context) {
	const minBeats = 3

	for beats := 1; ; beats++ {
		for _, f := range heartbeatFrames() {
			systray.SetTitle(f.frame)

			select {
			case <-ctx.Done():
				return
			case <-time.After(f.d):
			}
		}

		t.mu.Lock()
		done := t.seen && beats >= minBeats
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

	if styleStale { // the style radio changed — swap the dropdown icons once
		t.applyVisualStyle(cfg)
	}

	if !loading { // while loading, the heartbeat animation owns the title
		t.setTitle(s, cfg)
	}

	for gi := range t.groups {
		g := &t.groups[gi]
		if g.aggregate != nil {
			g.item.SetTitle(g.headerTitle(g.aggregate(s, cfg)))
		}

		for i := range g.metrics {
			m := &g.metrics[i]
			g.rows[i].SetTitle(m.label + ": " + m.menu(s, cfg))
		}
	}
}

// setTitle renders the pinned metrics into the status item. Only an icon
// pack in visual mode needs the attributed-title path; everything else
// (text mode, emoji) stays a plain string.
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

	if cfg.BarLabels == config.BarVisual && cfg.VisualStyle.UsesTemplateIcons() {
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
