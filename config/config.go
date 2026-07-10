// Package config — настройки приложения с персистом в JSON
// (~/Library/Application Support/pulse/config.json). Доступ через Store:
// UI-обработчики меняют настройки из своих горутин, monitor читает интервал
// из цикла сэмплирования.
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

type Config struct {
	IntervalSec   int               `json:"interval_sec"`
	TempUnit      TempUnit          `json:"temp_unit"`
	DecimalBytes  bool              `json:"decimal_bytes"` // false → GiB (binary), true → GB (decimal)
	Pinned        []entity.MetricID `json:"pinned"`        // порядок = порядок в menu bar
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

// Store — потокобезопасный доступ к настройкам; каждое изменение сразу
// сохраняется на диск.
type Store struct {
	mu   sync.Mutex
	c    Config
	path string
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support", "pulse", "config.json"), nil
}

// Load читает настройки из path; при отсутствии файла или битом JSON
// возвращает дефолты — приложение должно запускаться в любом случае.
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
	s.c = c
	return s
}

// Get возвращает копию текущих настроек (Pinned копируется).
func (s *Store) Get() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.c
	c.Pinned = append([]entity.MetricID(nil), s.c.Pinned...)
	return c
}

// Update применяет изменение и сохраняет на диск. Ошибка записи не фатальна:
// настройки продолжают действовать в памяти.
func (s *Store) Update(fn func(*Config)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.c)
	return s.save()
}

// TogglePin добавляет/убирает метрику из menu bar; возвращает новое состояние.
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
