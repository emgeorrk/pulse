//go:build !debug

// Package pprof exposes runtime profiling for debug builds only; this is the
// release stub, compiled when the `debug` build tag is absent. The real
// implementation lives in pprof_debug.go.
package pprof

// Start is a no-op in release builds: neither net/http nor net/http/pprof is
// linked into the binary, so the shipped app carries no profiling server.
func Start() {}
