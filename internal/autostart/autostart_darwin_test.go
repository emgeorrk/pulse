//go:build darwin

package autostart

import (
	"strings"
	"testing"
)

func TestPlist(t *testing.T) {
	p := Plist("/Applications/Pulse.app/Contents/MacOS/pulse")
	for _, want := range []string{
		"<string>com.emgeorrk.pulse</string>",
		"<string>/Applications/Pulse.app/Contents/MacOS/pulse</string>",
		"<key>RunAtLoad</key>",
		"<true/>",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("plist missing %q:\n%s", want, p)
		}
	}
}
