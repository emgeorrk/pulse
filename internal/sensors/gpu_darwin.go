//go:build darwin

package sensors

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <stdint.h>
#include <IOKit/IOKitLib.h>
#include <CoreFoundation/CoreFoundation.h>

// pulse_gpu_util ищет первый IOAccelerator с PerformanceStatistics
// (на Apple Silicon это AGXAccelerator) и берёт "Device Utilization %".
static int pulse_gpu_util(double *util) {
	io_iterator_t iter;
	if (IOServiceGetMatchingServices(kIOMainPortDefault,
	        IOServiceMatching("IOAccelerator"), &iter) != KERN_SUCCESS)
		return -1;

	int found = -1;
	io_registry_entry_t entry;
	while (found != 0 && (entry = IOIteratorNext(iter))) {
		CFMutableDictionaryRef props = NULL;
		if (IORegistryEntryCreateCFProperties(entry, &props, kCFAllocatorDefault,
		        kNilOptions) == KERN_SUCCESS && props) {
			CFDictionaryRef stats =
			    CFDictionaryGetValue(props, CFSTR("PerformanceStatistics"));
			if (stats) {
				CFNumberRef n = CFDictionaryGetValue(stats, CFSTR("Device Utilization %"));
				long long v;
				if (n && CFNumberGetValue(n, kCFNumberSInt64Type, &v)) {
					*util = (double)v / 100.0;
					found = 0;
				}
			}
			CFRelease(props);
		}
		IOObjectRelease(entry);
	}
	IOObjectRelease(iter);
	return found;
}
*/
import "C"

import (
	"fmt"

	"github.com/emgeorrk/pulse/internal/entity"
)

// GPUSensor читает загрузку GPU из IOAccelerator PerformanceStatistics.
type GPUSensor struct{}

func NewGPU() *GPUSensor { return &GPUSensor{} }

func (*GPUSensor) GPU() (entity.GPUStats, error) {
	var util C.double
	if C.pulse_gpu_util(&util) != 0 {
		return entity.GPUStats{}, fmt.Errorf("IOAccelerator statistics unavailable")
	}
	u := float64(util)
	if u < 0 {
		u = 0
	}
	if u > 1 {
		u = 1
	}
	return entity.GPUStats{Utilization: u}, nil
}
