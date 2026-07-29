package icons

import "testing"

func TestSharedMenuIcons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  string
	}{
		{name: "settings", key: Settings},
		{name: "about", key: About},
		{name: "activity monitor", key: ActivityMonitor},
		{name: "update", key: Update},
		{name: "quit", key: Quit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if png := PNG("", tt.key); len(png) == 0 {
				t.Errorf("PNG(%q) is empty", tt.key)
			}
		})
	}
}

func TestFlagPNG(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cc   string
		want bool // non-nil PNG expected
	}{
		{name: "upper", cc: "US", want: true},
		{name: "lower", cc: "nl", want: true},
		{name: "mixed case", cc: "nL", want: true},
		{name: "empty", cc: "", want: false},
		{name: "no asset", cc: "ZZ", want: false},
		{name: "three letters", cc: "USA", want: false},
		{name: "path trick", cc: "u/", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := FlagPNG(tt.cc) != nil; got != tt.want {
				t.Errorf("FlagPNG(%q) non-nil = %v, want %v", tt.cc, got, tt.want)
			}
		})
	}
}

func TestFlagTitleKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cc   string
		want string
	}{
		{name: "upper is lowered", cc: "US", want: "flag/us"},
		{name: "lower stays", cc: "nl", want: "flag/nl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := FlagTitleKey(tt.cc); got != tt.want {
				t.Errorf("FlagTitleKey(%q) = %q, want %q", tt.cc, got, tt.want)
			}
		})
	}
}
