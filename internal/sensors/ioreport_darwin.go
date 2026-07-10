//go:build darwin

package sensors

/*
#cgo LDFLAGS: -framework CoreFoundation -framework IOKit -lIOReport
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/IOKitLib.h>

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

// pulse_ioreport_read_states выгребает state-каналы (residency по P-стейтам).
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

// pulse_devicetree_bytes копирует бинарное свойство из IODeviceTree
// (таблицы частот voltage-states*-sram лежат в узле pmgr).
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
	"sort"
	"strings"
	"time"
	"unsafe"

	"github.com/emgeorrk/pulse/internal/entity"
)

const (
	maxEnergyChannels = 32
	maxStateChannels  = 16
)

// IOReport отдаёт мощность CPU/GPU/ANE (группа Energy Model) и частоту CPU
// (группа CPU Stats, performance states), усреднённые между вызовами:
// каналы — накопительные счётчики, значение = дельта / время.
type IOReport struct {
	sub    C.IOReportSubscriptionRef
	subbed C.CFMutableDictionaryRef
	prev   map[string]int64
	prevT  time.Time

	fsub    C.IOReportSubscriptionRef
	fsubbed C.CFMutableDictionaryRef
	prevRes map[string][]uint64 // residency по стейтам на кластер
	tables  [][]float64         // таблицы частот из device tree, Гц (по числу стейтов)
}

func NewIOReport() (*IOReport, error) {
	r := &IOReport{}
	r.sub = C.pulse_ioreport_open(C.CString("Energy Model"), nil, &r.subbed)
	if r.sub == nil {
		return nil, fmt.Errorf("IOReport Energy Model unavailable")
	}
	// первое чтение — точка отсчёта
	if _, err := r.Power(); err != nil {
		return nil, err
	}

	// частота — best effort: нет perf-state каналов или таблиц частот в
	// device tree — просто живём без неё
	r.tables = readFreqTables()
	if len(r.tables) > 0 {
		r.fsub = C.pulse_ioreport_open(C.CString("CPU Stats"),
			C.CString("CPU Complex Performance States"), &r.fsubbed)
		if r.fsub != nil {
			r.prevRes = map[string][]uint64{}
			r.Frequency() // точка отсчёта
		}
	}
	return r, nil
}

// FreqClusters — имена кластерных каналов, по которым считается частота
// (известны после первого сэмпла).
func (r *IOReport) FreqClusters() []string {
	names := make([]string, 0, len(r.prevRes))
	for name := range r.prevRes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// HasFreq — доступна ли частота CPU на этом железе.
func (r *IOReport) HasFreq() bool { return r.fsub != nil }

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

// Frequency — средневзвешенная по residency частота на кластерный канал
// (MCPU0/MCPU1/PCPU на M5 Pro, ECPU/PCPU на M1) с прошлого вызова.
// Таблица частот подбирается по числу активных стейтов канала.
func (r *IOReport) Frequency() (entity.FreqStats, error) {
	if r.fsub == nil {
		return entity.FreqStats{}, fmt.Errorf("perf states unavailable")
	}
	var buf [maxStateChannels]C.pulse_states
	n := int(C.pulse_ioreport_read_states(r.fsub, r.fsubbed, &buf[0], maxStateChannels))
	if n < 0 {
		return entity.FreqStats{}, fmt.Errorf("perf states sampling failed")
	}

	var stats entity.FreqStats
	for i := 0; i < n; i++ {
		name := C.GoString(&buf[i].name[0])
		// CPM-каналы (кластерные power-менеджеры) и _IDLE-зеркала дублируют
		// картину CPU-каналов — пропускаем
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

		// активные стейты — всё, кроме IDLE/DOWN/OFF; их порядок совпадает
		// с порядком таблицы частот из device tree
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
		return stats, fmt.Errorf("no active cluster residency yet")
	}
	return stats, nil
}

// tableFor подбирает таблицу частот с числом записей, равным числу активных
// стейтов канала (так PCPU с 20 стейтами находит voltage-states5-sram с 20
// записями, MCPU с 15 — voltage-states22-sram).
func (r *IOReport) tableFor(states int) []float64 {
	for _, t := range r.tables {
		if len(t) == states {
			return t
		}
	}
	return nil
}

// weightedFreq — средневзвешенная частота: residency-дельты активных
// стейтов × частоты стейтов из таблицы device tree.
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

// readFreqTables собирает таблицы частот CPU-кластеров из IODeviceTree
// (узел pmgr, свойства voltage-statesN-sram): little-endian пары uint32
// (частота, напряжение). Имена свойств зависят от поколения чипа (M1:
// states1/5, M5 Pro: states5/22/23), поэтому перебираем все N.
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
	var buf [1024]C.uchar
	cpath := C.CString("IODeviceTree:/arm-io/pmgr")
	cprop := C.CString(prop)
	n := int(C.pulse_devicetree_bytes(cpath, cprop, &buf[0], 1024))
	C.free(unsafe.Pointer(cpath))
	C.free(unsafe.Pointer(cprop))
	if n < 8 {
		return nil
	}
	var freqs []float64
	for off := 0; off+8 <= n; off += 8 {
		hz := float64(uint32(buf[off]) | uint32(buf[off+1])<<8 |
			uint32(buf[off+2])<<16 | uint32(buf[off+3])<<24)
		if hz <= 1 { // заглушки-единицы в некоторых таблицах — не частоты
			continue
		}
		// единицы зависят от чипа: Гц (M1) или кГц (M5) — нормализуем в Гц
		for hz < 1e8 {
			hz *= 1000
		}
		freqs = append(freqs, hz)
	}
	return freqs
}
