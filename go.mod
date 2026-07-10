module github.com/emgeorrk/pulse

go 1.26

require (
	fyne.io/systray v1.12.2
	golang.org/x/sys v0.47.0
)

require github.com/godbus/dbus/v5 v5.1.0 // indirect

// Локальный форк v1.12.2 с патчем systray_darwin.m: у пунктов с подменю
// снимается action, иначе на macOS 14+ клик по ним закрывает всё меню.
// Патчи помечены "PATCH(pulse)".
replace fyne.io/systray => ./third_party/systray
