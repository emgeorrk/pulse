# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

**Pulse** — a native macOS menu bar system monitor (Go + CGO), a *feature* (not
code) equivalent of the GNOME extension **Vitals**
(github.com/corecoding/Vitals). Vitals is JavaScript in GNOME Shell reading
Linux `/proc`, `/sys`, and `lm-sensors` — none of which exist on macOS. Treat
that repo as a **feature spec only**; never port its code or architecture.

All reads MUST work from userspace: **no sudo, no `powermetrics`** (it needs
root — a dead end for a background agent). The reference tools (mactop, iSMC)
prove every metric is reachable without admin rights.

## Build / test / run

CGO is mandatory (the sensor layer calls IOKit / Foundation / Mach); all sensor
code is darwin-only and must build on a real Mac.

```sh
make run      # build Pulse.app, ad-hoc sign, launch (the normal dev loop)
make once     # print one metrics frame to stdout — sensor check without the UI
make build    # CGO_ENABLED=1 go build -o bin/pulse ./cmd/pulse
make bundle   # assemble bin/Pulse.app with LSUIElement=true (no Dock icon)
make test     # go test ./...
make lint     # golangci-lint run   (lint-fix = with --fix)
make vet      # go vet ./...
make generate # regenerate gomock mocks (go generate ./...)
```

- Single package: `go test ./internal/sensors/...` · single test:
  `go test -run TestDecodeSMC ./internal/sensors/`.
- **Integration tests** that read real hardware skip under `-short`
  (`go test -short ./...`); they never assert exact sensor values.
- `PULSE_DEBUG=1 ./bin/pulse -once` also dumps the IOReport channels — useful
  when porting to a new chip generation.

## Architecture

Clean-architecture layers (inspired by go-clean-template), one-directional:

```
cmd/pulse (main, -once flag)
  → internal/app        wiring: probe hardware, build Sources + Caps, start loop
    → internal/sensors  data sources (repo layer): Mach, getifaddrs, IOKit, SMC, HID, IOReport
    → internal/usecase  sampling loop + metric math (counter deltas, aggregates)
    → internal/controller/tray  systray UI + the metric registry
internal/entity  domain types (no deps): Snapshot, *Stats, Caps, MetricID
config           JSON settings via a thread-safe Store
pkg/format       Vitals-style value formatting (Bytes, Speed, Temp, Hertz, Sparkline…)
```

**Startup flow** (`internal/app/app.go`): `ReadHWInfo()` detects the chip, then
`probe()` reads each optional sensor **once**. A source that responds is stored
in `sensors.Sources`; the corresponding `entity.Caps` flag/list is set.
`usecase.NewMonitor(&src, store)` gets those sources; `tray.New(store, hw,
caps)` builds the UI. CPU and Mem are the only mandatory sources — everything
else is optional.

**Capability gating is the core pattern.** A sensor missing on this hardware
disables *its own group*, never the app (CLAUDE spec: *hide, don't crash*).
`Caps` flows into `tray/registry.go:buildGroups`, which only constructs the
groups the hardware supports. When adding a metric you touch three places: a
`Source` interface + impl in `sensors`, a field in `entity.Snapshot`/`Caps`,
sampling in `usecase.Monitor.sample`, and a `group`/`metric` in `registry.go`.

**Sampling** (`usecase/monitor.go`): `Monitor.Start(ctx)` runs one goroutine
(never the UI thread), `prime()`s counter baselines, then ticks at
`store.Get().Interval()`, re-reading the interval each tick so a settings change
applies without restart. Each tick emits an `entity.Snapshot` on a buffered
channel; if the UI hasn't drained the previous frame it's dropped.

**Everything is delta-based.** CPU ticks and `if_data` net counters are **32-bit
and wrap** — subtract in `uint32` (correct modulo 2³²); disk I/O totals are
64-bit. Power is averaged from cumulative IOReport energy counters. See the
doc comments on `CPUUsage` / `NetRates` / `entity.CoreTicks`.

**UI** (`tray.go` + `registry.go`): pinned metrics render inline in the status
title; the dropdown lists all groups, each with a live aggregate in its header;
a checkbox click pins/unpins (`config.Store.TogglePin`). `systray.Run` owns the
Cocoa main thread — the sampling and animation goroutines start from `onReady`.

## Two hardware paths — keep them separate and labeled

`entity.HWInfo.IsAppleSilicon` (from `machdep.cpu.brand_string`) is the branch
point. Handle fanless models (MacBook Air) — no fan sensors, hide the section.

| Metric | Apple Silicon | Intel |
|---|---|---|
| CPU load | `host_processor_info` (Mach) | same |
| Memory | `host_statistics64` / `vm_statistics64` | same |
| Network | `getifaddrs()` deltas | same |
| Disk | `statfs` usage; IOKit for I/O | same |
| Temps | `IOHIDEventSystemClient` (HID hub, LE float); GPU fallback via SMC `Tg*` | SMC keys (`TC0P…`, sp78) — **untested** |
| Voltage | HID sensor hub | — |
| Fans | SMC `F%dAc`, type `flt ` LE | SMC, `fpe2` big-endian — **untested** |
| Power | IOReport Energy Model | — |
| CPU freq | IOReport perf states × device-tree freq tables | — |
| System (load, uptime, procs, open files) | sysctl `vm.loadavg`/`kern.boottime`/`kern.num_files` + libproc `proc_listallpids` | same |

The Intel SMC paths were written from references (iSMC, VirtualSMC docs) and are
**unverified on real Intel hardware**. Tested on: **Mac17,8 MacBook Pro, Apple
M5 Pro (18 cores), macOS 26.5.2** — all Apple Silicon paths incl. GPU temp via
SMC `Tg*`. Record the model + chip for any feature you verify.

## Conventions (enforced)

- **Build tags:** all platform code is `//go:build darwin` with a `_darwin.go`
  suffix; CGO preambles link frameworks via `#cgo LDFLAGS: -framework IOKit …`.
- **Errors — sentinels only.** No `fmt.Errorf`/`errors.New` producing anonymous
  errors in production code. Each package declares `var errFoo = errors.New(…)`
  in its `errors.go` (see `internal/sensors/errors.go`); wrap with
  `fmt.Errorf("…: %w", errFoo)`. Test files are exempt. See the **errors** skill.
- **Tests — table-driven + gomock.** Every test is a table with `t.Parallel()`
  at both levels (except real-hardware tests). Mocks are generated with
  `go tool mockgen` from `sensors/contracts.go` into `sensors/mocks` — never
  hand-write fakes, never edit generated files; run `make generate` after
  changing an interface. See the **testing** skill.
- **Linting is strict** (`.golangci.yml`): `gofumpt`, `fieldalignment`,
  `wsl_v5`, low `funlen`/`gocyclo`/`gocognit` limits, `mnd`, `err113`. Deliberate
  exceptions carry a `//nolint:<linter> // reason` with an explanation. Run
  `make lint` before finishing. `third_party/` and `*_test.go` are excluded.
- **Vendored systray fork:** `go.mod` `replace`s `fyne.io/systray` with
  `./third_party/systray` — a local patch (marked `PATCH(pulse)` in
  `systray_darwin.m`) that removes the action from items with a submenu, else
  macOS 14+ closes the whole menu on click. Preserve those patches across any
  upstream bump.

## Reference implementations (study the approach; respect licenses)

- **metaspartan/mactop** — Go+CGO SMC/IOReport/IOKit/HID, no sudo. PRIMARY.
- **dkorunic/iSMC** — Go+CGO SMC + Apple Silicon HID hub.
- **exelban/stats** — Swift menu-bar monitor; best FEATURE/UX reference.
- **acidanthera/VirtualSMC** `Docs/SMCSensorKeys.txt` — SMC key reference.

## Workflow rules

1. Before writing sensor code, inspect *this* machine (`ioreg -lfx`, enumerate
   SMC/HID keys) to confirm what actually exists.
2. Fail gracefully — a missing sensor/key hides its group, never crashes.
3. Distribution to other Macs needs notarization + an Apple Developer account;
   local builds are ad-hoc signed (`codesign -s -`).
