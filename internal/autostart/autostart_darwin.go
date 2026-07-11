//go:build darwin

// Package autostart manages launch-at-login via a LaunchAgent plist in
// ~/Library/LaunchAgents (not SMAppService — this avoids requiring ObjC and
// works with ad-hoc signing).
package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	label           = "com.emgeorrk.pulse"
	directoryMode   = 0o755
	privateFileMode = 0o600
)

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, "Library", "LaunchAgents", label+".plist"), nil
}

// Plist generates the LaunchAgent plist content for the given binary.
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

// Enable writes a plist pointing at the current binary.
func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	path, err := plistPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), directoryMode); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(Plist(exe)), privateFileMode)
}

// Disable removes the plist; a missing file is not an error.
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
