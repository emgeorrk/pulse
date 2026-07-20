// Package icons embeds the monochrome metric icon packs (from the Vitals
// GNOME Shell extension, see README.md) as 64x64 PNGs, one set per style:
// "gnome" (standard GNOME symbolic) and "classic" (Vitals' older, more
// detailed set). Only the alpha channel matters: menu items draw them as
// macOS template images, and the menu bar title tints them to the text color
// at draw time, so the glyphs adapt to light/dark menu bars automatically.
// The one exception is the country flags (png/flags): full-color images
// shared by both icon-pack styles, drawn untinted next to the public IP.
package icons

import (
	"embed"
	"strings"
)

//go:embed png
var pngs embed.FS

// Icon keys. Each matches a png/<style>/<key>.png asset, except the shared
// SF Symbols (Settings, About, ActivityMonitor), which live at the png root
// because they only appear on menu items, never in the menu bar title.
const (
	CPU             = "cpu"
	Memory          = "memory"
	Temperature     = "temperature"
	Fan             = "fan"
	Power           = "power" // SF Symbol, rendered into each style pack
	Network         = "network"
	NetworkDownload = "network-download"
	NetworkUpload   = "network-upload"
	Storage         = "storage"
	Battery         = "battery"
	GPU             = "gpu"
	System          = "system"
	Settings        = "settings" // SF Symbol, shared across styles
	About           = "about"    // SF Symbol, shared across styles
	ActivityMonitor = "activity" // SF Symbol, shared across styles
)

// ImageStyles lists the styles backed by a PNG pack. The strings equal the
// config.Visual* values; the package stays config-free to avoid an import
// cycle, so the two are kept in sync deliberately.
func ImageStyles() []string {
	return []string{"gnome", "classic"}
}

// MetricKeys returns every metric icon key (the shared menu-item symbols —
// Settings, About, ActivityMonitor — are not per-style metrics and never
// appear in the menu bar, so they are excluded).
func MetricKeys() []string {
	return []string{
		CPU, Memory, Temperature, Fan, Power,
		Network, NetworkDownload, NetworkUpload,
		Storage, Battery, GPU, System,
	}
}

// TitleKey is the menu-bar title-icon id for a style's metric icon; namespacing
// by style keeps the two packs' icons distinct in the systray title registry.
func TitleKey(style, key string) string {
	return style + "/" + key
}

// FlagKeyPrefix namespaces flag title-icon keys ("flag/us") apart from the
// style/metric keys ("gnome/cpu").
const FlagKeyPrefix = "flag/"

// countryCodeLen is the length of an ISO 3166-1 alpha-2 country code.
const countryCodeLen = 2

// FlagPNG returns the full-color flag PNG for an ISO 3166-1 alpha-2 country
// code (case-insensitive), or nil when the code or its asset is unknown —
// callers fall back to the group icon / bare text. One flag set (the Vitals
// 1x1 pack) is shared by the gnome and classic styles.
func FlagPNG(cc string) []byte {
	if len(cc) != countryCodeLen {
		return nil
	}

	b, err := pngs.ReadFile("png/flags/" + strings.ToLower(cc) + ".png")
	if err != nil {
		return nil
	}

	return b
}

// FlagTitleKey is the menu-bar title-icon id for a country flag.
func FlagTitleKey(cc string) string {
	return FlagKeyPrefix + strings.ToLower(cc)
}

// PNG returns the embedded template PNG for a style's key, or nil if it is
// missing — callers fall back to text rather than crash. The shared menu-item
// symbols are resolved regardless of style.
func PNG(style, key string) []byte {
	path := "png/" + style + "/" + key + ".png"

	switch key {
	case Settings, About, ActivityMonitor:
		path = "png/" + key + ".png"
	}

	b, err := pngs.ReadFile(path)
	if err != nil {
		return nil
	}

	return b
}
