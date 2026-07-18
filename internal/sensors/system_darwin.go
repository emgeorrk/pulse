//go:build darwin

package sensors

/*
#include <stdlib.h>
#include <libproc.h>

// pulse_proc_count returns the number of running processes, or -1.
// proc_listallpids returns a pid count both when probing with NULL and when
// filling a buffer (libproc divides the kernel's byte count by sizeof(int)),
// but the NULL probe includes kernel headroom — only the second call counts
// the pids actually present.
static int pulse_proc_count(void) {
	int cap = proc_listallpids(NULL, 0);
	if (cap <= 0) {
		return -1;
	}
	cap += 16; // headroom: processes may spawn between the two calls
	pid_t *buf = malloc((size_t)cap * sizeof(pid_t));
	if (buf == NULL) {
		return -1;
	}
	int n = proc_listallpids(buf, cap * (int)sizeof(pid_t));
	free(buf);
	return n;
}
*/
import "C"

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/emgeorrk/pulse/internal/entity"
	"golang.org/x/sys/unix"
)

// System reads system-wide state: load averages (vm.loadavg), uptime
// (kern.boottime), the process count (libproc) and the open-file count
// (kern.num_files). Everything is plain userspace sysctl/libproc — no root.
type System struct{}

func NewSystem() *System { return &System{} }

// loadAvg mirrors struct loadavg from <sys/resource.h>: three fixed-point
// samples plus the scale divisor (fixpt_t ldavg[3]; long fscale).
type loadAvg struct {
	Ldavg  [3]uint32
	Fscale uint64
}

func (s *System) System() (entity.SystemStats, error) {
	buf, err := unix.SysctlRaw("vm.loadavg")
	if err != nil {
		return entity.SystemStats{}, fmt.Errorf("sysctl vm.loadavg: %w", err)
	}

	if len(buf) < int(unsafe.Sizeof(loadAvg{})) {
		return entity.SystemStats{}, fmt.Errorf("%w: %d bytes", errLoadAvgShortRead, len(buf))
	}

	la := *(*loadAvg)(unsafe.Pointer(&buf[0]))
	if la.Fscale == 0 {
		return entity.SystemStats{}, errLoadAvgScale
	}

	scale := float64(la.Fscale)
	st := entity.SystemStats{
		Load1:  float64(la.Ldavg[0]) / scale,
		Load5:  float64(la.Ldavg[1]) / scale,
		Load15: float64(la.Ldavg[2]) / scale,
	}

	boot, err := unix.SysctlTimeval("kern.boottime")
	if err != nil {
		return entity.SystemStats{}, fmt.Errorf("sysctl kern.boottime: %w", err)
	}

	if up := time.Now().Unix() - boot.Sec; up > 0 {
		st.UptimeSec = uint64(up)
	}

	// Process and open-file counts aren't critical: on failure they stay 0
	// and their rows show "—".
	if n := int(C.pulse_proc_count()); n > 0 {
		st.Procs = n
	}

	if n, err := unix.SysctlUint32("kern.num_files"); err == nil {
		st.OpenFiles = int(n)
	}

	return st, nil
}
