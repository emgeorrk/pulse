//go:build darwin

package autostart

import (
	"strings"
	"testing"
)

func TestPlist(t *testing.T) {
	t.Parallel()

	p := Plist("/Applications/Pulse.app/Contents/MacOS/pulse")

	tests := []struct {
		name string
		want string
	}{
		{name: "label", want: "<string>com.emgeorrk.pulse</string>"},
		{name: "program path", want: "<string>/Applications/Pulse.app/Contents/MacOS/pulse</string>"},
		{name: "run at load key", want: "<key>RunAtLoad</key>"},
		{name: "run at load value", want: "<true/>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if !strings.Contains(p, tt.want) {
				t.Errorf("plist missing %q:\n%s", tt.want, p)
			}
		})
	}
}
