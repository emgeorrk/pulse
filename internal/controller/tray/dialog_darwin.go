//go:build darwin

package tray

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>
#include <stdlib.h>
#include <string.h>

// pulse_prompt_text runs a modal NSAlert with a one-line text field on the
// Cocoa main thread; returns strdup'd entered text on OK, NULL on Cancel.
// The caller frees the result.
static char *pulse_prompt_text(const char *title, const char *message, const char *initial) {
	__block char *result = NULL;

	dispatch_sync(dispatch_get_main_queue(), ^{
		NSAlert *alert = [[NSAlert alloc] init];
		alert.messageText = [NSString stringWithUTF8String:title];
		alert.informativeText = [NSString stringWithUTF8String:message];
		[alert addButtonWithTitle:@"OK"];     // first button is the default, bound to Return
		[alert addButtonWithTitle:@"Cancel"]; // the "Cancel" title binds Esc automatically

		NSTextField *field = [[NSTextField alloc] initWithFrame:NSMakeRect(0, 0, 200, 24)];
		field.stringValue = [NSString stringWithUTF8String:initial];
		alert.accessoryView = field;

		// An LSUIElement app is never active on its own; without activation
		// the alert opens behind other windows and gets no key focus.
		if (@available(macOS 14.0, *)) {
			[NSApp activate];
		} else {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
			[NSApp activateIgnoringOtherApps:YES];
#pragma clang diagnostic pop
		}

		[alert layout]; // materializes alert.window
		[alert.window makeFirstResponder:field];
		[field selectText:nil]; // preselect so typing replaces the old value

		if ([alert runModal] == NSAlertFirstButtonReturn) {
			result = strdup(field.stringValue.UTF8String);
		}
	});

	return result;
}
*/
import "C"

import "unsafe"

// promptString shows a modal one-line input dialog and reports the entered
// text; ok is false when the user canceled. Safe to call from any watcher
// goroutine: the main goroutine is locked to the Cocoa main thread (systray),
// so this never runs there, and dispatch_sync hands the UI work over.
func promptString(title, message, initial string) (value string, ok bool) {
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))

	cMessage := C.CString(message)
	defer C.free(unsafe.Pointer(cMessage))

	cInitial := C.CString(initial)
	defer C.free(unsafe.Pointer(cInitial))

	out := C.pulse_prompt_text(cTitle, cMessage, cInitial)
	if out == nil {
		return "", false
	}
	defer C.free(unsafe.Pointer(out))

	return C.GoString(out), true
}
