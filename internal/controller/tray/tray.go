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
	titleFlags   map[string]bool // flag title icons already registered
	sys          *systray.MenuItem
	actMon       *systray.MenuItem
	settings     *systray.MenuItem
	ipRow        *systray.MenuItem
	appliedStyle config.VisualStyle
	ipFlagCC     string // country of the flag on ipRow ("" = none), lowercase
	groups       []groupUI
	hw           entity.HWInfo
	mu           sync.Mutex
	seen         bool
	loading      bool
}

func New(store *config.Store, hw entity.HWInfo, caps entity.Caps) *Tray {
	t := &Tray{store: store, hw: hw, bar: map[entity.MetricID]metric{}, titleFlags: map[string]bool{}, loading: true}

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

	systray.SetTitleFixedWidth(cfg.FixedWidth)

	for gi := range t.groups {
		g := &t.groups[gi]

		g.item = systray.AddMenuItem(g.headerTitle(""), "")
		for metricIndex := range g.metrics {
			m := &g.metrics[metricIndex]
			it := g.item.AddSubMenuItemCheckbox(m.label+": —", rowTip(m, cfg), cfg.IsPinned(m.id))

			it.KeepMenuOpen() // pinning several metrics in one menu open
			go t.watchPin(m.id, it)

			if m.id == metricNetworkIP {
				t.ipRow = it // carries the country flag in the icon-pack styles
			}

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

// The Settings dropdown: every multi-choice setting is a submenu whose
// parent row carries the current value ("Temperature: °C"), the boolean
// toggles stay flat, and separators split the choice / toggle / login blocks.
func (t *Tray) buildSettings(cfg config.Config) {
	s := systray.AddMenuItem("Settings", "")
	t.settings = s

	t.addRadioGroup(s, "Update interval", intervalOptions(cfg))
	t.addRadioGroup(s, "Temperature", tempOptions(cfg))
	t.addRadioGroup(s, "Memory units", memoryOptions(cfg))
	t.addRadioGroup(s, iconsLabel, iconOptions(cfg))
	t.addRadioGroup(s, "Bar labels", barLabelOptions(cfg))

	s.AddSeparator()

	// public IP is opt-in: it costs an outbound HTTPS request every 15 min
	t.addToggle(s, "Show public IP", cfg.ShowPublicIP, func(c *config.Config) *bool { return &c.ShowPublicIP })
	t.addToggle(s, "Higher precision", cfg.HigherPrecision, func(c *config.Config) *bool { return &c.HigherPrecision })
	t.addToggle(s, "CPU sparkline in bar", cfg.ShowSparkline, func(c *config.Config) *bool { return &c.ShowSparkline })
	t.addToggle(s, "Fixed width", cfg.FixedWidth, func(c *config.Config) *bool { return &c.FixedWidth })

	tips := s.AddSubMenuItemCheckbox("Hide metric tooltips", "", cfg.HideTips)

	tips.KeepMenuOpen()
	go t.watchTips(tips)

	s.AddSeparator()

	login := s.AddSubMenuItemCheckbox("Start at login", "", cfg.StartAtLogin)

	login.KeepMenuOpen()
	go t.watchLogin(login)
}

// radioOption is one selectable leaf of a settings radio submenu.
type radioOption struct {
	apply   func(*config.Config)
	label   string
	checked bool
}

// labelSep joins a menu label with its value: "Temperature: °C".
const labelSep = ": "

// iconsLabel doubles as the visual-style group title and a Bar labels option.
const iconsLabel = "Icons"

func settingTitle(prefix, value string) string {
	return prefix + labelSep + value
}

// addRadioGroup builds one "<prefix>: <current>" submenu row under parent.
// The leaves are keep-open checkboxes forming a radio group: selecting one
// applies its config change, moves the check, and retitles the parent row.
// The parent itself must stay a plain submenu anchor — KeepMenuOpen is
// leaf-only, and the systray PATCH already makes clicking it action-free.
func (t *Tray) addRadioGroup(parent *systray.MenuItem, prefix string, opts []radioOption) {
	current := opts[0].label

	for _, o := range opts {
		if o.checked {
			current = o.label
		}
	}

	head := parent.AddSubMenuItem(settingTitle(prefix, current), "")

	items := make([]*systray.MenuItem, len(opts))
	for i, o := range opts {
		it := head.AddSubMenuItemCheckbox(o.label, "", o.checked)
		it.KeepMenuOpen()

		items[i] = it
	}

	for i, o := range opts { // watchers only after items is fully populated
		go t.watchRadioChoice(head, prefix, items, i, o)
	}
}

// watchRadioChoice reacts to clicks on option i of a radio submenu.
func (t *Tray) watchRadioChoice(head *systray.MenuItem, prefix string, items []*systray.MenuItem, i int, opt radioOption) {
	for range items[i].ClickedCh {
		t.updateConfig(opt.apply)

		for j, it := range items {
			setChecked(it, j == i)
		}

		head.SetTitle(settingTitle(prefix, opt.label))
		t.refresh()
	}
}

func intervalOptions(cfg config.Config) []radioOption {
	intervals := []int{1, 2, 3, 5}
	opts := make([]radioOption, 0, len(intervals))

	for _, sec := range intervals {
		opts = append(opts, radioOption{
			label:   formatSeconds(sec),
			checked: cfg.IntervalSec == sec,
			apply:   func(c *config.Config) { c.IntervalSec = sec },
		})
	}

	return opts
}

func tempOptions(cfg config.Config) []radioOption {
	return []radioOption{
		{label: "°C", checked: cfg.TempUnit == config.Celsius, apply: func(c *config.Config) { c.TempUnit = config.Celsius }},
		{label: "°F", checked: cfg.TempUnit == config.Fahrenheit, apply: func(c *config.Config) { c.TempUnit = config.Fahrenheit }},
	}
}

func memoryOptions(cfg config.Config) []radioOption {
	return []radioOption{
		{label: "GiB", checked: !cfg.DecimalBytes, apply: func(c *config.Config) { c.DecimalBytes = false }},
		{label: "GB", checked: cfg.DecimalBytes, apply: func(c *config.Config) { c.DecimalBytes = true }},
	}
}

// iconOptions: render picks the change up via refresh and swaps the dropdown
// icons once through applyVisualStyle.
func iconOptions(cfg config.Config) []radioOption {
	return []radioOption{
		{label: "Emoji", checked: cfg.VisualStyle == config.VisualEmoji, apply: func(c *config.Config) { c.VisualStyle = config.VisualEmoji }},
		{label: "GNOME", checked: cfg.VisualStyle == config.VisualGnome, apply: func(c *config.Config) { c.VisualStyle = config.VisualGnome }},
		{label: "Classic", checked: cfg.VisualStyle == config.VisualClassic, apply: func(c *config.Config) { c.VisualStyle = config.VisualClassic }},
	}
}

func barLabelOptions(cfg config.Config) []radioOption {
	return []radioOption{
		{label: "Text", checked: cfg.BarLabels == config.BarText, apply: func(c *config.Config) { c.BarLabels = config.BarText }},
		{label: iconsLabel, checked: cfg.BarLabels == config.BarVisual, apply: func(c *config.Config) { c.BarLabels = config.BarVisual }},
	}
}

// addToggle builds one flat keep-open checkbox row; field selects the bool
// to flip on click.
func (t *Tray) addToggle(parent *systray.MenuItem, title string, checked bool, field func(*config.Config) *bool) {
	it := parent.AddSubMenuItemCheckbox(title, "", checked)
	it.KeepMenuOpen()

	go func() {
		for range it.ClickedCh {
			var on bool

			t.updateConfig(func(c *config.Config) {
				p := field(c)
				*p = !*p
				on = *p
			})
			setChecked(it, on)
			t.refresh()
		}
	}()
}

// rowTip is the tooltip a metric row is created with — empty when the
// HideTips setting is on.
func rowTip(m *metric, cfg config.Config) string {
	if cfg.HideTips {
		return ""
	}

	return m.tip
}

// watchTips flips HideTips and applies it to every metric row in place.
func (t *Tray) watchTips(item *systray.MenuItem) {
	for range item.ClickedCh {
		var hide bool

		t.updateConfig(func(c *config.Config) { c.HideTips = !c.HideTips; hide = c.HideTips })
		setChecked(item, hide)
		t.applyTips(hide)
	}
}

// applyTips clears or restores the metric tooltips per the HideTips setting.
func (t *Tray) applyTips(hide bool) {
	for gi := range t.groups {
		g := &t.groups[gi]
		for i, it := range g.rows {
			tip := ""
			if !hide {
				tip = g.metrics[i].tip
			}

			it.SetTooltip(tip)
		}
	}
}

// watchLogin flips StartAtLogin and writes/removes the LaunchAgent. A write
// failure rolls the setting back so the checkbox never lies.
func (t *Tray) watchLogin(login *systray.MenuItem) {
	for range login.ClickedCh {
		var on bool

		t.updateConfig(func(c *config.Config) { c.StartAtLogin = !c.StartAtLogin; on = c.StartAtLogin })

		if err := applyAutostart(on); err != nil {
			t.updateConfig(func(c *config.Config) { c.StartAtLogin = !on })

			on = !on
		}

		setChecked(login, on)
	}
}

// applyAutostart enables or disables the LaunchAgent.
func applyAutostart(on bool) error {
	if on {
		return autostart.Enable()
	}

	return autostart.Disable()
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

	// Each call resets the tracked max width, so the title re-fits after a
	// config change (an unpin shrinks the item once instead of never).
	systray.SetTitleFixedWidth(t.store.Get().FixedWidth)

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

	t.updateIPFlag(s, cfg)

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
			g.rows[i].SetTitle(m.label + labelSep + m.menu(s, cfg))
		}
	}
}

// updateIPFlag mirrors the country flag onto the Public IP row's image in
// the icon-pack styles (the emoji style keeps the flag in the row text).
// The last applied code is cached so ticks don't re-decode the image; the
// flag set is style-independent, so a gnome↔classic switch is a no-op.
func (t *Tray) updateIPFlag(s entity.Snapshot, cfg config.Config) { //nolint:gocritic // Snapshot values are immutable render inputs.
	if t.ipRow == nil {
		return
	}

	var cc string
	if cfg.VisualStyle.UsesTemplateIcons() && s.Net != nil && icons.FlagPNG(s.Net.IPCountry) != nil {
		cc = strings.ToLower(s.Net.IPCountry)
	}

	t.mu.Lock()
	changed := t.ipFlagCC != cc
	t.ipFlagCC = cc
	t.mu.Unlock()

	if !changed {
		return
	}

	if cc == "" {
		t.ipRow.ClearIcon()

		return
	}

	t.ipRow.SetIcon(icons.FlagPNG(cc))
}

// ensureTitleIcon lazily registers a country flag as a color title icon the
// first time it is referenced — at most a handful of countries per session,
// versus decoding all 257 flags up front. The style packs are registered in
// build. Registration and the SetTitleParts that references it both run on
// the main thread (wait:YES), so the order holds from any goroutine.
func (t *Tray) ensureTitleIcon(key string) {
	if !strings.HasPrefix(key, icons.FlagKeyPrefix) {
		return
	}

	t.mu.Lock()
	seen := t.titleFlags[key]
	t.titleFlags[key] = true
	t.mu.Unlock()

	if seen {
		return
	}

	systray.RegisterColorTitleIcon(key, icons.FlagPNG(strings.TrimPrefix(key, icons.FlagKeyPrefix)))
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
			t.ensureTitleIcon(iconKey)
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
