module github.com/emgeorrk/pulse

go 1.26

require (
	fyne.io/systray v1.12.2
	golang.org/x/sys v0.47.0
)

require github.com/godbus/dbus/v5 v5.1.0 // indirect

// Local fork of v1.12.2 patching systray_darwin.m: items with a submenu have
// their action removed, otherwise on macOS 14+ clicking them closes the
// whole menu. Patches are marked "PATCH(pulse)".
replace fyne.io/systray => ./third_party/systray
