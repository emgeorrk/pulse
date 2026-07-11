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

static long long pulse_dict_num(CFDictionaryRef d, CFStringRef key) {
	CFNumberRef n = CFDictionaryGetValue(d, key);
	long long v = 0;
	if (n) CFNumberGetValue(n, kCFNumberSInt64Type, &v);
	return v;
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

	b->rawCurrent      = pulse_dict_num(props, CFSTR("AppleRawCurrentCapacity"));
	b->rawMax          = pulse_dict_num(props, CFSTR("AppleRawMaxCapacity"));
	b->designCapacity  = pulse_dict_num(props, CFSTR("DesignCapacity"));
	b->currentCapacity = pulse_dict_num(props, CFSTR("CurrentCapacity"));
	b->maxCapacity     = pulse_dict_num(props, CFSTR("MaxCapacity"));
	b->cycleCount      = pulse_dict_num(props, CFSTR("CycleCount"));
	b->temperature     = pulse_dict_num(props, CFSTR("Temperature"));
	b->voltage         = pulse_dict_num(props, CFSTR("Voltage"));
	b->amperage        = pulse_dict_num(props, CFSTR("Amperage"));
	b->timeRemaining   = pulse_dict_num(props, CFSTR("TimeRemaining"));
	b->isCharging        = CFDictionaryGetValue(props, CFSTR("IsCharging")) == kCFBooleanTrue;
	b->externalConnected = CFDictionaryGetValue(props, CFSTR("ExternalConnected")) == kCFBooleanTrue;

	CFRelease(props);
	IOObjectRelease(svc);
	return 0;
}
*/
import "C"

import (
	"fmt"

	"github.com/emgeorrk/pulse/internal/entity"
)

// Batt reads AppleSmartBattery from IORegistry. Desktop Macs have no such
// service — probe will disable the group.
type Batt struct{}

func NewBattery() *Batt { return &Batt{} }

func (*Batt) Battery() (entity.BatteryStats, error) {
	var b C.pulse_batt
	if C.pulse_battery_read(&b) != 0 {
		return entity.BatteryStats{}, fmt.Errorf("AppleSmartBattery unavailable")
	}

	st := entity.BatteryStats{
		Cycles:   int(b.cycleCount),
		TempC:    float64(b.temperature) / 100, // hundredths of °C
		Volts:    float64(b.voltage) / 1000,    // mV
		Charging: b.isCharging != 0,
		External: b.externalConnected != 0,
	}

	// Percent: on Apple Silicon CurrentCapacity is already 0–100, on Intel
	// it's mAh; the raw mAh keys work the same everywhere, so prefer them.
	switch {
	case b.rawMax > 0:
		st.Percent = float64(b.rawCurrent) / float64(b.rawMax)
	case b.maxCapacity > 0:
		st.Percent = float64(b.currentCapacity) / float64(b.maxCapacity)
	}
	if b.designCapacity > 0 && b.rawMax > 0 {
		st.Health = float64(b.rawMax) / float64(b.designCapacity)
	}

	// mV × mA → W; Amperage is negative while discharging
	st.Watts = st.Volts * float64(b.amperage) / 1000

	st.MinutesLeft = int(b.timeRemaining)
	if b.timeRemaining <= 0 || b.timeRemaining >= 0xFFFF { // 65535 = still being calculated
		st.MinutesLeft = -1
	}
	return st, nil
}
