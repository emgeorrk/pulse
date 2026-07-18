// Package entity describes domain metric types, independent of data sources
// and UI.
package entity

// MetricID is a stable metric identifier used for pinning in the menu bar
// and settings: "cpu.total", "mem.used", "temp.cpu", "fan.1", "net.down", ….
type MetricID string

// CoreTicks holds cumulative load ticks for one core from Mach
// PROCESSOR_CPU_LOAD_INFO. In the kernel these are 32-bit counters that wrap
// around modulo 2^32, so deltas must be computed in uint32 arithmetic.
type CoreTicks struct {
	User   uint32
	System uint32
	Idle   uint32
	Nice   uint32
}

// CPUStats holds CPU load over the interval between two samples, as 0..1 fractions.
type CPUStats struct {
	Cores   []float64
	History []float64
	Total   float64
}

// MemStats holds physical memory and swap state, in bytes.
type MemStats struct {
	Total     uint64 // total physical memory size
	Used      uint64 // app memory + wired + compressed (matches "Memory Used" in Activity Monitor)
	Available uint64 // free + inactive
	Free      uint64
	Cached    uint64 // file-backed + purgeable (matches "Cached Files" in Activity Monitor)
	SwapTotal uint64
	SwapUsed  uint64
}

// UsedFraction returns the used-memory fraction, 0..1.
func (m MemStats) UsedFraction() float64 {
	if m.Total == 0 {
		return 0
	}

	return float64(m.Used) / float64(m.Total)
}

// HWInfo holds hardware info; IsAppleSilicon is the branch point for the
// sensor layers (SMC on Intel, IOHIDEventSystemClient on Apple Silicon).
type HWInfo struct {
	Chip           string // machdep.cpu.brand_string, e.g. "Apple M5 Pro"
	Model          string // hw.model, e.g. "Mac17,8"
	ModelName      string // product-name from IODeviceTree, e.g. "MacBook Pro (16-inch, M5 Pro)"; empty on Intel
	OSVersion      string // kern.osproductversion, e.g. "26.5.2"
	IsAppleSilicon bool
	NumCores       int
}

// NetCounters holds cumulative counters for one interface from if_data
// (getifaddrs). In the kernel these are 32-bit and wrap around modulo 2^32.
type NetCounters struct {
	Name string
	Rx   uint32
	Tx   uint32
}

// NetIface holds one interface's throughput over the interval.
type NetIface struct {
	Name string
	Down float64 // bytes/s
	Up   float64
}

// NetStats holds network stats: total throughput, session-accumulated
// traffic, the per-interface breakdown, and the public IP (empty when the
// lookup is disabled or hasn't answered yet).
type NetStats struct {
	PublicIP    string
	IPCountry   string
	Ifaces      []NetIface
	Down        float64
	Up          float64
	SessionDown uint64
	SessionUp   uint64
}

// PublicIPInfo is one public IP lookup result. Country is the ISO 3166-1
// alpha-2 code (upper case), empty when the provider doesn't report it.
type PublicIPInfo struct {
	IP      string
	Country string
}

// DiskUsage holds root-volume usage, in bytes.
type DiskUsage struct {
	Total     uint64
	Used      uint64
	Available uint64
}

func (d DiskUsage) UsedFraction() float64 {
	if d.Total == 0 {
		return 0
	}

	return float64(d.Used) / float64(d.Total)
}

// DiskStats holds usage + throughput and cumulative I/O since boot.
type DiskStats struct {
	DiskUsage
	ReadRate   float64 // bytes/s
	WriteRate  float64
	ReadTotal  uint64 // since boot (64-bit IOKit counters)
	WriteTotal uint64
}

// SystemStats holds system-wide state: load averages, uptime and the
// process/open-file counts. Zero Procs/OpenFiles means "unknown".
type SystemStats struct {
	Load1     float64
	Load5     float64
	Load15    float64
	UptimeSec uint64
	Procs     int
	OpenFiles int
}

// Reading is a single reading from a named sensor (temperature °C, volts, …).
type Reading struct {
	Name  string
	Value float64
}

// TempStats holds aggregates + every temperature sensor.
type TempStats struct {
	Hottest Reading
	Coolest Reading
	All     []Reading
	CPU     float64
	GPU     float64
	Avg     float64 // average across all sensors
}

// Fan is a single fan: current RPM and its rated limits.
type Fan struct {
	Name string
	RPM  float64
	Min  float64
	Max  float64
}

// BatteryStats holds battery state from IORegistry (AppleSmartBattery).
type BatteryStats struct {
	Percent     float64 // 0..1
	Health      float64 // 0..1, actual capacity vs rated capacity
	Cycles      int
	TempC       float64
	Volts       float64
	Watts       float64 // >0 charging, <0 discharging
	Charging    bool
	External    bool // running on AC power
	MinutesLeft int  // until discharged (or until fully charged while charging); -1 = unknown
}

// GPUStats holds GPU utilization from IOAccelerator PerformanceStatistics.
type GPUStats struct {
	Utilization float64 // 0..1
}

// PowerStats holds power per IOReport Energy Model channel, in watts.
type PowerStats struct {
	CPU   float64
	GPU   float64
	ANE   float64
	Total float64
}

// FreqStats holds the weighted-average CPU frequency per cluster (IOReport
// perf states × frequency tables from the device tree).
type FreqStats struct {
	Clusters []Reading // "E-cores"/"P-cores", Hz
	Max      float64   // max across clusters — the current effective frequency
}

// Caps records which metric groups are actually available on this hardware;
// the UI hides groups that aren't (per CLAUDE.md: hide, don't crash).
type Caps struct {
	VoltSensors  []string
	NetIfaces    []string
	FreqClusters []string
	TempSensors  []string
	FanCount     int
	Temps        bool
	Volts        bool
	Fans         bool
	Net          bool
	Battery      bool
	GPU          bool
	Power        bool
	Freq         bool
	Disk         bool
	System       bool
}

// Snapshot is one frame of all metrics sent to the UI. Groups that may be
// absent on this hardware or in this frame are pointers.
type Snapshot struct {
	Net     *NetStats
	Disk    *DiskStats
	Temps   *TempStats
	Battery *BatteryStats
	GPU     *GPUStats
	Power   *PowerStats
	Freq    *FreqStats
	System  *SystemStats
	Volts   []Reading
	Fans    []Fan
	CPU     CPUStats
	Mem     MemStats
}
