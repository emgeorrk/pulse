//go:build debug

// Package pprof exposes runtime profiling for debug builds. In release builds
// (without the `debug` tag) Start is a no-op and neither net/http nor
// net/http/pprof is linked into the binary — see pprof_noop.go.
package pprof

import (
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"time"
)

const (
	// defaultAddr is loopback-only so the profiling server is never reachable
	// off the machine; override with the PULSE_PPROF_ADDR env var.
	defaultAddr = "localhost:6060"
	addrEnv     = "PULSE_PPROF_ADDR"

	statsInterval     = 10 * time.Second
	readHeaderTimeout = 5 * time.Second

	bytesPerMiB = 1024 * 1024
)

// Start launches the pprof HTTP server and a periodic runtime-stats logger,
// each on its own goroutine. Both live for the whole process lifetime (the
// process exits on Quit), so no shutdown handle is needed.
func Start() {
	addr := defaultAddr
	if v := os.Getenv(addrEnv); v != "" {
		addr = v
	}

	go serve(addr)
	go logStats()
}

// serve runs the pprof endpoints on a private mux (never DefaultServeMux) so
// no handlers leak onto any other server the process might start.
func serve(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index) // heap, goroutine, allocs, block, mutex…
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	//nolint:gosec // addr is an operator-supplied env var in a debug-only build, not attacker-controlled.
	log.Printf("pprof: profiling server on http://%s/debug/pprof/", addr)

	if err := srv.ListenAndServe(); err != nil {
		log.Printf("pprof: server stopped: %v", err)
	}
}

// logStats prints the goroutine count and heap stats every statsInterval, so a
// leak shows up as a monotonically rising count and a plateau confirms the GC
// is keeping up.
func logStats() {
	ticker := time.NewTicker(statsInterval)
	defer ticker.Stop()

	for range ticker.C {
		var ms runtime.MemStats

		runtime.ReadMemStats(&ms)

		log.Printf(
			"pprof: goroutines=%d heap=%dMiB objects=%d sys=%dMiB numgc=%d",
			runtime.NumGoroutine(),
			ms.HeapAlloc/bytesPerMiB,
			ms.HeapObjects,
			ms.Sys/bytesPerMiB,
			ms.NumGC,
		)
	}
}
