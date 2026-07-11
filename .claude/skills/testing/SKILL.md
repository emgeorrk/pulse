---
name: testing
description: Review code for Go testing conventions and write or fix tests. Use when writing new tests or reviewing existing ones in this project: table-driven tests, gomock mocks, parallel subtests.
user-invocable: false
---

# Testing Conventions

## Table-driven tests

All tests MUST be table-driven. Each test case is a struct in a slice.

Use `t.Parallel()` at both levels — at the top of the test function and inside each subtest:

```go
func TestFoo(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name    string
        input   ...
        want    ...
        wantErr bool
    }{
        {name: "success", ...},
        {name: "error case", ..., wantErr: true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            // ...
        })
    }
}
```

**Note:** do NOT use `t.Parallel()` in tests that touch real hardware (SMC, HID, IOReport) unless the test explicitly handles concurrency.

**Exception:** a test with a single scenario, a sequential stateful flow
(e.g. pin/unpin order in `config.TestTogglePin`), or a one-off invariant
check may skip the table — say why in a short comment — but still calls
`t.Parallel()` at the top.

## Mocks (gomock / mockgen)

This project uses **gomock** (`go.uber.org/mock`). Do NOT write fakes or stubs
by hand — generate mocks with mockgen. `mockgen` is a `tool` dependency in
`go.mod`, so it runs via `go tool mockgen` (no global install needed).

- Sensor interfaces live in `internal/sensors/contracts.go`; their mocks are
  generated into `internal/sensors/mocks` (package `mocks`) by the
  `//go:generate` directive at the top of `contracts.go`.
- After changing any interface, regenerate: `go generate ./...`
  (or `make generate`). Never edit generated files.
- When adding an interface in a new file/package, add a `//go:generate` line
  next to it following the same pattern:

```go
//go:generate go tool mockgen -source=<file>.go -destination=mocks/<pkg>.go -package=mocks
```

Usage in tests:

```go
ctrl := gomock.NewController(t) // Finish is registered via t.Cleanup automatically
src := mocks.NewMockTempSource(ctrl)
src.EXPECT().Temps().Return([]entity.Reading{{Name: "PMU tdie1", Value: 35}}, nil)
```

- Create the controller and mocks **inside each subtest** (after
  `t.Parallel()`), never share mocks across subtests.
- A test package importing `internal/sensors/mocks` from within
  `internal/sensors` must be an external test package (`package sensors_test`)
  to avoid an import cycle — see `multi_test.go`.
- Func adapters used by production code (e.g. `sensors.TempFunc` in
  `internal/sensors/multi.go`, used by `internal/app`) stay — they are wiring,
  not test fakes.

## Which functions to test

Not every function needs a test. Determine which functions are **primary** vs **auxiliary**:

**Primary functions (always test):**
- Exported functions — callable from outside the package
- Unexported handlers, middleware, or entry points (e.g. HTTP handlers registered on a router)

**Auxiliary functions (test only if complex logic warrants it):**
- Unexported helpers called only by primary functions
- Simple formatters, converters, or wrappers

## One test function per primary function

Each primary function gets **exactly one** test function with a table inside it.
Do not split cases across multiple test functions for the same subject.

```
// Good
func TestHandleAction(t *testing.T) { /* table with all cases */ }

// Bad
func TestHandleAction_Success(t *testing.T) { ... }
func TestHandleAction_Error(t *testing.T) { ... }
```

## Exception: real-hardware integration

Unit tests must not require real sensors: test pure logic (deltas, decoding,
merging) on fixed inputs. If a **second** test function reading real hardware
(SMC / HID / IOReport, darwin-only) is genuinely useful, it is allowed:

```go
// Unit test (fixed inputs)
func TestDecodeSMC(t *testing.T) { /* table over raw byte fixtures */ }

// Integration test (real Mac required)
func TestReadSMC_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping hardware test")
    }
    // reads the actual SMC on this machine
}
```

The integration test must be skippable via the `-short` flag and must not
assert exact sensor values (they vary per Mac model) — only that reads succeed
and values are plausible. Remember such tests only run on macOS
(`*_darwin.go` + CGO).
