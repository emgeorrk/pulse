// Package icons embeds the monochrome GNOME-style metric icons (from the
// Vitals GNOME Shell extension, see README.md) as 64x64 PNGs. Only the
// alpha channel matters: menu items draw them as macOS template images,
// and the menu bar title tints them to the text color at draw time, so the
// glyphs adapt to light/dark menu bars automatically.
package icons

import "embed"

//go:embed png/*.png
var pngs embed.FS

// Icon keys. Each matches a png/<key>.png asset.
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
	Settings        = "settings" // SF Symbol, not from the Vitals set
)

// Keys lists every embedded icon key.
var Keys = []string{
	CPU, Memory, Temperature, Fan, Voltage,
	Network, NetworkDownload, NetworkUpload,
	Storage, Battery, GPU, System, Settings,
}

// PNG returns the embedded template PNG for key, or nil if the key is
// unknown — callers fall back to text rather than crash.
func PNG(key string) []byte {
	b, err := pngs.ReadFile("png/" + key + ".png")
	if err != nil {
		return nil
	}

	return b
}
