#!/bin/sh
# Regenerates internal/controller/tray/icons/png/*.png from the SVG sources
# in internal/controller/tray/icons/svg/. The PNGs are committed, so this
# only needs to run when the SVG set changes.
#
# Output is 64x64 (black/#222 + alpha) — drawn at ~17 pt by the app, i.e. a
# @2x asset with headroom. Only the alpha channel is used: the app tints the
# glyph to match the menu bar text.
set -eu

root="$(cd "$(dirname "$0")/.." && pwd)"
svg_dir="$root/internal/controller/tray/icons/svg"
png_dir="$root/internal/controller/tray/icons/png"
size=64

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

# The Vitals set has no gear; the Settings item uses an SF Symbol instead.
swift "$root/scripts/sfsymbol2png.swift" gearshape.fill "$png_dir/settings.png" "$size"
echo "$png_dir/settings.png"
