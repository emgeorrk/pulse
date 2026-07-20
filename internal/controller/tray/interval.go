//go:build darwin

package tray

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/systray"
	"github.com/emgeorrk/pulse/config"
)

const intervalPrefix = "Update interval"

// intervalTitle renders the flat Settings row: "Update interval: 2 s…".
// The trailing ellipsis is the macOS cue that clicking opens a dialog.
func intervalTitle(sec int) string {
	return settingTitle(intervalPrefix, formatSeconds(sec)) + "…"
}

// parseInterval parses the typed input: non-numeric or empty is rejected;
// numeric input is clamped into [MinIntervalSeconds, MaxIntervalSeconds].
func parseInterval(s string) (int, bool) {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, false
	}

	switch {
	case v < config.MinIntervalSeconds:
		return config.MinIntervalSeconds, true
	case v > config.MaxIntervalSeconds:
		return config.MaxIntervalSeconds, true
	default:
		return v, true
	}
}

// addIntervalPrompt builds the "Update interval: N s…" row under parent.
// It is a flat leaf without KeepMenuOpen: the menu must close so the modal
// input dialog can take over the main thread.
func (t *Tray) addIntervalPrompt(parent *systray.MenuItem, cfg config.Config) {
	item := parent.AddSubMenuItem(intervalTitle(cfg.IntervalSec), "")

	go t.watchIntervalPrompt(item)
}

// watchIntervalPrompt reacts to clicks on the interval row. No re-entrancy
// guard is needed: systray delivers clicks with a non-blocking send (dropped
// while promptInterval blocks), and runModal occupies the Cocoa main thread,
// so the menu cannot even open while the dialog is up.
func (t *Tray) watchIntervalPrompt(item *systray.MenuItem) {
	for range item.ClickedCh {
		t.promptInterval(item)
	}
}

func (t *Tray) promptInterval(item *systray.MenuItem) {
	current := t.store.Get().IntervalSec
	message := fmt.Sprintf("Seconds between updates (%d–%d).",
		config.MinIntervalSeconds, config.MaxIntervalSeconds)

	entered, ok := promptString(intervalPrefix, message, strconv.Itoa(current))
	if !ok {
		return // Cancel keeps the old value
	}

	v, ok := parseInterval(entered)
	if !ok {
		return // garbage input is a no-op, same as Cancel
	}

	t.updateConfig(func(c *config.Config) { c.IntervalSec = v })
	item.SetTitle(intervalTitle(v))
	t.refresh()
}
