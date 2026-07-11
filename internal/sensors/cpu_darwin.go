//go:build darwin

package sensors

/*
#include <mach/mach.h>

static kern_return_t pulse_cpu_load(natural_t *ncpu,
                                    processor_cpu_load_info_data_t **info,
                                    mach_msg_type_number_t *cnt) {
	processor_info_array_t arr;
	kern_return_t kr = host_processor_info(mach_host_self(), PROCESSOR_CPU_LOAD_INFO,
	                                       ncpu, &arr, cnt);
	if (kr == KERN_SUCCESS)
		*info = (processor_cpu_load_info_data_t *)arr;
	return kr;
}

static void pulse_cpu_load_free(processor_cpu_load_info_data_t *info,
                                mach_msg_type_number_t cnt) {
	vm_deallocate(mach_task_self(), (vm_address_t)info, cnt * sizeof(integer_t));
}
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/emgeorrk/pulse/internal/entity"
)

// CPU reads per-core load ticks via Mach host_processor_info().
// The API is identical on Intel and Apple Silicon, and needs no sudo.
type CPU struct{}

func NewCPU() *CPU { return &CPU{} }

func (*CPU) Ticks() ([]entity.CoreTicks, error) {
	var (
		ncpu C.natural_t
		info *C.processor_cpu_load_info_data_t
		cnt  C.mach_msg_type_number_t
	)

	if kr := C.pulse_cpu_load(&ncpu, &info, &cnt); kr != C.KERN_SUCCESS { //nolint:gocritic // cgo constants confuse dupSubExpr.
		return nil, fmt.Errorf("%w: kern_return_t %d", errCPUInfo, int(kr))
	}
	defer C.pulse_cpu_load_free(info, cnt)

	cores := unsafe.Slice(info, int(ncpu))
	out := make([]entity.CoreTicks, len(cores))
	for i, ci := range cores {
		out[i] = entity.CoreTicks{
			User:   uint32(ci.cpu_ticks[C.CPU_STATE_USER]),
			System: uint32(ci.cpu_ticks[C.CPU_STATE_SYSTEM]),
			Idle:   uint32(ci.cpu_ticks[C.CPU_STATE_IDLE]),
			Nice:   uint32(ci.cpu_ticks[C.CPU_STATE_NICE]),
		}
	}

	return out, nil
}
