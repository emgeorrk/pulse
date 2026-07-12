# Pulse

A native macOS menu bar system monitor — a *feature* (not code) equivalent of
the GNOME extension [Vitals](https://github.com/corecoding/Vitals). Go + CGO, no
sudo, no `powermetrics`.

<p align="left">
  <a href="https://github.com/emgeorrk/pulse/actions/workflows/ci.yml"><img src="https://github.com/emgeorrk/pulse/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/emgeorrk/pulse/releases/latest"><img src="https://img.shields.io/github/v/release/emgeorrk/pulse?sort=semver" alt="Latest release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/emgeorrk/pulse" alt="License: MIT"></a>
  <img src="https://img.shields.io/badge/platform-macOS%2012%2B-lightgrey" alt="Platform: macOS 12+">
</p>

<p align="center">
  <img src="docs/screenshot.png" alt="Pulse menu bar dropdown showing CPU, memory, temperature, and other live metrics" width="340">
</p>

Pulse lives in your menu bar: pinned metrics render inline, and the dropdown
lists every group with a live aggregate. No Dock icon, no window — quit from the
dropdown.

## Install

### Download (Apple Silicon)

1. Grab the latest `Pulse-<version>-arm64.zip` from the
   [Releases](https://github.com/emgeorrk/pulse/releases/latest) page and unzip
   it.
2. Move `Pulse.app` to `/Applications`.
3. Clear the quarantine flag, then open it:

   ```sh
   xattr -dr com.apple.quarantine /Applications/Pulse.app
   open /Applications/Pulse.app
   ```

The `xattr` step is required because the build is **ad-hoc signed, not
notarized** — there's no paid Apple Developer account behind it, so Gatekeeper
would otherwise refuse to launch it. It is not malware: the full source is in
this repo, and you can build it yourself (below).

> **Apple Silicon only.** The prebuilt binary is arm64. On an Intel Mac, build
> from source — but note the Intel sensor paths are unverified (see
> [Platform paths](#platform-paths)).

### Build from source

Requires macOS 12+, the Xcode command line tools, and Go 1.26.

```sh
make run    # build Pulse.app, ad-hoc sign it, and launch it
make once   # check sensors without the UI: one metrics frame to stdout
make test   # unit tests
```

Locally built apps aren't quarantined, so no `xattr` step is needed.
`PULSE_DEBUG=1 ./bin/pulse -once` additionally prints the IOReport channels —
useful when porting to a new chip generation.

### Launch at login

Turn on **Launch at login** in Pulse's Settings; it installs a LaunchAgent.
Settings live in `~/Library/Application Support/pulse/config.json`.

## Features

- **CPU**: total load + per-core, history sparkline (▁▂▄▇) in the menu bar,
  per-cluster frequency (IOReport performance states × frequency tables from
  the device tree)
- **Memory**: used / available / physical / swap (Activity Monitor formula)
- **Temperatures**: all chip sensors + CPU/GPU/hottest aggregates
  (Apple Silicon — HID sensor hub; Intel — SMC keys)
- **Fans**: RPM + % of max (SMC; hidden on fanless models)
- **Voltage**: PMU sensors (Apple Silicon)
- **Network**: total ↓/↑ and per-interface, session traffic
- **Disk**: volume usage, read/write speeds, totals since boot
- **GPU**: utilization (IOAccelerator)
- **Power**: CPU/GPU/ANE in watts (IOReport Energy Model)
- **Battery**: charge, health, cycles, temperature, voltage, watts, time

## UI (modeled on Vitals)

- Pinned metrics show inline in the menu bar; clicking a metric in the dropdown
  (checkbox) pins/unpins it, and pin order matches bar order
- Groups in the dropdown show a live aggregate right in the header
- **Settings**: interval (1/2/3/5 s, no restart needed), °C/°F, GiB/GB,
  sparkline, launch at login
- A sensor unavailable on this hardware disables its own group, not the whole
  app

## Structure

Layers inspired by [go-clean-template](https://github.com/evrone/go-clean-template):
`internal/sensors` (data sources: Mach, getifaddrs, IOKit, SMC, HID, IOReport)
→ `internal/usecase` (sampling, deltas, aggregates) →
`internal/controller/tray` (systray UI, metric registry); domain types live in
`internal/entity`, and Vitals-style formatting lives in `pkg/format`.
[CLAUDE.md](CLAUDE.md) is the as-built architecture guide.

## Platform paths

`entity.HWInfo.IsAppleSilicon` is the branch point; fanless models (MacBook Air)
have no fan sensors and hide that section.

| Metric | Apple Silicon | Intel |
|---|---|---|
| Temperatures | `IOHIDEventSystemClient` (0xff00/5), GPU fallback via SMC `Tg*` | SMC keys (TC0P…) — **untested** |
| Voltage | HID (0xff08/3) | — |
| Fans | SMC `F%dAc`, type `flt ` LE | SMC, type `fpe2` BE — **untested** |
| Power | IOReport Energy Model | — |
| CPU frequency | IOReport perf states | — |

The Intel paths (SMC sp78 temperatures, fpe2 fans) were written from references
(iSMC, VirtualSMC docs) and have **not been verified on real Intel hardware**.

## Tested on

| Model | Chip | macOS | Features |
|---|---|---|---|
| Mac17,8 (MacBook Pro) | Apple M5 Pro, 18 cores | 26.5.2 | everything except the Intel paths; GPU temperature verified via SMC `Tg*` |

## Contributing

CGO is mandatory (the sensor layer links IOKit / Foundation / Mach), so all
sensor code is darwin-only and must build on a real Mac. Before opening a PR:

```sh
make test   # unit tests
make lint   # golangci-lint (strict; see .golangci.yml)
make vet    # go vet
```

Conventions (sentinel errors, table-driven + gomock tests, capability gating)
are documented in [CLAUDE.md](CLAUDE.md). If you verify a feature on new
hardware, record the model + chip in **Tested on**.

## License

[MIT](LICENSE) © 2026 Egor Merkushev.
