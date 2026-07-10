//go:build darwin

package sensors

/*
#cgo LDFLAGS: -framework CoreFoundation -lIOReport
#include <stdint.h>
#include <CoreFoundation/CoreFoundation.h>

// IOReport — приватный, но стабильный API (libIOReport.tbd есть в SDK);
// так мощность читают mactop и socpowerbud. Публичных хедеров нет.
typedef struct IOReportSubscription *IOReportSubscriptionRef;

extern CFDictionaryRef IOReportCopyChannelsInGroup(CFStringRef group, CFStringRef subgroup,
                                                   uint64_t a, uint64_t b, uint64_t c);
extern IOReportSubscriptionRef IOReportCreateSubscription(void *allocator,
        CFMutableDictionaryRef channels, CFMutableDictionaryRef *subbed,
        uint64_t channel_id, CFTypeRef options);
extern CFDictionaryRef IOReportCreateSamples(IOReportSubscriptionRef sub,
        CFMutableDictionaryRef subbed, CFTypeRef options);
extern CFStringRef IOReportChannelGetChannelName(CFDictionaryRef channel);
extern CFStringRef IOReportChannelGetUnitLabel(CFDictionaryRef channel);
extern int64_t IOReportSimpleGetIntegerValue(CFDictionaryRef channel, int32_t index);

typedef struct {
	char      name[64];
	char      unit[16];
	long long value;
} pulse_energy;

static IOReportSubscriptionRef pulse_ioreport_open(CFMutableDictionaryRef *subbed) {
	CFDictionaryRef chans = IOReportCopyChannelsInGroup(CFSTR("Energy Model"), NULL, 0, 0, 0);
	if (!chans)
		return NULL;
	IOReportSubscriptionRef sub =
	    IOReportCreateSubscription(NULL, (CFMutableDictionaryRef)chans, subbed, 0, NULL);
	CFRelease(chans);
	return sub;
}

// pulse_ioreport_read выгребает накопительные счётчики энергии по каналам.
// Каналы Energy Model простые (single integer), блоки IOReportIterate не
// нужны — массив IOReportChannels обходится напрямую.
static int pulse_ioreport_read(IOReportSubscriptionRef sub, CFMutableDictionaryRef subbed,
                               pulse_energy *out, int max) {
	CFDictionaryRef samples = IOReportCreateSamples(sub, subbed, NULL);
	if (!samples)
		return -1;
	int n = 0;
	CFArrayRef arr = CFDictionaryGetValue(samples, CFSTR("IOReportChannels"));
	if (arr) {
		CFIndex cnt = CFArrayGetCount(arr);
		for (CFIndex i = 0; i < cnt && n < max; i++) {
			CFDictionaryRef ch = CFArrayGetValueAtIndex(arr, i);
			out[n].name[0] = 0;
			out[n].unit[0] = 0;
			CFStringRef name = IOReportChannelGetChannelName(ch);
			CFStringRef unit = IOReportChannelGetUnitLabel(ch);
			if (name)
				CFStringGetCString(name, out[n].name, sizeof(out[n].name), kCFStringEncodingUTF8);
			if (unit)
				CFStringGetCString(unit, out[n].unit, sizeof(out[n].unit), kCFStringEncodingUTF8);
			out[n].value = IOReportSimpleGetIntegerValue(ch, 0);
			n++;
		}
	}
	CFRelease(samples);
	return n;
}
*/
import "C"

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/emgeorrk/pulse/internal/entity"
)

const maxEnergyChannels = 32

// IOReport отдаёт мощность CPU/GPU/ANE, усреднённую между вызовами Power():
// каналы Energy Model — накопительные счётчики энергии (nJ/uJ/mJ),
// ватты = дельта энергии / время.
type IOReport struct {
	sub    C.IOReportSubscriptionRef
	subbed C.CFMutableDictionaryRef
	prev   map[string]int64
	prevT  time.Time
}

func NewIOReport() (*IOReport, error) {
	r := &IOReport{}
	r.sub = C.pulse_ioreport_open(&r.subbed)
	if r.sub == nil {
		return nil, fmt.Errorf("IOReport Energy Model unavailable")
	}
	// первое чтение — точка отсчёта
	if _, err := r.Power(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *IOReport) Power() (entity.PowerStats, error) {
	var buf [maxEnergyChannels]C.pulse_energy
	n := int(C.pulse_ioreport_read(r.sub, r.subbed, &buf[0], maxEnergyChannels))
	if n < 0 {
		return entity.PowerStats{}, fmt.Errorf("IOReport sampling failed")
	}

	now := time.Now()
	dt := now.Sub(r.prevT).Seconds()
	cur := make(map[string]int64, n)

	var stats entity.PowerStats
	for i := 0; i < n; i++ {
		name := C.GoString(&buf[i].name[0])
		unit := C.GoString(&buf[i].unit[0])
		val := int64(buf[i].value)
		cur[name] = val

		// PULSE_DEBUG=1 pulse -once — посмотреть имена каналов на новом чипе
		if os.Getenv("PULSE_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "ioreport: %q [%s] = %d\n", name, unit, val)
		}

		prev, ok := r.prev[name]
		if !ok || dt <= 0 || val < prev {
			continue // первый вызов или сброс счётчика
		}
		watts := float64(val-prev) / joulesDivisor(unit) / dt

		// Имена каналов зависят от поколения чипа: M1 — "CPU Energy"/"GPU
		// Energy"/"ANE Energy"; M5 Pro — PACC_x (P-ядра), MCPU0_x (E-ядра),
		// MCPM/PCPM (кластерные power-менеджеры), GPU/ANE-каналов нет.
		lower := strings.ToLower(name)
		switch {
		case strings.Contains(lower, "gpu") || strings.Contains(lower, "gfx"):
			stats.GPU += watts
		case strings.Contains(lower, "ane"):
			stats.ANE += watts
		case strings.Contains(lower, "cpu") || strings.Contains(lower, "acc") ||
			strings.Contains(lower, "cpm"):
			stats.CPU += watts
		}
		stats.Total += watts
	}

	r.prev = cur
	r.prevT = now
	return stats, nil
}

// joulesDivisor переводит единицу канала в делитель до джоулей.
func joulesDivisor(unit string) float64 {
	switch strings.TrimSpace(unit) {
	case "nJ":
		return 1e9
	case "uJ", "µJ":
		return 1e6
	case "mJ":
		return 1e3
	default:
		return 1 // J
	}
}
