//go:build darwin

package sensors

/*
#include <mach/mach.h>

static kern_return_t pulse_vm_stat(vm_statistics64_data_t *stat) {
	mach_msg_type_number_t count = HOST_VM_INFO64_COUNT;
	return host_statistics64(mach_host_self(), HOST_VM_INFO64,
	                         (host_info64_t)stat, &count);
}

static vm_size_t pulse_page_size(void) {
	vm_size_t sz = 0;
	host_page_size(mach_host_self(), &sz);
	return sz;
}
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/emgeorrk/pulse/internal/entity"
	"golang.org/x/sys/unix"
)

// Mem reads memory state via Mach host_statistics64() and sysctl.
type Mem struct {
	total    uint64
	pageSize uint64
}

func NewMem() (*Mem, error) {
	total, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return nil, fmt.Errorf("sysctl hw.memsize: %w", err)
	}
	ps := uint64(C.pulse_page_size())
	if ps == 0 {
		return nil, fmt.Errorf("host_page_size returned 0")
	}
	return &Mem{total: total, pageSize: ps}, nil
}

func (m *Mem) Read() (entity.MemStats, error) {
	var stat C.vm_statistics64_data_t
	if kr := C.pulse_vm_stat(&stat); kr != C.KERN_SUCCESS {
		return entity.MemStats{}, fmt.Errorf("host_statistics64: kern_return_t %d", int(kr))
	}

	ps := m.pageSize
	free := uint64(stat.free_count) * ps
	inactive := uint64(stat.inactive_count) * ps
	wired := uint64(stat.wire_count) * ps
	compressed := uint64(stat.compressor_page_count) * ps
	internal := uint64(stat.internal_page_count) * ps
	purgeable := uint64(stat.purgeable_count) * ps

	// "Memory Used" as in Activity Monitor: App Memory (internal − purgeable)
	// + Wired + Compressed.
	appMem := uint64(0)
	if internal > purgeable {
		appMem = internal - purgeable
	}

	st := entity.MemStats{
		Total:     m.total,
		Used:      appMem + wired + compressed,
		Available: free + inactive,
		Free:      free,
	}

	// Swap isn't critical: on error we show zeros instead of failing.
	if total, used, err := readSwap(); err == nil {
		st.SwapTotal, st.SwapUsed = total, used
	}
	return st, nil
}

// xswUsage mirrors struct xsw_usage from <sys/sysctl.h>.
type xswUsage struct {
	Total     uint64
	Avail     uint64
	Used      uint64
	Pagesize  uint32
	Encrypted uint32
}

func readSwap() (total, used uint64, err error) {
	buf, err := unix.SysctlRaw("vm.swapusage")
	if err != nil {
		return 0, 0, fmt.Errorf("sysctl vm.swapusage: %w", err)
	}
	if len(buf) < int(unsafe.Sizeof(xswUsage{})) {
		return 0, 0, fmt.Errorf("vm.swapusage: short read (%d bytes)", len(buf))
	}
	x := *(*xswUsage)(unsafe.Pointer(&buf[0]))
	return x.Total, x.Used, nil
}
