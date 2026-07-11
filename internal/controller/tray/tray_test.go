//go:build darwin

package tray

import "testing"

// prettyModel must not duplicate the chip name (shown on its own System
// line) and must return the name as-is if the product-name format is unfamiliar.
func TestPrettyModel(t *testing.T) {
	cases := []struct {
		name, chip, want string
	}{
		{"MacBook Pro (16-inch, M5 Pro)", "Apple M5 Pro", "MacBook Pro 16-inch"},
		{"MacBook Air (13-inch, M3, 2024)", "Apple M3", "MacBook Air 13-inch 2024"},
		{"Mac mini (M4, 2024)", "Apple M4", "Mac mini 2024"},
		{"iMac", "Apple M1", "iMac"},
	}
	for _, c := range cases {
		if got := prettyModel(c.name, c.chip); got != c.want {
			t.Errorf("prettyModel(%q, %q) = %q, want %q", c.name, c.chip, got, c.want)
		}
	}
}
