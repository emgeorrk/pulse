//go:build darwin

package tray

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/systray"
	"github.com/emgeorrk/pulse/config"
)

// intervalPrefix labels the inline interval editor row in Settings.
const intervalPrefix = "Update interval"

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

// addIntervalEditor builds the "Update interval [N] s" Settings row whose
// value is typed directly in the menu: the row carries an inline text field,
// submitted on Return or when the field loses focus (menu closing included).
func (t *Tray) addIntervalEditor(parent *systray.MenuItem, cfg config.Config) {
	tip := fmt.Sprintf("Seconds between updates (%d–%d)",
		config.MinIntervalSeconds, config.MaxIntervalSeconds)
	item := parent.AddSubMenuItem(intervalPrefix, tip)
	item.EnableInlineEdit(strconv.Itoa(cfg.IntervalSec), "s")

	go t.watchIntervalEdits(item)
}

// watchIntervalEdits applies each submitted value and echoes the accepted
// (clamped) one back into the field; garbage input reverts the field to the
// stored value. Submissions racing a slow apply are dropped by the buffered
// channel — the same policy systray uses for clicks.
func (t *Tray) watchIntervalEdits(item *systray.MenuItem) {
	for text := range item.EditedCh {
		v, ok := parseInterval(text)
		if !ok {
			item.SetInlineEditText(strconv.Itoa(t.store.Get().IntervalSec))

			continue
		}

		t.updateConfig(func(c *config.Config) { c.IntervalSec = v })
		item.SetInlineEditText(strconv.Itoa(v))
		t.refresh()
	}
}
