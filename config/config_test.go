package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		content      string // written to the config file; empty means the file is missing
		wantStyle    VisualStyle
		wantBar      BarLabelStyle
		wantDefaults bool // expect full defaults(): sparkline on, default pins
	}{
		{
			name:         "missing file gives defaults",
			wantStyle:    VisualEmoji,
			wantBar:      BarText,
			wantDefaults: true,
		},
		{
			name:         "corrupt file gives defaults",
			content:      "{not json",
			wantStyle:    VisualEmoji,
			wantBar:      BarText,
			wantDefaults: true,
		},
		{
			// A config written by an older version (or hand-edited to junk) must
			// normalize the style fields instead of leaking unknown values into the UI.
			name:      "junk style values normalized",
			content:   `{"interval_sec":2,"visual_style":"neon","bar_labels":"dancing"}`,
			wantStyle: VisualEmoji,
			wantBar:   BarText,
		},
		{
			name:      "valid style values kept",
			content:   `{"interval_sec":2,"visual_style":"gnome","bar_labels":"visual"}`,
			wantStyle: VisualGnome,
			wantBar:   BarVisual,
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

			if c.IntervalSec != 2 || c.TempUnit != Celsius {
				t.Errorf("interval/unit = %d/%s, want 2/%s", c.IntervalSec, c.TempUnit, Celsius)
			}

			if c.VisualStyle != tt.wantStyle || c.BarLabels != tt.wantBar {
				t.Errorf("style = %s/%s, want %s/%s", c.VisualStyle, c.BarLabels, tt.wantStyle, tt.wantBar)
			}

			if tt.wantDefaults {
				if !c.ShowSparkline {
					t.Error("ShowSparkline = false, want default true")
				}

				if !c.IsPinned("cpu.total") || !c.IsPinned("mem.used") {
					t.Errorf("default pins broken: %v", c.Pinned)
				}
			}
		})
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
