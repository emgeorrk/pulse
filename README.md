# pulse

A native menu bar monitor for macOS — a feature (not code) equivalent of the
GNOME extension [Vitals](https://github.com/corecoding/Vitals). Go + CGO, no
sudo, no `powermetrics`.

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

- Pinned metrics show inline in the menu bar; clicking a metric in the
  dropdown (checkbox) pins/unpins it, and pin order matches bar order
- Groups in the dropdown show a live aggregate right in the header
- **Settings**: interval (1/2/3/5 s, no restart needed), °C/°F, GiB/GB,
  sparkline, launch at login (LaunchAgent)
- Settings are stored in `~/Library/Application Support/pulse/config.json`

## Build and run

Requires macOS and the Xcode command line tools.

```sh
make run    # build Pulse.app, ad-hoc sign it, and launch it
make once   # check sensors without the UI: one metrics frame to stdout
make test   # unit tests
```

`PULSE_DEBUG=1 ./bin/pulse -once` additionally prints the IOReport channels —
useful when porting to a new chip generation.

`Pulse.app` is a background agent (`LSUIElement=true`): no Dock icon, just a
menu bar item. Quit via "Quit pulse" in the dropdown.

## Structure

Layers inspired by [go-clean-template](https://github.com/evrone/go-clean-template):
`internal/sensors` (data sources: Mach, getifaddrs, IOKit, SMC, HID,
IOReport) → `internal/usecase` (sampling, deltas, aggregates) →
`internal/controller/tray` (systray UI, metric registry); domain types live
in `internal/entity`, and Vitals-style formatting lives in `pkg/format`.

A sensor unavailable on this hardware disables its own group (Caps), not the
whole app.

## Platform paths

| Metric | Apple Silicon | Intel |
|---|---|---|
| Temperatures | `IOHIDEventSystemClient` (0xff00/5), GPU fallback via SMC `Tg*` | SMC keys (TC0P…) — **untested** |
| Voltage | HID (0xff08/3) | — |
| Fans | SMC `F%dAc`, type `flt ` LE | SMC, type `fpe2` BE — **untested** |
| Power | IOReport Energy Model | — |
| CPU frequency | IOReport perf states | — |

## Tested on

| Model | Chip | macOS | Features |
|---|---|---|---|
| Mac17,8 (MacBook Pro) | Apple M5 Pro, 18 cores | 26.5.2 | everything except the Intel paths; GPU temperature verified via SMC `Tg*` |

The Intel paths (SMC sp78 temperatures, fpe2 fans) were written from
references (iSMC, VirtualSMC docs) and have not been verified on real Intel
hardware.
