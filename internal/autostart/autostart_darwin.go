//go:build darwin

// Package autostart управляет запуском при логине через LaunchAgent-плист
// в ~/Library/LaunchAgents (без SMAppService — не требует ObjC и работает
// с ad-hoc подписью).
package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

const label = "com.emgeorrk.pulse"

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist"), nil
}

// Plist генерирует содержимое LaunchAgent-плиста для данного бинарника.
func Plist(execPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>ProcessType</key>
	<string>Background</string>
</dict>
</plist>
`, label, execPath)
}

// Enable записывает плист, указывающий на текущий бинарник.
func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	path, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(Plist(exe)), 0o644)
}

// Disable удаляет плист; отсутствие файла — не ошибка.
func Disable() error {
	path, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
