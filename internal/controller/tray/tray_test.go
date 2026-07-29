//go:build darwin

package tray

import (
	"context"
	"errors"
	"testing"

	"github.com/emgeorrk/pulse/config"
	"github.com/emgeorrk/pulse/internal/updatecheck"
)

type fakeUpdateChecker struct {
	release *updatecheck.Release
	err     error
}

func (f fakeUpdateChecker) Check(context.Context) (*updatecheck.Release, error) {
	return f.release, f.err
}

type fakeUpdateItem struct {
	title   string
	tooltip string
	visible bool
}

func (f *fakeUpdateItem) SetTitle(title string) {
	f.title = title
}

func (f *fakeUpdateItem) SetTooltip(tooltip string) {
	f.tooltip = tooltip
}

func (f *fakeUpdateItem) Hide() {
	f.visible = false
}

func (f *fakeUpdateItem) Show() {
	f.visible = true
}

// prettyModel must not duplicate the chip name (shown on its own System
// line) and must return the name as-is if the product-name format is unfamiliar.
func TestPrettyModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		chip string
		want string
	}{
		{"MacBook Pro (16-inch, M5 Pro)", "Apple M5 Pro", "MacBook Pro 16-inch"},
		{"MacBook Air (13-inch, M3, 2024)", "Apple M3", "MacBook Air 13-inch 2024"},
		{"Mac mini (M4, 2024)", "Apple M4", "Mac mini 2024"},
		{"iMac", "Apple M1", "iMac"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := prettyModel(tt.name, tt.chip); got != tt.want {
				t.Errorf("prettyModel(%q, %q) = %q, want %q", tt.name, tt.chip, got, tt.want)
			}
		})
	}
}

// The option builders own the whole label↔config mapping of the Settings
// radio submenus, so one combined table covers them all: the label sequence
// must match the menu design, and applying option i must make it the single
// checked option when the builder re-runs on the resulting config.
func TestSettingsRadioOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		build      func(config.Config) []radioOption
		wantLabels []string
	}{
		{"interval", intervalOptions, []string{"1 s", "2 s", "3 s", "5 s"}},
		{"temperature", tempOptions, []string{"°C", "°F"}},
		{"memory", memoryOptions, []string{"GiB", "GB"}},
		{"icons", iconOptions, []string{"Emoji", "GNOME", "Classic"}},
		{"bar labels", barLabelOptions, []string{"Text", "Icons"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := tt.build(config.Config{})
			if len(opts) != len(tt.wantLabels) {
				t.Fatalf("got %d options, want %d", len(opts), len(tt.wantLabels))
			}

			for i, o := range opts {
				if o.label != tt.wantLabels[i] {
					t.Errorf("option %d label = %q, want %q", i, o.label, tt.wantLabels[i])
				}
			}

			for i := range opts {
				var c config.Config

				opts[i].apply(&c)

				for j, o := range tt.build(c) {
					if want := j == i; o.checked != want {
						t.Errorf("after applying %q: option %q checked = %v, want %v",
							opts[i].label, o.label, o.checked, want)
					}
				}
			}
		})
	}
}

func TestCheckForUpdate(t *testing.T) {
	t.Parallel()

	release := &updatecheck.Release{
		Version: "v1.1.0",
		URL:     "https://github.com/emgeorrk/pulse/releases/tag/v1.1.0",
	}

	tests := []struct {
		checker      fakeUpdateChecker
		name         string
		initialURL   string
		wantURL      string
		wantTitle    string
		wantTooltip  string
		initialShown bool
		wantShown    bool
	}{
		{
			name:        "available release is shown",
			checker:     fakeUpdateChecker{release: release},
			wantURL:     release.URL,
			wantTitle:   updateAvailableTitle,
			wantTooltip: updateTooltip,
			wantShown:   true,
		},
		{
			name:         "no update hides existing row",
			checker:      fakeUpdateChecker{},
			initialURL:   release.URL,
			initialShown: true,
		},
		{
			name:         "error preserves existing notification",
			checker:      fakeUpdateChecker{err: errors.New("temporary failure")},
			initialURL:   release.URL,
			initialShown: true,
			wantURL:      release.URL,
			wantShown:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			item := &fakeUpdateItem{visible: tt.initialShown}
			tray := &Tray{
				updates:   tt.checker,
				updateRow: item,
				updateURL: tt.initialURL,
			}

			tray.checkForUpdate(context.Background())

			if item.visible != tt.wantShown {
				t.Errorf("visible = %v, want %v", item.visible, tt.wantShown)
			}

			if item.title != tt.wantTitle || item.tooltip != tt.wantTooltip {
				t.Errorf("title/tooltip = %q/%q, want %q/%q",
					item.title, item.tooltip, tt.wantTitle, tt.wantTooltip)
			}

			if tray.updateURL != tt.wantURL {
				t.Errorf("update URL = %q, want %q", tray.updateURL, tt.wantURL)
			}
		})
	}
}

func TestOpenAvailableUpdate(t *testing.T) {
	t.Parallel()

	const releaseURL = "https://github.com/emgeorrk/pulse/releases/tag/v1.1.0"

	var opened string

	tray := &Tray{
		updateURL: releaseURL,
		openURL:   func(pageURL string) { opened = pageURL },
	}
	tray.openAvailableUpdate()

	if opened != releaseURL {
		t.Errorf("opened URL = %q, want %q", opened, releaseURL)
	}
}
