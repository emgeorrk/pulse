//go:build darwin

package sensors

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>

// IOHIDEventSystemClient is a private but stable IOKit API: this is exactly
// how mactop, iSMC, and exelban/stats read Apple Silicon sensors. There are
// no public headers — we declare them ourselves.
typedef struct __IOHIDEventSystemClient *IOHIDEventSystemClientRef;
typedef struct __IOHIDServiceClient *IOHIDServiceClientRef;
typedef struct __IOHIDEvent *IOHIDEventRef;

IOHIDEventSystemClientRef IOHIDEventSystemClientCreate(CFAllocatorRef);
int IOHIDEventSystemClientSetMatching(IOHIDEventSystemClientRef, CFDictionaryRef);
CFArrayRef IOHIDEventSystemClientCopyServices(IOHIDEventSystemClientRef);
CFTypeRef IOHIDServiceClientCopyProperty(IOHIDServiceClientRef, CFStringRef);
IOHIDEventRef IOHIDServiceClientCopyEvent(IOHIDServiceClientRef, int64_t, int32_t, int64_t);
double IOHIDEventGetFloatValue(IOHIDEventRef, int32_t);

#define kIOHIDEventTypeTemperature 15
#define IOHIDEventFieldBase(type)  ((type) << 16)

// Apple sensor pages/usages (AppleHIDUsageTables):
//   0xff00/5 — temperature sensors
static IOHIDEventSystemClientRef pulse_hid_client(int page, int usage) {
	IOHIDEventSystemClientRef client = IOHIDEventSystemClientCreate(kCFAllocatorDefault);
	if (!client)
		return NULL;
	CFNumberRef p = CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, &page);
	CFNumberRef u = CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, &usage);
	CFStringRef keys[2] = { CFSTR("PrimaryUsagePage"), CFSTR("PrimaryUsage") };
	CFTypeRef  vals[2] = { p, u };
	CFDictionaryRef match = CFDictionaryCreate(kCFAllocatorDefault,
	    (const void **)keys, (const void **)vals, 2,
	    &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFRelease(p);
	CFRelease(u);
	IOHIDEventSystemClientSetMatching(client, match);
	CFRelease(match);
	return client;
}

// pulse_hid_read reads one service's name and value; -1 means no event.
static int pulse_hid_read(IOHIDServiceClientRef svc, int64_t eventType,
                          double *val, char *name, int nameLen) {
	name[0] = 0;
	CFStringRef prod = (CFStringRef)IOHIDServiceClientCopyProperty(svc, CFSTR("Product"));
	if (prod) {
		CFStringGetCString(prod, name, nameLen, kCFStringEncodingUTF8);
		CFRelease(prod);
	}
	IOHIDEventRef ev = IOHIDServiceClientCopyEvent(svc, eventType, 0, 0);
	if (!ev)
		return -1;
	*val = IOHIDEventGetFloatValue(ev, IOHIDEventFieldBase((int32_t)eventType));
	CFRelease(ev);
	return 0;
}
*/
import "C"

import (
	"sort"
	"strings"

	"github.com/emgeorrk/pulse/internal/entity"
)

const (
	temperatureUsagePage = 0xff00
	temperatureUsage     = 5
)

// HID reads temperatures from the HID sensor hub — the Apple Silicon path.
// The client is created once: the service list is stable for the session.
type HID struct {
	temp C.IOHIDEventSystemClientRef
}

func NewHID() (*HID, error) {
	h := &HID{
		temp: C.pulse_hid_client(temperatureUsagePage, temperatureUsage),
	}
	if h.temp == nil {
		return nil, errHIDUnavailable
	}

	return h, nil
}

// Temps returns temperature sensors with valid values (0 < t ≤ 125 °C).
// Calibration channels (tcal) aren't temperatures and are discarded;
// multiple channels for the same sensor are averaged into one reading.
func (h *HID) Temps() ([]entity.Reading, error) {
	return h.read(h.temp, C.kIOHIDEventTypeTemperature, func(name string, v float64) bool {
		return v > 0 && v <= 125 && !strings.Contains(strings.ToLower(name), "tcal")
	})
}

func (h *HID) read(client C.IOHIDEventSystemClientRef, eventType int64, valid func(string, float64) bool) ([]entity.Reading, error) {
	if client == nil {
		return nil, errHIDClient
	}

	// CF types are uintptr in cgo (pointer-invariance rules), so compare to 0
	services := C.IOHIDEventSystemClientCopyServices(client)
	if services == 0 {
		return nil, errHIDServices
	}

	defer C.CFRelease(C.CFTypeRef(services))

	n := int(C.CFArrayGetCount(services))
	sum := map[string]float64{}
	cnt := map[string]int{}
	for i := 0; i < n; i++ {
		svc := C.IOHIDServiceClientRef(C.CFArrayGetValueAtIndex(services, C.CFIndex(i)))
		var (
			val  C.double
			name [128]C.char
		)
		if C.pulse_hid_read(svc, C.int64_t(eventType), &val, &name[0], 128) != 0 {
			continue
		}
		v := float64(val)
		label := C.GoString(&name[0])
		if label == "" || !valid(label, v) {
			continue
		}
		sum[label] += v
		cnt[label]++
	}
	if len(sum) == 0 {
		return nil, errHIDSensors
	}
	out := make([]entity.Reading, 0, len(sum))
	for name, s := range sum {
		out = append(out, entity.Reading{Name: name, Value: s / float64(cnt[name])})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	return out, nil
}
