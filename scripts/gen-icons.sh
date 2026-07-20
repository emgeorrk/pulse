#!/bin/sh
# Regenerates internal/controller/tray/icons/png/<style>/*.png from the SVG
# sources in internal/controller/tray/icons/svg/<style>/. Each style is a pack
# (gnome, classic); the PNGs are committed, so this only needs to run when the
# SVG set changes.
#
# Output is 64x64 (opaque glyph + alpha) — drawn at ~17 pt by the app, i.e. a
# @2x asset with headroom. Only the alpha channel is used: the app tints the
# glyph to match the menu bar text.
set -eu

root="$(cd "$(dirname "$0")/.." && pwd)"
svg_root="$root/internal/controller/tray/icons/svg"
png_root="$root/internal/controller/tray/icons/png"
size=64

for svg_dir in "$svg_root"/*/; do
  style="$(basename "$svg_dir")"
  if [ "$style" = "flags" ]; then continue; fi # full color, rendered below
  png_dir="$png_root/$style"
  mkdir -p "$png_dir"

  for svg in "$svg_dir"/*-symbolic.svg; do
    name="$(basename "$svg" -symbolic.svg)"
    out="$png_dir/$name.png"
    if command -v rsvg-convert >/dev/null 2>&1; then
      rsvg-convert -w "$size" -h "$size" "$svg" -o "$out"
    else
      swift "$root/scripts/svg2png.swift" "$svg" "$out" "$size"
    fi
    echo "$out"
  done
done

# The Vitals sets have no power glyph. Power is a pinnable metric (its icon
# must resolve per style like the rest), so the SF Symbol is rendered into
# every style pack.
for svg_dir in "$svg_root"/*/; do
  style="$(basename "$svg_dir")"
  if [ "$style" = "flags" ]; then continue; fi # not a style pack, no metric glyphs
  swift "$root/scripts/sfsymbol2png.swift" powerplug.fill "$png_root/$style/power.png" "$size"
  echo "$png_root/$style/power.png"
done

# Country flags (full color, shared by the gnome and classic styles). Unlike
# the glyph packs the RGB channels matter; several flags use SVG features
# (<marker> in us.svg) that NSImage's renderer mishandles, so rsvg-convert
# is required here — no svg2png.swift fallback.
flags_dir="$svg_root/flags"
if [ -d "$flags_dir" ]; then
  command -v rsvg-convert >/dev/null 2>&1 || {
    echo "gen-icons.sh: rsvg-convert required for flags (brew install librsvg)" >&2
    exit 1
  }
  mkdir -p "$png_root/flags"
  for svg in "$flags_dir"/*.svg; do
    out="$png_root/flags/$(basename "$svg" .svg).png"
    rsvg-convert -w "$size" -h "$size" "$svg" -o "$out"
    echo "$out"
  done
fi

# The Vitals sets also have no gear, info or activity glyphs; the Settings,
# About and Activity Monitor items use SF Symbols instead. These are
# menu items only (never in the menu bar title), so one PNG is shared across
# every style and lives at the png root (not in a style dir).
for spec in "gearshape.fill settings" "info.circle about" "waveform.path.ecg activity"; do
  set -- $spec
  swift "$root/scripts/sfsymbol2png.swift" "$1" "$png_root/$2.png" "$size"
  echo "$png_root/$2.png"
done
