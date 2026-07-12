// Package icons embeds the monochrome metric icon packs (from the Vitals
// GNOME Shell extension, see README.md) as 64x64 PNGs, one set per style:
// "gnome" (standard GNOME symbolic) and "classic" (Vitals' older, more
// detailed set). Only the alpha channel matters: menu items draw them as
// macOS template images, and the menu bar title tints them to the text color
// at draw time, so the glyphs adapt to light/dark menu bars automatically.
package icons

import "embed"

//go:embed png
var pngs embed.FS

// Icon keys. Each matches a png/<style>/<key>.png asset, except Settings,
// which is a single shared SF Symbol at png/settings.png (no Vitals gear).
const (
	CPU             = "cpu"
	Memory          = "memory"
	Temperature     = "temperature"
	Fan             = "fan"
	Voltage         = "voltage"
	Network         = "network"
	NetworkDownload = "network-download"
	NetworkUpload   = "network-upload"
	Storage         = "storage"
	Battery         = "battery"
	GPU             = "gpu"
	System          = "system"
	Settings        = "settings" // SF Symbol, shared across styles
)

// ImageStyles lists the styles backed by a PNG pack. The strings equal the
// config.Visual* values; the package stays config-free to avoid an import
// cycle, so the two are kept in sync deliberately.
func ImageStyles() []string {
	return []string{"gnome", "classic"}
}

// MetricKeys returns every metric icon key (the shared Settings gear is not a
// per-style metric and never appears in the menu bar, so it is excluded).
func MetricKeys() []string {
	return []string{
		CPU, Memory, Temperature, Fan, Voltage,
		Network, NetworkDownload, NetworkUpload,
		Storage, Battery, GPU, System,
	}
}

// TitleKey is the menu-bar title-icon id for a style's metric icon; namespacing
// by style keeps the two packs' icons distinct in the systray title registry.
func TitleKey(style, key string) string {
	return style + "/" + key
}

// PNG returns the embedded template PNG for a style's key, or nil if it is
// missing — callers fall back to text rather than crash. The shared Settings
// gear is resolved regardless of style.
func PNG(style, key string) []byte {
	path := "png/" + style + "/" + key + ".png"
	if key == Settings {
		path = "png/settings.png"
	}

	b, err := pngs.ReadFile(path)
	if err != nil {
		return nil
	}

	return b
}
