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

# The Vitals sets have no gear; the Settings item uses an SF Symbol instead,
# shared across every style, so it lives at the png root (not in a style dir).
swift "$root/scripts/sfsymbol2png.swift" gearshape.fill "$png_root/settings.png" "$size"
echo "$png_root/settings.png"
