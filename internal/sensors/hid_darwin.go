//go:build darwin

package sensors

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>

// IOHIDEventSystemClient — приватный, но стабильный API IOKit: именно так
// читают сенсоры Apple Silicon mactop, iSMC и exelban/stats. Публичных
// хедеров нет — декларируем сами.
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
#define kIOHIDEventTypePower       25
#define IOHIDEventFieldBase(type)  ((type) << 16)

// Страницы/usage сенсоров Apple (AppleHIDUsageTables):
//   0xff00/5 — температурные сенсоры
//   0xff08/3 — напряжение, 0xff08/2 — ток
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

// pulse_hid_read читает имя и значение одного сервиса; -1 = события нет.
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
	"fmt"
	"sort"
	"strings"

	"github.com/emgeorrk/pulse/internal/entity"
)

// HID читает температуры и напряжения с HID sensor hub — путь Apple Silicon.
// Клиенты создаются один раз: список сервисов стабилен в рамках сессии.
type HID struct {
	temp C.IOHIDEventSystemClientRef
	volt C.IOHIDEventSystemClientRef
}

func NewHID() (*HID, error) {
	h := &HID{
		temp: C.pulse_hid_client(0xff00, 5),
		volt: C.pulse_hid_client(0xff08, 3),
	}
	if h.temp == nil && h.volt == nil {
		return nil, fmt.Errorf("HID event system unavailable")
	}
	return h, nil
}

// Temps — температурные сенсоры с валидными значениями (0 < t ≤ 125 °C).
// Калибровочные каналы (tcal) — не температуры, отбрасываем; несколько
// каналов одного сенсора усредняются в одно показание.
func (h *HID) Temps() ([]entity.Reading, error) {
	return h.read(h.temp, C.kIOHIDEventTypeTemperature, func(name string, v float64) bool {
		return v > 0 && v <= 125 && !strings.Contains(strings.ToLower(name), "tcal")
	})
}

// Voltages — сенсоры напряжения (0.01 ≤ v ≤ 100 В; спящие LDO с нулём не
// показываем).
func (h *HID) Voltages() ([]entity.Reading, error) {
	return h.read(h.volt, C.kIOHIDEventTypePower, func(_ string, v float64) bool {
		return v >= 0.01 && v <= 100
	})
}

func (h *HID) read(client C.IOHIDEventSystemClientRef, eventType int64, valid func(string, float64) bool) ([]entity.Reading, error) {
	if client == nil {
		return nil, fmt.Errorf("HID client not created")
	}
	// CF-типы в cgo — uintptr (правила pointer-invariance), сравниваем с 0
	services := C.IOHIDEventSystemClientCopyServices(client)
	if services == 0 {
		return nil, fmt.Errorf("no HID services")
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
		return nil, fmt.Errorf("no readable HID sensors")
	}
	out := make([]entity.Reading, 0, len(sum))
	for name, s := range sum {
		out = append(out, entity.Reading{Name: name, Value: s / float64(cnt[name])})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
