//go:build darwin

package sensors

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <string.h>
#include <IOKit/IOKitLib.h>
#include <CoreFoundation/CoreFoundation.h>

// pulse_product_name reads product-name from IODeviceTree:/product — a
// human-readable model name ("MacBook Pro (16-inch, M5 Pro)").
// The node only exists on Apple Silicon; on Intel this returns -1.
static int pulse_product_name(char *buf, size_t cap) {
	io_registry_entry_t e = IORegistryEntryFromPath(kIOMainPortDefault,
	    "IODeviceTree:/product");
	if (e == MACH_PORT_NULL) return -1;
	CFTypeRef ref = IORegistryEntryCreateCFProperty(e,
	    CFSTR("product-name"), kCFAllocatorDefault, 0);
	IOObjectRelease(e);
	if (!ref) return -1;
	int ok = -1;
	if (CFGetTypeID(ref) == CFDataGetTypeID()) {
		CFIndex n = CFDataGetLength(ref);
		if (n > 0 && (size_t)n < cap) {
			memcpy(buf, CFDataGetBytePtr(ref), n);
			buf[n] = '\0';
			ok = 0;
		}
	}
	CFRelease(ref);
	return ok;
}
*/
import "C"

import (
	"runtime"
	"strings"

	"github.com/emgeorrk/pulse/internal/entity"
	"golang.org/x/sys/unix"
)

// ReadHWInfo determines the chip, Mac model, and macOS version. sysctl
// errors aren't fatal — missing fields stay empty, and the UI just won't show them.
func ReadHWInfo() entity.HWInfo {
	chip, _ := unix.Sysctl("machdep.cpu.brand_string")
	model, _ := unix.Sysctl("hw.model")
	osVer, _ := unix.Sysctl("kern.osproductversion")

	chip = strings.TrimSpace(chip)
	if chip == "" {
		chip = runtime.GOARCH
	}

	var modelName string
	var buf [256]C.char
	if C.pulse_product_name(&buf[0], C.size_t(len(buf))) == 0 {
		modelName = strings.TrimSpace(C.GoString(&buf[0]))
	}

	return entity.HWInfo{
		Chip:           chip,
		Model:          model,
		ModelName:      modelName,
		OSVersion:      osVer,
		IsAppleSilicon: strings.Contains(chip, "Apple"),
		NumCores:       runtime.NumCPU(),
	}
}
