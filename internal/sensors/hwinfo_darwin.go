//go:build darwin

package sensors

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <string.h>
#include <IOKit/IOKitLib.h>
#include <CoreFoundation/CoreFoundation.h>

// pulse_product_name читает product-name из IODeviceTree:/product —
// человекочитаемое имя модели ("MacBook Pro (16-inch, M5 Pro)").
// Узел есть только на Apple Silicon; на Intel вернёт -1.
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

	"golang.org/x/sys/unix"

	"github.com/emgeorrk/pulse/internal/entity"
)

// ReadHWInfo определяет чип, модель Mac и версию macOS. Ошибки sysctl не
// фатальны — недостающие поля остаются пустыми, UI их просто не покажет.
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
