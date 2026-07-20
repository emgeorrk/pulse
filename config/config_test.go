package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInterval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sec  int
		want time.Duration
	}{
		{name: "positive interval", sec: 5, want: 5 * time.Second},
		{name: "one second", sec: 1, want: time.Second},
		{name: "zero falls back to default", sec: 0, want: defaultIntervalSeconds * time.Second},
		{name: "negative falls back to default", sec: -3, want: defaultIntervalSeconds * time.Second},
		{name: "max kept", sec: MaxIntervalSeconds, want: MaxIntervalSeconds * time.Second},
		{name: "above max clamps to max", sec: MaxIntervalSeconds + 1, want: MaxIntervalSeconds * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := (Config{IntervalSec: tt.sec}).Interval(); got != tt.want {
				t.Errorf("Interval() = %v, want %v", got, tt.want)
			}
		})
	}
}

// DefaultPath depends only on the OS home dir, so a single invariant check.
func TestDefaultPath(t *testing.T) {
	t.Parallel()

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}

	if want := filepath.Join("pulse", "config.json"); !strings.HasSuffix(path, want) {
		t.Errorf("DefaultPath() = %q, want suffix %q", path, want)
	}
}

// A store with an empty path keeps changes in memory but must not error on
// save (the persistence-optional branch).
func TestUpdateWithoutPath(t *testing.T) {
	t.Parallel()

	s := Load("")
	if err := s.Update(func(c *Config) { c.IntervalSec = 3 }); err != nil {
		t.Errorf("Update on empty-path store: %v", err)
	}

	if got := s.Get().IntervalSec; got != 3 {
		t.Errorf("IntervalSec = %d, want 3 (change kept in memory)", got)
	}

	if pinned := s.TogglePin("test.metric"); pinned != s.Get().IsPinned("test.metric") {
		t.Errorf("TogglePin returned %v but IsPinned = %v", pinned, s.Get().IsPinned("test.metric"))
	}
}

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		content      string // written to the config file; empty means the file is missing
		wantStyle    VisualStyle
		wantBar      BarLabelStyle
		wantInterval int
		wantDefaults bool // expect full defaults(): sparkline off, default pins
	}{
		{
			name:         "missing file gives defaults",
			wantStyle:    VisualEmoji,
			wantBar:      BarVisual,
			wantInterval: defaultIntervalSeconds,
			wantDefaults: true,
		},
		{
			name:         "corrupt file gives defaults",
			content:      "{not json",
			wantStyle:    VisualEmoji,
			wantBar:      BarVisual,
			wantInterval: defaultIntervalSeconds,
			wantDefaults: true,
		},
		{
			// A config written by an older version (or hand-edited to junk) must
			// normalize the style fields instead of leaking unknown values into the UI.
			name:         "junk style values normalized",
			content:      `{"interval_sec":2,"visual_style":"neon","bar_labels":"dancing"}`,
			wantStyle:    VisualEmoji,
			wantBar:      BarVisual,
			wantInterval: 2,
		},
		{
			name:         "valid style values kept",
			content:      `{"interval_sec":2,"visual_style":"emoji","bar_labels":"text"}`,
			wantStyle:    VisualEmoji,
			wantBar:      BarText,
			wantInterval: 2,
		},
		{
			name:         "classic style kept",
			content:      `{"interval_sec":2,"visual_style":"classic","bar_labels":"visual"}`,
			wantStyle:    VisualClassic,
			wantBar:      BarVisual,
			wantInterval: 2,
		},
		{
			// Hand-edited intervals beyond the stepper max clamp on load so the
			// menu title never shows a value the sampler won't honor.
			name:         "interval above max clamps",
			content:      `{"interval_sec":120,"visual_style":"emoji","bar_labels":"text"}`,
			wantStyle:    VisualEmoji,
			wantBar:      BarText,
			wantInterval: MaxIntervalSeconds,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "config.json")
			if tt.content != "" {
				if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			c := Load(path).Get()

			if c.IntervalSec != tt.wantInterval || c.TempUnit != Celsius {
				t.Errorf("interval/unit = %d/%s, want %d/%s", c.IntervalSec, c.TempUnit, tt.wantInterval, Celsius)
			}

			if c.VisualStyle != tt.wantStyle || c.BarLabels != tt.wantBar {
				t.Errorf("style = %s/%s, want %s/%s", c.VisualStyle, c.BarLabels, tt.wantStyle, tt.wantBar)
			}

			if tt.wantDefaults {
				if c.ShowSparkline {
					t.Error("ShowSparkline = true, want default false")
				}

				if !c.IsPinned("cpu.total") || !c.IsPinned("mem.usage") || !c.IsPinned("temp.hottest") {
					t.Errorf("default pins broken: %v", c.Pinned)
				}
			}
		})
	}
}

// The newer boolean toggles must default to off and survive a
// persist-and-reload round trip; a single scenario, so no table here.
func TestLoadNewToggles(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")

	s := Load(path)
	if c := s.Get(); c.HigherPrecision || c.ShowPublicIP || c.FixedWidth {
		t.Errorf("defaults: HigherPrecision=%t ShowPublicIP=%t FixedWidth=%t, want all false", c.HigherPrecision, c.ShowPublicIP, c.FixedWidth)
	}

	if err := s.Update(func(c *Config) { c.HigherPrecision = true; c.ShowPublicIP = true; c.FixedWidth = true }); err != nil {
		t.Fatal(err)
	}

	c := Load(path).Get()
	if !c.HigherPrecision || !c.ShowPublicIP || !c.FixedWidth {
		t.Errorf("after reload: HigherPrecision=%t ShowPublicIP=%t FixedWidth=%t, want all true", c.HigherPrecision, c.ShowPublicIP, c.FixedWidth)
	}
}

// A single persist-and-reload scenario, so no table here.
func TestUpdate(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")

	s := Load(path)
	if err := s.Update(func(c *Config) { c.IntervalSec = 5; c.TempUnit = Fahrenheit }); err != nil {
		t.Fatalf("Update: %v", err)
	}

	re := Load(path).Get()
	if re.IntervalSec != 5 || re.TempUnit != Fahrenheit {
		t.Errorf("reloaded config = %+v, want interval 5, unit F", re)
	}
}

// A sequential pin/unpin scenario over shared state, so no table here.
func TestTogglePin(t *testing.T) {
	t.Parallel()

	s := Load(filepath.Join(t.TempDir(), "config.json"))

	if pinned := s.TogglePin("temp.cpu"); !pinned {
		t.Error("first toggle should pin")
	}

	if !s.Get().IsPinned("temp.cpu") {
		t.Error("temp.cpu should be pinned")
	}

	if pinned := s.TogglePin("temp.cpu"); pinned {
		t.Error("second toggle should unpin")
	}

	if s.Get().IsPinned("temp.cpu") {
		t.Error("temp.cpu should be unpinned")
	}

	// pin order is preserved
	s.TogglePin("a")
	s.TogglePin("b")

	got := s.Get().Pinned
	if got[len(got)-2] != "a" || got[len(got)-1] != "b" {
		t.Errorf("pin order broken: %v", got)
	}
}
