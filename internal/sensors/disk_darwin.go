//go:build darwin

package sensors

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <stdint.h>
#include <IOKit/IOKitLib.h>
#include <IOKit/storage/IOBlockStorageDriver.h>
#include <CoreFoundation/CoreFoundation.h>

// pulse_disk_io суммирует байты чтения/записи по всем блочным устройствам.
static int pulse_disk_io(uint64_t *readBytes, uint64_t *writeBytes) {
	io_iterator_t iter;
	if (IOServiceGetMatchingServices(kIOMainPortDefault,
	        IOServiceMatching(kIOBlockStorageDriverClass), &iter) != KERN_SUCCESS)
		return -1;

	uint64_t r = 0, w = 0;
	io_registry_entry_t drive;
	while ((drive = IOIteratorNext(iter))) {
		CFMutableDictionaryRef props = NULL;
		if (IORegistryEntryCreateCFProperties(drive, &props, kCFAllocatorDefault,
		        kNilOptions) == KERN_SUCCESS && props) {
			CFDictionaryRef stats =
			    CFDictionaryGetValue(props, CFSTR(kIOBlockStorageDriverStatisticsKey));
			if (stats) {
				CFNumberRef n;
				int64_t v;
				n = CFDictionaryGetValue(stats,
				        CFSTR(kIOBlockStorageDriverStatisticsBytesReadKey));
				if (n && CFNumberGetValue(n, kCFNumberSInt64Type, &v)) r += (uint64_t)v;
				n = CFDictionaryGetValue(stats,
				        CFSTR(kIOBlockStorageDriverStatisticsBytesWrittenKey));
				if (n && CFNumberGetValue(n, kCFNumberSInt64Type, &v)) w += (uint64_t)v;
			}
			CFRelease(props);
		}
		IOObjectRelease(drive);
	}
	IOObjectRelease(iter);
	*readBytes = r;
	*writeBytes = w;
	return 0;
}
*/
import "C"

import (
	"fmt"

	"golang.org/x/sys/unix"

	"github.com/emgeorrk/pulse/internal/entity"
)

// Disk: заполненность корневого тома через statfs, I/O — через IOKit
// IOBlockStorageDriver (64-битные счётчики с загрузки системы).
type Disk struct{}

func NewDisk() *Disk { return &Disk{} }

func (*Disk) Usage() (entity.DiskUsage, error) {
	var st unix.Statfs_t
	if err := unix.Statfs("/", &st); err != nil {
		return entity.DiskUsage{}, fmt.Errorf("statfs /: %w", err)
	}
	bsize := uint64(st.Bsize)
	total := st.Blocks * bsize
	avail := st.Bavail * bsize
	return entity.DiskUsage{
		Total:     total,
		Used:      total - st.Bfree*bsize,
		Available: avail,
	}, nil
}

func (*Disk) IOTotals() (read, write uint64, err error) {
	var r, w C.uint64_t
	if C.pulse_disk_io(&r, &w) != 0 {
		return 0, 0, fmt.Errorf("IOBlockStorageDriver statistics unavailable")
	}
	return uint64(r), uint64(w), nil
}
