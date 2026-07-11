//go:build darwin

package sensors

/*
#cgo LDFLAGS: -framework CoreFoundation -framework IOKit -lIOReport
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/IOKitLib.h>

// IOReport is a private but stable API (libIOReport.tbd ships in the SDK);
// this is how mactop and socpowerbud read power. There are no public headers.
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
extern int32_t IOReportStateGetCount(CFDictionaryRef channel);
extern uint64_t IOReportStateGetResidency(CFDictionaryRef channel, int32_t index);
extern CFStringRef IOReportStateGetNameForIndex(CFDictionaryRef channel, int32_t index);

typedef struct {
	char      name[64];
	char      unit[16];
	long long value;
} pulse_energy;

#define PULSE_MAX_STATES 40

typedef struct {
	char               name[64];
	int                nstates;
	unsigned long long residency[PULSE_MAX_STATES];
	char               statenames[PULSE_MAX_STATES][16];
} pulse_states;

static IOReportSubscriptionRef pulse_ioreport_open(const char *group, const char *subgroup,
                                                   CFMutableDictionaryRef *subbed) {
	CFStringRef g = CFStringCreateWithCString(kCFAllocatorDefault, group, kCFStringEncodingUTF8);
	CFStringRef sg = subgroup
	    ? CFStringCreateWithCString(kCFAllocatorDefault, subgroup, kCFStringEncodingUTF8)
	    : NULL;
	CFDictionaryRef chans = IOReportCopyChannelsInGroup(g, sg, 0, 0, 0);
	CFRelease(g);
	if (sg) CFRelease(sg);
	if (!chans)
		return NULL;
	IOReportSubscriptionRef sub =
	    IOReportCreateSubscription(NULL, (CFMutableDictionaryRef)chans, subbed, 0, NULL);
	CFRelease(chans);
	return sub;
}

// pulse_ioreport_read_states pulls the state channels (residency per P-state).
static int pulse_ioreport_read_states(IOReportSubscriptionRef sub, CFMutableDictionaryRef subbed,
                                      pulse_states *out, int max) {
	CFDictionaryRef samples = IOReportCreateSamples(sub, subbed, NULL);
	if (!samples)
		return -1;
	int n = 0;
	CFArrayRef arr = CFDictionaryGetValue(samples, CFSTR("IOReportChannels"));
	if (arr) {
		CFIndex cnt = CFArrayGetCount(arr);
		for (CFIndex i = 0; i < cnt && n < max; i++) {
			CFDictionaryRef ch = CFArrayGetValueAtIndex(arr, i);
			int32_t ns = IOReportStateGetCount(ch);
			if (ns <= 0)
				continue;
			if (ns > PULSE_MAX_STATES) ns = PULSE_MAX_STATES;
			out[n].name[0] = 0;
			CFStringRef name = IOReportChannelGetChannelName(ch);
			if (name)
				CFStringGetCString(name, out[n].name, sizeof(out[n].name), kCFStringEncodingUTF8);
			out[n].nstates = ns;
			for (int32_t s = 0; s < ns; s++) {
				out[n].residency[s] = IOReportStateGetResidency(ch, s);
				out[n].statenames[s][0] = 0;
				CFStringRef sn = IOReportStateGetNameForIndex(ch, s);
				if (sn)
					CFStringGetCString(sn, out[n].statenames[s],
					                   sizeof(out[n].statenames[s]), kCFStringEncodingUTF8);
			}
			n++;
		}
	}
	CFRelease(samples);
	return n;
}

// pulse_devicetree_bytes copies a binary property from IODeviceTree (the
// voltage-states*-sram frequency tables live under the pmgr node).
static int pulse_devicetree_bytes(const char *path, const char *prop,
                                  unsigned char *out, int max) {
	io_registry_entry_t e = IORegistryEntryFromPath(kIOMainPortDefault, path);
	if (!e)
		return -1;
	CFStringRef key = CFStringCreateWithCString(kCFAllocatorDefault, prop, kCFStringEncodingUTF8);
	CFTypeRef v = IORegistryEntryCreateCFProperty(e, key, kCFAllocatorDefault, 0);
	CFRelease(key);
	IOObjectRelease(e);
	if (!v)
		return -1;
	int n = -1;
	if (CFGetTypeID(v) == CFDataGetTypeID()) {
		CFIndex len = CFDataGetLength((CFDataRef)v);
		if (len > max) len = max;
		memcpy(out, CFDataGetBytePtr((CFDataRef)v), len);
		n = (int)len;
	}
	CFRelease(v);
	return n;
}

// pulse_ioreport_read pulls the cumulative energy counters per channel.
// Energy Model channels are simple (single integer), so IOReportIterate
// blocks aren't needed — the IOReportChannels array is walked directly.
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
	"sort"
	"strings"
	"time"
	"unsafe"

	"github.com/emgeorrk/pulse/internal/entity"
)

const (
	maxEnergyChannels     = 32
	maxStateChannels      = 16
	nanojoulesPerJoule    = 1e9
	microjoulesPerJoule   = 1e6
	millijoulesPerJoule   = 1e3
	deviceTreeBufferBytes = 1024
	frequencyEntryBytes   = 8
	minimumFrequencyHz    = 1e8
	frequencyUnitScale    = 1000
)

// IOReport returns CPU/GPU/ANE power (Energy Model group) and CPU frequency
// (CPU Stats group, performance states), averaged between calls: channels
// are cumulative counters, value = delta / time.
type IOReport struct {
	sub    C.IOReportSubscriptionRef
	subbed C.CFMutableDictionaryRef
	prev   map[string]int64
	prevT  time.Time

	fsub    C.IOReportSubscriptionRef
	fsubbed C.CFMutableDictionaryRef
	prevRes map[string][]uint64 // per-state residency per cluster
	tables  [][]float64         // frequency tables from the device tree, Hz (keyed by state count)
}

func NewIOReport() (*IOReport, error) {
	r := &IOReport{}
	r.sub = C.pulse_ioreport_open(C.CString("Energy Model"), nil, &r.subbed) //nolint:gocritic // cgo pointers confuse dupSubExpr.
	if r.sub == nil {
		return nil, errIOReport
	}
	// first read establishes the baseline
	if _, err := r.Power(); err != nil { //nolint:gocritic // The inline error is checked immediately.
		return nil, err
	}

	// frequency is best effort: if there are no perf-state channels or
	// frequency tables in the device tree, we just live without it
	r.tables = readFreqTables()
	if len(r.tables) > 0 {
		r.fsub = C.pulse_ioreport_open(C.CString("CPU Stats"),
			C.CString("CPU Complex Performance States"), &r.fsubbed) //nolint:gocritic // cgo pointers confuse dupSubExpr.
		if r.fsub != nil {
			r.prevRes = map[string][]uint64{}
			r.primeFrequency()
		}
	}

	return r, nil
}

func (r *IOReport) primeFrequency() {
	if _, err := r.Frequency(); err != nil { //nolint:gocritic // The baseline error is intentionally consumed.
		return // The first call only establishes residency baselines.
	}
}

// FreqClusters returns the cluster channel names frequency is computed
// from (known after the first sample).
func (r *IOReport) FreqClusters() []string {
	names := make([]string, 0, len(r.prevRes))
	for name := range r.prevRes {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

// HasFreq reports whether CPU frequency is available on this hardware.
func (r *IOReport) HasFreq() bool { return r.fsub != nil }

func (r *IOReport) Power() (entity.PowerStats, error) { //nolint:cyclop,gocyclo // Channel names require an explicit classification table.
	var buf [maxEnergyChannels]C.pulse_energy
	n := int(C.pulse_ioreport_read(r.sub, r.subbed, &buf[0], maxEnergyChannels))
	if n < 0 {
		return entity.PowerStats{}, errIOReportSample
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

		// PULSE_DEBUG=1 pulse -once — inspect channel names on a new chip
		if os.Getenv("PULSE_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "ioreport: %q [%s] = %d\n", name, unit, val)
		}

		prev, ok := r.prev[name]
		if !ok || dt <= 0 || val < prev {
			continue // first call, or the counter was reset
		}
		watts := float64(val-prev) / joulesDivisor(unit) / dt

		// Channel names depend on the chip generation: M1 uses "CPU
		// Energy"/"GPU Energy"/"ANE Energy"; M5 Pro uses PACC_x (P-cores),
		// MCPU0_x (E-cores), MCPM/PCPM (cluster power managers), and has no
		// GPU/ANE channels.
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

// joulesDivisor converts a channel's unit into a divisor down to joules.
func joulesDivisor(unit string) float64 {
	switch strings.TrimSpace(unit) {
	case "nJ":
		return nanojoulesPerJoule
	case "uJ", "µJ":
		return microjoulesPerJoule
	case "mJ":
		return millijoulesPerJoule
	default:
		return 1 // J
	}
}

// Frequency returns the residency-weighted frequency per cluster channel
// (MCPU0/MCPU1/PCPU on M5 Pro, ECPU/PCPU on M1) since the last call.
// The frequency table is chosen by the channel's number of active states.
func (r *IOReport) Frequency() (entity.FreqStats, error) { //nolint:cyclop,gocognit,gocyclo // Residency parsing mirrors IOReport's nested channel/state format.
	if r.fsub == nil {
		return entity.FreqStats{}, errPerfStates
	}
	var buf [maxStateChannels]C.pulse_states
	n := int(C.pulse_ioreport_read_states(r.fsub, r.fsubbed, &buf[0], maxStateChannels))
	if n < 0 {
		return entity.FreqStats{}, errPerfStatesSample
	}

	var stats entity.FreqStats
	for i := 0; i < n; i++ {
		name := C.GoString(&buf[i].name[0])
		// CPM channels (cluster power managers) and _IDLE mirrors duplicate
		// the CPU channels — skip them
		if strings.Contains(name, "CPM") || strings.HasSuffix(name, "_IDLE") {
			continue
		}
		ns := int(buf[i].nstates)

		cur := make([]uint64, ns)
		for s := 0; s < ns; s++ {
			cur[s] = uint64(buf[i].residency[s])
		}
		prev, ok := r.prevRes[name]
		r.prevRes[name] = cur
		if !ok || len(prev) != ns {
			continue
		}

		// active states are everything except IDLE/DOWN/OFF; their order
		// matches the order of the device-tree frequency table
		var deltas []uint64
		for s := 0; s < ns; s++ {
			st := strings.ToUpper(C.GoString(&buf[i].statenames[s][0]))
			if st == "IDLE" || st == "DOWN" || st == "OFF" {
				continue
			}
			deltas = append(deltas, cur[s]-prev[s])
		}

		if os.Getenv("PULSE_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "perfstates: %q active=%d table=%d\n",
				name, len(deltas), len(r.tableFor(len(deltas))))
		}

		hz := weightedFreq(deltas, r.tableFor(len(deltas)))
		if hz <= 0 {
			continue
		}
		stats.Clusters = append(stats.Clusters, entity.Reading{Name: name, Value: hz})
		if hz > stats.Max {
			stats.Max = hz
		}
	}
	if len(stats.Clusters) == 0 {
		return stats, errClusterResidency
	}

	return stats, nil
}

// tableFor picks the frequency table whose entry count matches the
// channel's active state count (so PCPU with 20 states finds
// voltage-states5-sram with 20 entries, MCPU with 15 finds
// voltage-states22-sram).
func (r *IOReport) tableFor(states int) []float64 {
	for _, t := range r.tables {
		if len(t) == states {
			return t
		}
	}
	return nil
}

// weightedFreq computes the weighted-average frequency: active-state
// residency deltas × state frequencies from the device-tree table.
func weightedFreq(deltas []uint64, table []float64) float64 {
	n := len(deltas)
	if len(table) < n {
		n = len(table)
	}
	var sum, weight float64
	for i := 0; i < n; i++ {
		sum += float64(deltas[i]) * table[i]
		weight += float64(deltas[i])
	}
	if weight == 0 {
		return 0
	}
	return sum / weight
}

// readFreqTables collects CPU cluster frequency tables from IODeviceTree
// (pmgr node, voltage-statesN-sram properties): little-endian uint32 pairs
// (frequency, voltage). Property names depend on the chip generation (M1:
// states1/5, M5 Pro: states5/22/23), so we scan every N.
func readFreqTables() [][]float64 {
	var tables [][]float64
	for n := 0; n < 32; n++ {
		t := readFreqTable(fmt.Sprintf("voltage-states%d-sram", n))
		if len(t) > 0 {
			tables = append(tables, t)
		}
	}
	return tables
}

func readFreqTable(prop string) []float64 {
	var buf [deviceTreeBufferBytes]C.uchar
	cpath := C.CString("IODeviceTree:/arm-io/pmgr")
	cprop := C.CString(prop)
	n := int(C.pulse_devicetree_bytes(cpath, cprop, &buf[0], deviceTreeBufferBytes))
	C.free(unsafe.Pointer(cpath))
	C.free(unsafe.Pointer(cprop))
	if n < frequencyEntryBytes {
		return nil
	}
	var freqs []float64
	for off := 0; off+frequencyEntryBytes <= n; off += frequencyEntryBytes {
		hz := float64(uint32(buf[off]) | uint32(buf[off+1])<<8 |
			uint32(buf[off+2])<<16 | uint32(buf[off+3])<<24)
		if hz <= 1 { // placeholder 1s in some tables aren't real frequencies
			continue
		}
		// units depend on the chip: Hz (M1) or kHz (M5) — normalize to Hz
		for hz < minimumFrequencyHz {
			hz *= frequencyUnitScale
		}
		freqs = append(freqs, hz)
	}
	return freqs
}
