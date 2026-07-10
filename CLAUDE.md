# macOS System Monitor — Vitals-equivalent menu bar app

## Goal
Build a **native macOS menu bar app** that replicates the FEATURES of the GNOME
Shell extension "Vitals" (github.com/corecoding/Vitals) — **not** its code.

Vitals is JavaScript running inside GNOME Shell and reads Linux `/proc`, `/sys`,
and `lm-sensors`. None of that architecture or those data sources exist on macOS.
Treat the Vitals repo as a **feature spec only**. Do not attempt to port its code.

## Feature parity target (from Vitals)
- CPU: total load %, per-core load %, frequency
- Memory: used / free / cached, swap usage
- Temperatures: CPU, GPU, and other available sensors
- Fan speed(s) in RPM
- Network: upload/download throughput per interface
- Disk: usage % and read/write throughput
- Voltage (if available)
- Show a few user-selected metrics inline in the menu bar; full list in a dropdown
- User settings: which metrics are visible, update interval, units

## PLATFORM CONSTRAINTS — READ CAREFULLY

### Two hardware generations with DIFFERENT sensor access
- **Intel Macs:** temps/fans via SMC keys. Fan RPM is big-endian fpe2 fixed-point.
- **Apple Silicon (M1–M5, A-series):** temperature / voltage / current come from
  the **HID sensor hub via IOKit `IOHIDEventSystemClient`**, NOT standard SMC keys.
  Power/energy come from **IOReport**. Values are little-endian IEEE-754 floats.
- **Detect the chip at runtime and branch.** Read `machdep.cpu.brand_string`.
- Handle fanless models (e.g. MacBook Air): no fan sensors — hide that section.

### No sudo, no powermetrics
- All reads MUST work from userspace without admin rights.
- Do NOT depend on `powermetrics` or `sudo` — `powermetrics` requires root and is
  a dead end for a background app. Reference tools (mactop, iSMC) prove all needed
  data is available without root.

### Data sources
| Metric        | API |
|---------------|-----|
| CPU load      | `host_processor_info()` (Mach) |
| Memory        | `host_statistics64` / `vm_statistics64` (Mach) |
| Network       | `getifaddrs()` deltas |
| Disk usage    | `statfs`; disk I/O via IOKit |
| Temps / fans  | SMC (`AppleSMC` via `IOServiceOpen`) on Intel; `IOHIDEventSystemClient` on Apple Silicon |
| Power         | IOReport |

### Language / build
- Language: **Go** (project preference).
- Sensor layer requires **CGO** (`CGO_ENABLED=1`) to call IOKit / Foundation.
  Link with `-framework IOKit -framework Foundation`.
- Menu bar UI: use a systray library (e.g. `fyne.io/systray`) for v1 — one status
  item whose title shows selected metrics inline, plus a dropdown menu listing all
  metrics. (Migrate to a Swift `NSStatusItem` + popover only if richer UI is needed
  later, keeping the Go part as a helper.)
- App must be a **background agent**: set `LSUIElement = true` in `Info.plist`
  (no Dock icon). Sampling loop ~1–2 s, never on the UI thread.

## Reference implementations (study the sensor-access approach; respect licenses)
- **metaspartan/mactop** — Go + CGO; SMC / IOReport / IOKit / IOHIDEventSystemClient, no sudo. PRIMARY sensor reference.
- **dkorunic/iSMC** — Go + CGO; SMC + Apple Silicon HID sensor hub.
- **exelban/stats** — canonical Swift menu bar monitor; best FEATURE/UX reference.
- **ryyansafar/MacMonitor** — menu bar app architecture (status item + popover) with a native helper.
- **acidanthera/VirtualSMC** `Docs/SMCSensorKeys.txt` — SMC key reference.

## Build / test (MUST run on a real Mac)
- Requires macOS + Xcode command line tools. The agent cannot verify sensor
  readings without running on the target hardware.
- Provide a Makefile that builds a proper `.app` bundle (with the `LSUIElement`
  Info.plist) and ad-hoc signs it for local use (`codesign -s -`).
- Distribution to other machines needs notarization + an Apple Developer account.

## Workflow rules for the agent
1. Before writing sensor code, inspect THIS machine: run `ioreg -lfx` and enumerate
   the available SMC / HID keys to confirm what actually exists on this Mac.
2. Build in this order: **CPU + memory first** (pure Mach, no SMC — easiest), get
   live numbers showing in the menu bar, THEN add temps/fans/network/disk.
3. Keep Intel and Apple Silicon code paths clearly separated and labeled.
4. Record which Mac model + chip each feature was tested on.
5. Fail gracefully: if a sensor/key is missing on this hardware, hide it rather
   than crash.
