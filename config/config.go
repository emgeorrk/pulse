// Package config задаёт настройки приложения. В v0.1 — только значения по
// умолчанию; загрузка пользовательских настроек появится вместе с prefs UI.
package config

import "time"

type Config struct {
	// Interval — период опроса сенсоров (Vitals по умолчанию тоже ~2 c).
	Interval time.Duration
	// Какие метрики показывать инлайн в menu bar.
	ShowCPUInBar bool
	ShowMemInBar bool
}

func New() Config {
	return Config{
		Interval:     2 * time.Second,
		ShowCPUInBar: true,
		ShowMemInBar: true,
	}
}
