//go:build darwin

package tray

import (
	"fyne.io/systray"
	"github.com/emgeorrk/pulse/config"
)

// stepper describes one +/-adjustable integer setting rendered as a
// value-titled submenu ("<prefix>: <formatted value>") with two keep-open
// leaves that raise or lower the value by step within [min, max].
type stepper struct {
	get    func(config.Config) int
	set    func(*config.Config, int)
	format func(int) string
	min    int
	max    int
	step   int
}

// next returns current+delta clamped to [min, max].
func (sp stepper) next(current, delta int) int {
	v := current + delta

	switch {
	case v < sp.min:
		return sp.min
	case v > sp.max:
		return sp.max
	default:
		return v
	}
}

// stepLabels renders the increment and decrement leaf titles ("+1 s", "−1 s").
func stepLabels(sp stepper) (inc, dec string) {
	return "+" + sp.format(sp.step), "−" + sp.format(sp.step)
}

func intervalStepper() stepper {
	return stepper{
		get:    func(c config.Config) int { return c.IntervalSec },
		set:    func(c *config.Config, v int) { c.IntervalSec = v },
		format: formatSeconds,
		min:    config.MinIntervalSeconds,
		max:    config.MaxIntervalSeconds,
		step:   config.IntervalStepSeconds,
	}
}

// stepperUI bundles the menu items a stepper click handler must update.
type stepperUI struct {
	head   *systray.MenuItem
	inc    *systray.MenuItem
	dec    *systray.MenuItem
	prefix string
	sp     stepper
}

// setEnabled greys out the +/- leaves at the bounds; next clamps anyway,
// so this is presentation only.
func (ui stepperUI) setEnabled(v int) {
	setItemEnabled(ui.inc, v < ui.sp.max)
	setItemEnabled(ui.dec, v > ui.sp.min)
}

func setItemEnabled(item *systray.MenuItem, on bool) {
	if on {
		item.Enable()
	} else {
		item.Disable()
	}
}

// addStepper builds one "<prefix>: <current>" submenu row under parent whose
// keep-open +/- leaves adjust the value in place: each click applies the
// config change, retitles the parent row, and re-evaluates the bounds.
// The parent itself must stay a plain submenu anchor — KeepMenuOpen is
// leaf-only, and the systray PATCH already makes clicking it action-free.
func (t *Tray) addStepper(parent *systray.MenuItem, prefix string, sp stepper, cfg config.Config) {
	v := sp.get(cfg)
	head := parent.AddSubMenuItem(settingTitle(prefix, sp.format(v)), "")
	incLabel, decLabel := stepLabels(sp)

	inc := head.AddSubMenuItem(incLabel, "")
	inc.KeepMenuOpen()

	dec := head.AddSubMenuItem(decLabel, "")
	dec.KeepMenuOpen()

	ui := stepperUI{head: head, inc: inc, dec: dec, sp: sp, prefix: prefix}
	ui.setEnabled(v)

	go t.watchStep(ui, inc, sp.step)
	go t.watchStep(ui, dec, -sp.step)
}

// watchStep reacts to clicks on one +/- leaf of a stepper submenu.
func (t *Tray) watchStep(ui stepperUI, item *systray.MenuItem, delta int) {
	for range item.ClickedCh {
		t.applyStep(ui, delta)
	}
}

func (t *Tray) applyStep(ui stepperUI, delta int) {
	var v int

	t.updateConfig(func(c *config.Config) {
		v = ui.sp.next(ui.sp.get(*c), delta)
		ui.sp.set(c, v)
	})

	ui.head.SetTitle(settingTitle(ui.prefix, ui.sp.format(v)))
	ui.setEnabled(v)
	t.refresh()
}
