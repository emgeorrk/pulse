//go:build darwin

package sensors

/*
#cgo LDFLAGS: -framework IOKit
#include <stdint.h>
#include <string.h>
#include <IOKit/IOKitLib.h>

// AppleSMC protocol structs (references: beltex/SMCKit, dkorunic/iSMC).
// The layout must match the kernel's byte-for-byte.
typedef struct {
	unsigned char  major;
	unsigned char  minor;
	unsigned char  build;
	unsigned char  reserved;
	unsigned short release;
} SMCVersion;

typedef struct {
	uint16_t version;
	uint16_t length;
	uint32_t cpuPLimit;
	uint32_t gpuPLimit;
	uint32_t memPLimit;
} SMCPLimitData;

typedef struct {
	uint32_t dataSize;
	uint32_t dataType;
	uint8_t  dataAttributes;
} SMCKeyInfoData;

typedef struct {
	uint32_t       key;
	SMCVersion     vers;
	SMCPLimitData  pLimitData;
	SMCKeyInfoData keyInfo;
	uint8_t        result;
	uint8_t        status;
	uint8_t        data8;
	uint32_t       data32;
	uint8_t        bytes[32];
} SMCParamStruct;

enum {
	kSMCHandleYPCEvent  = 2,
	kSMCReadKey         = 5,
	kSMCGetKeyFromIndex = 8,
	kSMCGetKeyInfo      = 9,
};

static int pulse_smc_open(io_connect_t *conn) {
	io_service_t svc = IOServiceGetMatchingService(kIOMainPortDefault,
	                                               IOServiceMatching("AppleSMC"));
	if (!svc)
		return -1;
	kern_return_t kr = IOServiceOpen(svc, mach_task_self(), 0, conn);
	IOObjectRelease(svc);
	return kr == KERN_SUCCESS ? 0 : -1;
}

static void pulse_smc_close(io_connect_t conn) {
	IOServiceClose(conn);
}

static int pulse_smc_call(io_connect_t conn, SMCParamStruct *in, SMCParamStruct *out) {
	size_t outSize = sizeof(SMCParamStruct);
	kern_return_t kr = IOConnectCallStructMethod(conn, kSMCHandleYPCEvent,
	                                             in, sizeof(SMCParamStruct),
	                                             out, &outSize);
	if (kr != KERN_SUCCESS) return -1;
	return out->result; // 0 = ok, 132 = key not found
}

// pulse_smc_read reads a key: first info (size+type), then the data.
static int pulse_smc_read(io_connect_t conn, uint32_t key,
                          uint32_t *dataType, uint32_t *dataSize, uint8_t *bytes) {
	SMCParamStruct in, out;

	memset(&in, 0, sizeof(in));
	memset(&out, 0, sizeof(out));
	in.key = key;
	in.data8 = kSMCGetKeyInfo;
	int rc = pulse_smc_call(conn, &in, &out);
	if (rc != 0) return rc;

	*dataType = out.keyInfo.dataType;
	*dataSize = out.keyInfo.dataSize;
	if (*dataSize > 32) return -2;

	memset(&in, 0, sizeof(in));
	in.key = key;
	in.keyInfo.dataSize = *dataSize;
	in.data8 = kSMCReadKey;
	memset(&out, 0, sizeof(out));
	rc = pulse_smc_call(conn, &in, &out);
	if (rc != 0) return rc;

	memcpy(bytes, out.bytes, 32);
	return 0;
}

// pulse_smc_key_at_index returns the fourcc of the key at the given index
// (0 .. "#KEY"-1); used to enumerate every key the SMC exposes.
static int pulse_smc_key_at_index(io_connect_t conn, uint32_t index, uint32_t *key) {
	SMCParamStruct in, out;
	memset(&in, 0, sizeof(in));
	memset(&out, 0, sizeof(out));
	in.data8  = kSMCGetKeyFromIndex;
	in.data32 = index;
	int rc = pulse_smc_call(conn, &in, &out);
	if (rc != 0) return rc;
	*key = out.key;
	return 0;
}
*/
import "C"

import (
	"fmt"
	"sync"

	"github.com/emgeorrk/pulse/internal/entity"
)

// SMC is an AppleSMC client (works on both Intel and Apple Silicon).
// Here: fans (both platforms), temperatures (the Intel path), and the
// Apple Silicon GPU temperature keys (Tg*).
type SMC struct {
	conn    C.io_connect_t
	gpuOnce sync.Once
	gpuKeys []string // discovered Apple Silicon "Tg*" GPU temperature keys
}

func NewSMC() (*SMC, error) {
	s := &SMC{}
	if C.pulse_smc_open(&s.conn) != 0 {
		return nil, fmt.Errorf("AppleSMC service unavailable")
	}
	return s, nil
}

func (s *SMC) Close() {
	C.pulse_smc_close(s.conn)
}

// ReadKey reads and decodes a single SMC key ("FNum", "F0Ac", "TC0P", …).
func (s *SMC) ReadKey(key string) (float64, error) {
	if len(key) != 4 {
		return 0, fmt.Errorf("SMC key must be 4 chars: %q", key)
	}
	var (
		dataType C.uint32_t
		dataSize C.uint32_t
		buf      [32]C.uint8_t
	)
	k := C.uint32_t(key[0])<<24 | C.uint32_t(key[1])<<16 | C.uint32_t(key[2])<<8 | C.uint32_t(key[3])
	if rc := C.pulse_smc_read(s.conn, k, &dataType, &dataSize, &buf[0]); rc != 0 {
		return 0, fmt.Errorf("SMC read %q: rc=%d", key, int(rc))
	}
	typ := string([]byte{
		byte(dataType >> 24), byte(dataType >> 16), byte(dataType >> 8), byte(dataType),
	})
	b := make([]byte, dataSize)
	for i := range b {
		b[i] = byte(buf[i])
	}
	return decodeSMC(typ, b)
}

// FanCount returns the number of fans (0 on fanless models).
func (s *SMC) FanCount() int {
	n, err := s.ReadKey("FNum")
	if err != nil || n < 0 || n > 8 {
		return 0
	}
	return int(n)
}

// Fans reads the RPM of every fan; rated min/max are read on a best-effort basis.
func (s *SMC) Fans() ([]entity.Fan, error) {
	count := s.FanCount()
	if count == 0 {
		return nil, fmt.Errorf("no fans")
	}
	fans := make([]entity.Fan, 0, count)
	for i := 0; i < count; i++ {
		rpm, err := s.ReadKey(fmt.Sprintf("F%dAc", i))
		if err != nil {
			continue
		}
		f := entity.Fan{Name: fmt.Sprintf("Fan %d", i+1), RPM: rpm}
		f.Min, _ = s.ReadKey(fmt.Sprintf("F%dMn", i))
		f.Max, _ = s.ReadKey(fmt.Sprintf("F%dMx", i))
		fans = append(fans, f)
	}
	if len(fans) == 0 {
		return nil, fmt.Errorf("fan keys unreadable")
	}
	return fans, nil
}

// intelTempKeys is a curated list of temperature SMC keys for Intel Macs
// (VirtualSMC Docs/SMCSensorKeys.txt). WARNING: this path has not been
// verified on real Intel hardware (the dev machine is Apple Silicon).
var intelTempKeys = []struct{ key, label string }{
	{"TC0P", "CPU proximity"},
	{"TC0D", "CPU die"},
	{"TC0E", "CPU die (PECI)"},
	{"TC0F", "CPU die (filtered)"},
	{"TG0P", "GPU proximity"},
	{"TG0D", "GPU die"},
	{"TM0P", "Memory proximity"},
	{"TB0T", "Battery"},
	{"Ts0P", "Palm rest"},
	{"TA0P", "Ambient"},
	{"TH0P", "Drive bay"},
	{"TW0P", "Airport"},
}

// Temps is the Intel temperature path via SMC keys; on Apple Silicon HID is
// used instead (richer and more accurate), so this isn't called there.
func (s *SMC) Temps() ([]entity.Reading, error) {
	var out []entity.Reading
	for _, k := range intelTempKeys {
		v, err := s.ReadKey(k.key)
		if err != nil || v <= 0 || v > 125 {
			continue // key doesn't exist on this model, or garbage value — skip
		}
		out = append(out, entity.Reading{Name: k.label, Value: v})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no readable temperature keys")
	}
	return out, nil
}

// keyCount returns the total number of keys the SMC exposes ("#KEY").
func (s *SMC) keyCount() (int, error) {
	n, err := s.ReadKey("#KEY")
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// keyAtIndex returns the 4-char name of the key at the given index.
func (s *SMC) keyAtIndex(i int) (string, error) {
	var key C.uint32_t
	if rc := C.pulse_smc_key_at_index(s.conn, C.uint32_t(i), &key); rc != 0 {
		return "", fmt.Errorf("SMC key at index %d: rc=%d", i, int(rc))
	}
	return smcKeyString(uint32(key)), nil
}

// discoverGPUTempKeys enumerates every SMC key and keeps the readable Tg*
// ones. Values are deliberately not checked: a power-gated GPU may report
// 0 °C at startup, and dropping its key here would hide it forever.
func (s *SMC) discoverGPUTempKeys() {
	count, err := s.keyCount()
	if err != nil {
		return
	}
	for i := 0; i < count; i++ {
		key, err := s.keyAtIndex(i)
		if err != nil || !isGPUTempKey(key) {
			continue
		}
		if _, err := s.ReadKey(key); err != nil {
			continue
		}
		s.gpuKeys = append(s.gpuKeys, key)
	}
}

// GPUTempSensors returns the display labels contributed by the discovered
// Apple Silicon GPU temperature keys. Discovery runs once, lazily.
func (s *SMC) GPUTempSensors() []string {
	s.gpuOnce.Do(s.discoverGPUTempKeys)
	if len(s.gpuKeys) == 0 {
		return nil
	}
	return []string{gpuTempSensorName}
}

// GPUTemps reads the discovered Tg* keys and averages them into a single
// "GPU die" reading. Implausible values (v ≤ 0 or v > 125) are skipped for
// this tick only; the key set never shrinks. It errors only when no keys
// were discovered.
func (s *SMC) GPUTemps() ([]entity.Reading, error) {
	s.gpuOnce.Do(s.discoverGPUTempKeys)
	if len(s.gpuKeys) == 0 {
		return nil, fmt.Errorf("no GPU temperature keys")
	}
	var (
		sum float64
		n   int
	)
	for _, key := range s.gpuKeys {
		v, err := s.ReadKey(key)
		if err != nil || v <= 0 || v > 125 {
			continue
		}
		sum += v
		n++
	}
	if n == 0 {
		return nil, nil // keys exist but every value is implausible this tick
	}
	return []entity.Reading{{Name: gpuTempSensorName, Value: sum / float64(n)}}, nil
}
