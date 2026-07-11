// Package config holds app settings persisted as JSON
// (~/Library/Application Support/pulse/config.json). Accessed via Store:
// UI handlers change settings from their own goroutines, monitor reads the
// interval from the sampling loop.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/emgeorrk/pulse/internal/entity"
)

type TempUnit string

const (
	Celsius    TempUnit = "C"
	Fahrenheit TempUnit = "F"
)

// VisualStyle picks which visuals represent metrics: emoji or the
// GNOME-style monochrome icons (from Vitals).
type VisualStyle string

const (
	VisualEmoji VisualStyle = "emoji"
	VisualGnome VisualStyle = "gnome"
)

// BarLabelStyle picks what precedes each pinned value in the menu bar:
// text tags ("CPU", "MEM"…) or the chosen visual.
type BarLabelStyle string

const (
	BarText   BarLabelStyle = "text"
	BarVisual BarLabelStyle = "visual"
)

type Config struct {
	TempUnit      TempUnit          `json:"temp_unit"`
	VisualStyle   VisualStyle       `json:"visual_style"`
	BarLabels     BarLabelStyle     `json:"bar_labels"`
	Pinned        []entity.MetricID `json:"pinned"`
	IntervalSec   int               `json:"interval_sec"`
	DecimalBytes  bool              `json:"decimal_bytes"`
	ShowSparkline bool              `json:"show_sparkline"`
	StartAtLogin  bool              `json:"start_at_login"`
}

func defaults() Config {
	return Config{
		IntervalSec:   2,
		TempUnit:      Celsius,
		DecimalBytes:  false,
		Pinned:        []entity.MetricID{"cpu.total", "mem.used"},
		ShowSparkline: true,
		VisualStyle:   VisualEmoji,
		BarLabels:     BarText,
	}
}

func (c Config) Interval() time.Duration {
	if c.IntervalSec < 1 {
		return 2 * time.Second
	}

	return time.Duration(c.IntervalSec) * time.Second
}

func (c Config) IsPinned(id entity.MetricID) bool {
	for _, p := range c.Pinned {
		if p == id {
			return true
		}
	}

	return false
}

// Store provides thread-safe access to settings; every change is saved to
// disk immediately.
type Store struct {
	c    Config
	path string
	mu   sync.Mutex
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, "Library", "Application Support", "pulse", "config.json"), nil
}

// Load reads settings from path; if the file is missing or the JSON is
// broken, it returns defaults — the app must be able to start regardless.
func Load(path string) *Store {
	s := &Store{c: defaults(), path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}

	var c Config
	if json.Unmarshal(data, &c) != nil {
		return s
	}

	if c.IntervalSec < 1 {
		c.IntervalSec = defaults().IntervalSec
	}

	if c.TempUnit != Fahrenheit {
		c.TempUnit = Celsius
	}

	if c.VisualStyle != VisualGnome {
		c.VisualStyle = VisualEmoji
	}

	if c.BarLabels != BarVisual {
		c.BarLabels = BarText
	}

	s.c = c

	return s
}

// Get returns a copy of the current settings (Pinned is copied).
func (s *Store) Get() Config {
	s.mu.Lock()
	defer s.mu.Unlock()

	c := s.c
	c.Pinned = append([]entity.MetricID(nil), s.c.Pinned...)

	return c
}

// Update applies a change and saves it to disk. A write error is not fatal:
// the settings still take effect in memory.
func (s *Store) Update(fn func(*Config)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fn(&s.c)

	return s.save()
}

// TogglePin adds/removes a metric from the menu bar; returns the new state.
func (s *Store) TogglePin(id entity.MetricID) bool {
	pinned := false

	s.Update(func(c *Config) {
		for i, p := range c.Pinned {
			if p == id {
				c.Pinned = append(c.Pinned[:i], c.Pinned[i+1:]...)

				return
			}
		}

		c.Pinned = append(c.Pinned, id)
		pinned = true
	})

	return pinned
}

func (s *Store) save() error {
	if s.path == "" {
		return nil
	}

	data, err := json.MarshalIndent(s.c, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmp, s.path)
}
