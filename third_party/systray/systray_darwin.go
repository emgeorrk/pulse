//go:build !ios

package systray

/*
#cgo darwin CFLAGS: -DDARWIN -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa

#include <stdbool.h>
#include "systray.h"

void setInternalLoop(bool);
*/
import "C"

import (
	"fmt"
	"os"
	"unsafe"
)

// SetTemplateIcon sets the systray icon as a template icon (on Mac), falling back
// to a regular icon on other platforms.
// templateIconBytes and regularIconBytes should be the content of .ico for windows and
// .ico/.jpg/.png for other platforms.
func SetTemplateIcon(templateIconBytes []byte, regularIconBytes []byte) {
	cstr := (*C.char)(unsafe.Pointer(&templateIconBytes[0]))
	C.setIcon(cstr, (C.int)(len(templateIconBytes)), true)
}

// SetIcon sets the icon of a menu item. Only works on macOS and Windows.
// iconBytes should be the content of .ico/.jpg/.png
func (item *MenuItem) SetIcon(iconBytes []byte) {
	cstr := (*C.char)(unsafe.Pointer(&iconBytes[0]))
	C.setMenuItemIcon(cstr, (C.int)(len(iconBytes)), C.int(item.id), false)
}

// SetIconFromFilePath sets the icon of a menu item from a file path.
// iconFilePath should be the path to a .ico for windows and .ico/.jpg/.png for other platforms.
func (item *MenuItem) SetIconFromFilePath(iconFilePath string) error {
	iconBytes, err := os.ReadFile(iconFilePath)
	if err != nil {
		return fmt.Errorf("failed to read icon file: %v", err)
	}
	item.SetIcon(iconBytes)
	return nil
}

// SetTemplateIcon sets the icon of a menu item as a template icon (on macOS). On Windows, it
// falls back to the regular icon bytes and on Linux it does nothing.
// templateIconBytes and regularIconBytes should be the content of .ico for windows and
// .ico/.jpg/.png for other platforms.
func (item *MenuItem) SetTemplateIcon(templateIconBytes []byte, regularIconBytes []byte) {
	cstr := (*C.char)(unsafe.Pointer(&templateIconBytes[0]))
	C.setMenuItemIcon(cstr, (C.int)(len(templateIconBytes)), C.int(item.id), true)
}

// SetRemovalAllowed sets whether a user can remove the systray icon or not.
// This is only supported on macOS.
func SetRemovalAllowed(allowed bool) {
	C.setRemovalAllowed((C.bool)(allowed))
}

func registerSystray() {
	C.registerSystray()
}

func nativeLoop() {
	C.nativeLoop()
}

func nativeEnd() {
	C.nativeEnd()
}

func nativeStart() {
	C.nativeStart()
}

func quit() {
	C.quit()
}

func setInternalLoop(internal bool) {
	C.setInternalLoop(C.bool(internal))
}

// SetIcon sets the systray icon.
// iconBytes should be the content of .ico for windows and .ico/.jpg/.png
// for other platforms.
func SetIcon(iconBytes []byte) {
	cstr := (*C.char)(unsafe.Pointer(&iconBytes[0]))
	C.setIcon(cstr, (C.int)(len(iconBytes)), false)
}

// SetIconFromFilePath sets the systray icon from a file path.
// iconFilePath should be the path to a .ico for windows and .ico/.jpg/.png for other platforms.
func SetIconFromFilePath(iconFilePath string) error {
	bytes, err := os.ReadFile(iconFilePath)
	if err != nil {
		return fmt.Errorf("failed to read icon file: %v", err)
	}
	SetIcon(bytes)
	return nil
}

// SetTitle sets the systray title, only available on Mac and Linux.
func SetTitle(title string) {
	C.setTitle(C.CString(title))
}

// PATCH(pulse): attributed status-item titles with inline template icons.

// RegisterTitleIcon registers a template image (PNG bytes, glyph + alpha)
// under key for use in SetTitleParts. Register each icon once, before the
// first SetTitleParts call that references it.
func RegisterTitleIcon(key string, png []byte) {
	registerTitleIcon(key, png, false)
}

// RegisterColorTitleIcon registers a full-color image (country flags) drawn
// as-is in the title, without the labelColor tint template icons get.
func RegisterColorTitleIcon(key string, png []byte) {
	registerTitleIcon(key, png, true)
}

func registerTitleIcon(key string, png []byte, color bool) {
	if len(png) == 0 {
		return
	}
	cstr := (*C.char)(unsafe.Pointer(&png[0]))
	C.registerTitleIcon(C.CString(key), cstr, (C.int)(len(png)), C.bool(color))
}

// SetTitleParts sets the status item title as attributed text where each
// part may start with a registered template icon.
func SetTitleParts(parts []TitlePart) {
	C.setTitleParts(C.CString(encodeTitleParts(parts)))
}

// SetTitleFixedWidth pins the status item to the widest title measured since
// the last call, so live values changing digit count don't shift neighboring
// menu bar items; digits render monospaced while it is on. Every call resets
// the tracked width — the next SetTitle/SetTitleParts re-fits it — so call it
// again after a config change to let the item shrink to the new content.
// Passing false restores automatic sizing. PATCH(pulse): macOS only.
func SetTitleFixedWidth(on bool) {
	C.setTitleFixedWidth(C.bool(on))
}

// SetEmojiIcon renders emoji into the menu item's icon slot (macOS only).
// PATCH(pulse): with an image present in every visual style, a style switch
// only swaps image contents — an open menu never gains or loses its icon
// column, which AppKit re-lays out incorrectly while the menu is showing.
func (item *MenuItem) SetEmojiIcon(emoji string) {
	C.setMenuItemEmojiIcon(C.int(item.id), C.CString(emoji))
}

// ClearIcon removes the icon of a menu item.
func (item *MenuItem) ClearIcon() {
	C.clearMenuItemIcon(C.int(item.id))
}

// SetTooltip sets the systray tooltip to display on mouse hover of the tray icon,
// only available on Mac and Windows.
func SetTooltip(tooltip string) {
	C.setTooltip(C.CString(tooltip))
}

func addOrUpdateMenuItem(item *MenuItem) {
	var disabled C.short
	if item.disabled {
		disabled = 1
	}
	var checked C.short
	if item.checked {
		checked = 1
	}
	var isCheckable C.short
	if item.isCheckable {
		isCheckable = 1
	}
	var parentID uint32 = 0
	if item.parent != nil {
		parentID = item.parent.id
	}
	C.add_or_update_menu_item(
		C.int(item.id),
		C.int(parentID),
		C.CString(item.title),
		C.CString(item.tooltip),
		disabled,
		checked,
		isCheckable,
	)
}

func addSeparator(id uint32, parent uint32) {
	C.add_separator(C.int(id), C.int(parent))
}

// PATCH(pulse): attach a keep-open view so a click doesn't dismiss the menu.
func keepMenuOpen(item *MenuItem) {
	C.set_menu_item_keep_open(C.int(item.id))
}

func hideMenuItem(item *MenuItem) {
	C.hide_menu_item(
		C.int(item.id),
	)
}

func showMenuItem(item *MenuItem) {
	C.show_menu_item(
		C.int(item.id),
	)
}

func removeMenuItem(item *MenuItem) {
	C.remove_menu_item(
		C.int(item.id),
	)
}

func resetMenu() {
	C.reset_menu()
}

//export systray_left_click
func systray_left_click() {
	if fn := tappedLeft; fn != nil {
		fn()
		return
	}

	C.show_menu()
}

//export systray_right_click
func systray_right_click() {
	if fn := tappedRight; fn != nil {
		fn()
		return
	}

	C.show_menu()
}

//export systray_ready
func systray_ready() {
	systrayReady()
}

//export systray_on_exit
func systray_on_exit() {
	runSystrayExit()
}

//export systray_menu_item_selected
func systray_menu_item_selected(cID C.int) {
	systrayMenuItemSelected(uint32(cID))
}

//export systray_menu_will_open
func systray_menu_will_open() {
	select {
	case TrayOpenedCh <- struct{}{}:
	default:
	}
}

