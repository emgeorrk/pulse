package sensors

import "encoding/binary"

const smcKeySize = 4

// Apple Silicon GPU temperature key helpers. The HID sensor hub exposes no
// GPU-named temperature sensors on some generations (M5: only "PMU tdie*"),
// but the SMC does, under "Tg*" keys of type flt (e.g. M1: Tg05/Tg0D,
// M2: Tg0f/Tg0j, M4: Tg0G/Tg1U, M5: Tg0U/Tg0d — see btop#1653 and
// exelban/stats). Keys are discovered by enumeration rather than a hardcoded
// per-chip list, so new generations and SKUs work without code changes.

// smcKeyString converts a fourcc SMC key to its 4-char string form.
func smcKeyString(k uint32) string {
	var key [smcKeySize]byte
	binary.BigEndian.PutUint32(key[:], k)

	return string(key[:])
}

// isGPUTempKey reports whether an SMC key is an Apple Silicon GPU
// temperature key. Case-sensitive on purpose: Apple Silicon uses "Tg*",
// Intel's "TG*" keys belong to the curated Intel path and never reach here.
func isGPUTempKey(key string) bool {
	return len(key) == smcKeySize && key[0] == 'T' && key[1] == 'g'
}

// gpuTempSensorName labels the single averaged reading built from all Tg*
// keys (an M5 Pro exposes 42 of them, one per GPU block — individual rows
// would flood the sensor list). Contains "gpu" so AggregateTemps counts it
// toward the GPU aggregate; matches the Intel "GPU die" label.
const gpuTempSensorName = "GPU die"
