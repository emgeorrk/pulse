//go:build darwin

package sensors

/*
#include <sys/types.h>
#include <sys/socket.h>
#include <ifaddrs.h>
#include <net/if.h>
#include <net/if_var.h>
#include <net/if_dl.h>
#include <string.h>

typedef struct {
	char         name[16];
	unsigned int ibytes;
	unsigned int obytes;
} pulse_ifstat;

// pulse_net_counters collects counters for AF_LINK interfaces (excluding loopback).
static int pulse_net_counters(pulse_ifstat *out, int max) {
	struct ifaddrs *list, *cur;
	if (getifaddrs(&list) != 0)
		return -1;
	int n = 0;
	for (cur = list; cur && n < max; cur = cur->ifa_next) {
		if (!cur->ifa_addr || cur->ifa_addr->sa_family != AF_LINK) continue;
		if (cur->ifa_flags & IFF_LOOPBACK) continue;
		struct if_data *d = (struct if_data *)cur->ifa_data;
		if (!d) continue;
		strlcpy(out[n].name, cur->ifa_name, sizeof(out[n].name));
		out[n].ibytes = d->ifi_ibytes;
		out[n].obytes = d->ifi_obytes;
		n++;
	}
	freeifaddrs(list);
	return n;
}
*/
import "C"

import "github.com/emgeorrk/pulse/internal/entity"

const maxIfaces = 64

// Net reads cumulative traffic counters via getifaddrs()/if_data.
// The kernel counters are 32-bit — usecase handles the overflow.
type Net struct{}

func NewNet() *Net { return &Net{} }

func (*Net) Counters() ([]entity.NetCounters, error) {
	var buf [maxIfaces]C.pulse_ifstat
	n := int(C.pulse_net_counters(&buf[0], maxIfaces))
	if n < 0 {
		return nil, errNetworkCounters
	}
	out := make([]entity.NetCounters, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, entity.NetCounters{
			Name: C.GoString(&buf[i].name[0]),
			Rx:   uint32(buf[i].ibytes),
			Tx:   uint32(buf[i].obytes),
		})
	}
	return out, nil
}
