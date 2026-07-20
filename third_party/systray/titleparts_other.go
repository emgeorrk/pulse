//go:build !darwin

package systray

// PATCH(pulse): no-op fallbacks — inline title icons are macOS-only.

// RegisterTitleIcon is a no-op outside macOS.
func RegisterTitleIcon(key string, png []byte) {}

// SetTitleParts falls back to the concatenated text via SetTitle.
func SetTitleParts(parts []TitlePart) {
	var b []byte
	for _, p := range parts {
		b = append(b, p.Text...)
	}
	SetTitle(string(b))
}

// SetTitleFixedWidth is a no-op outside macOS.
func SetTitleFixedWidth(on bool) {}

// ClearIcon is a no-op outside macOS.
func (item *MenuItem) ClearIcon() {}
