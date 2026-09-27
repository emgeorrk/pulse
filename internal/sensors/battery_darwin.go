//go:build darwin

package sensors

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <stdint.h>
#include <IOKit/IOKitLib.h>
#include <CoreFoundation/CoreFoundation.h>

typedef struct {
	long long rawCurrent, rawMax, designCapacity;
	long long currentCapacity, maxCapacity;
	long long cycleCount, temperature, voltage, amperage, timeRemaining;
	int       isCharging, externalConnected;
} pulse_batt;

enum { PULSE_BATT_MAX_DICTS = 3 };

typedef struct {
	CFDictionaryRef d[PULSE_BATT_MAX_DICTS];
	int             n;
} pulse_batt_dicts;

static void pulse_batt_dicts_add(pulse_batt_dicts *ds, CFTypeRef d) {
	if (d && CFGetTypeID(d) == CFDictionaryGetTypeID() && ds->n < PULSE_BATT_MAX_DICTS)
		ds->d[ds->n++] = (CFDictionaryRef)d;
}

// pulse_dict_num returns the key from the first dictionary that has it as a
// number, so the earlier (older-layout) dictionary wins; 0 when none does.
static long long pulse_dict_num(const pulse_batt_dicts *ds, CFStringRef key) {
	long long v = 0;
	for (int i = 0; i < ds->n; i++) {
		CFTypeRef n = CFDictionaryGetValue(ds->d[i], key);
		if (n && CFGetTypeID(n) == CFNumberGetTypeID()) {
			CFNumberGetValue((CFNumberRef)n, kCFNumberSInt64Type, &v);
			break;
		}
	}
	return v;
}

// pulse_pack_data copies the BatteryData dictionary of the battery's first
// AppleSmartBatteryPack child, or returns NULL. The caller releases it.
static CFTypeRef pulse_pack_data(io_service_t svc) {
	io_iterator_t it;
	if (IORegistryEntryGetChildIterator(svc, kIOServicePlane, &it) != KERN_SUCCESS)
		return NULL;
	CFTypeRef data = NULL;
	io_object_t child;
	while (!data && (child = IOIteratorNext(it))) {
		if (IOObjectConformsTo(child, "AppleSmartBatteryPack"))
			data = IORegistryEntryCreateCFProperty(child, CFSTR("BatteryData"),
			                                       kCFAllocatorDefault, kNilOptions);
		IOObjectRelease(child);
	}
	IOObjectRelease(it);
	return data;
}

static int pulse_battery_read(pulse_batt *b) {
	io_service_t svc = IOServiceGetMatchingService(kIOMainPortDefault,
	                                               IOServiceMatching("AppleSmartBattery"));
	if (!svc)
		return -1;
	CFMutableDictionaryRef props = NULL;
	if (IORegistryEntryCreateCFProperties(svc, &props, kCFAllocatorDefault,
	        kNilOptions) != KERN_SUCCESS || !props) {
		IOObjectRelease(svc);
		return -1;
	}

	// macOS 27 dropped the raw capacities, DesignCapacity and Temperature from
	// the top-level properties: DesignCapacity survives in the service's own
	// BatteryData, the rest only in the AppleSmartBatteryPack child's
	// BatteryData. Search the pre-27 layout first.
	CFTypeRef pack = pulse_pack_data(svc);
	pulse_batt_dicts ds = {0};
	pulse_batt_dicts_add(&ds, props);
	pulse_batt_dicts_add(&ds, CFDictionaryGetValue(props, CFSTR("BatteryData")));
	pulse_batt_dicts_add(&ds, pack);

	b->rawCurrent      = pulse_dict_num(&ds, CFSTR("AppleRawCurrentCapacity"));
	b->rawMax          = pulse_dict_num(&ds, CFSTR("AppleRawMaxCapacity"));
	b->designCapacity  = pulse_dict_num(&ds, CFSTR("DesignCapacity"));
	b->currentCapacity = pulse_dict_num(&ds, CFSTR("CurrentCapacity"));
	b->maxCapacity     = pulse_dict_num(&ds, CFSTR("MaxCapacity"));
	b->cycleCount      = pulse_dict_num(&ds, CFSTR("CycleCount"));
	b->temperature     = pulse_dict_num(&ds, CFSTR("Temperature"));
	b->voltage         = pulse_dict_num(&ds, CFSTR("Voltage"));
	b->amperage        = pulse_dict_num(&ds, CFSTR("Amperage"));
	b->timeRemaining   = pulse_dict_num(&ds, CFSTR("TimeRemaining"));
	b->isCharging        = CFDictionaryGetValue(props, CFSTR("IsCharging")) == kCFBooleanTrue;
	b->externalConnected = CFDictionaryGetValue(props, CFSTR("ExternalConnected")) == kCFBooleanTrue;

	if (pack)
		CFRelease(pack);
	CFRelease(props);
	IOObjectRelease(svc);
	return 0;
}
*/
import "C"

import "github.com/emgeorrk/pulse/internal/entity"

const (
	percentDivisor    = 100
	milliUnitDivisor  = 1000
	unknownTimeMarker = 0xFFFF
)

// Batt reads AppleSmartBattery from IORegistry (plus its AppleSmartBatteryPack
// child, where macOS 27 moved the raw capacity and temperature keys). Desktop
// Macs have no such service — probe will disable the group.
type Batt struct{}

func NewBattery() *Batt { return &Batt{} }

func (*Batt) Battery() (entity.BatteryStats, error) {
	var b C.pulse_batt
	if C.pulse_battery_read(&b) != 0 {
		return entity.BatteryStats{}, errBatteryUnavailable
	}

	st := entity.BatteryStats{
		Cycles:   int(b.cycleCount),
		TempC:    float64(b.temperature) / percentDivisor, // hundredths of °C
		Volts:    float64(b.voltage) / milliUnitDivisor,   // mV
		Charging: b.isCharging != 0,
		External: b.externalConnected != 0,
	}

	// Percent is what the macOS menu bar shows: the fuel gauge's smoothed
	// state of charge (CurrentCapacity, 0–100 on Apple Silicon; mAh on Intel,
	// where the ratio is still a valid fraction). RawPercent is the plain mAh
	// ratio — the gauge holds back reserve capacity and smooths near full, so
	// it trails Percent by 1–3%. Each pair of keys is optional; fall back to
	// the other.
	switch {
	case b.maxCapacity > 0:
		st.Percent = float64(b.currentCapacity) / float64(b.maxCapacity)
	case b.rawMax > 0:
		st.Percent = float64(b.rawCurrent) / float64(b.rawMax)
	}

	switch {
	case b.rawMax > 0:
		st.RawPercent = float64(b.rawCurrent) / float64(b.rawMax)
	case b.maxCapacity > 0:
		st.RawPercent = float64(b.currentCapacity) / float64(b.maxCapacity)
	}
	if b.designCapacity > 0 && b.rawMax > 0 {
		st.Health = float64(b.rawMax) / float64(b.designCapacity)
	}

	// mV × mA → W; Amperage is negative while discharging
	st.Watts = st.Volts * float64(b.amperage) / milliUnitDivisor

	st.MinutesLeft = int(b.timeRemaining)
	if b.timeRemaining <= 0 || b.timeRemaining >= unknownTimeMarker { // 65535 = still being calculated
		st.MinutesLeft = -1
	}

	return st, nil
}
