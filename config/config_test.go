package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileGivesDefaults(t *testing.T) {
	s := Load(filepath.Join(t.TempDir(), "nope", "config.json"))
	c := s.Get()
	if c.IntervalSec != 2 || c.TempUnit != Celsius || !c.ShowSparkline {
		t.Errorf("defaults broken: %+v", c)
	}
	if c.VisualStyle != VisualEmoji || c.BarLabels != BarText {
		t.Errorf("style defaults broken: %+v", c)
	}
	if !c.IsPinned("cpu.total") || !c.IsPinned("mem.used") {
		t.Errorf("default pins broken: %v", c.Pinned)
	}
}

// A config written by an older version (or hand-edited to junk) must
// normalize the style fields instead of leaking unknown values into the UI.
func TestLoadNormalizesStyleFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"interval_sec":2,"visual_style":"neon","bar_labels":"dancing"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := Load(path).Get()
	if c.VisualStyle != VisualEmoji || c.BarLabels != BarText {
		t.Errorf("junk style values not normalized: %+v", c)
	}

	if err := os.WriteFile(path, []byte(`{"interval_sec":2,"visual_style":"gnome","bar_labels":"visual"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c = Load(path).Get()
	if c.VisualStyle != VisualGnome || c.BarLabels != BarVisual {
		t.Errorf("valid style values not kept: %+v", c)
	}
}

func TestUpdatePersistsAndReloads(t *testing.T) {
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

func TestTogglePin(t *testing.T) {
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

func TestLoadCorruptFileGivesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, []byte("{not json"), 0o644)
	if c := Load(path).Get(); c.IntervalSec != 2 {
		t.Errorf("corrupt file should give defaults, got %+v", c)
	}
}
