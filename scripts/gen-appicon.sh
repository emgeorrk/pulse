#!/bin/sh
# Regenerates build/darwin/AppIcon.icns from build/darwin/AppIcon.svg.
# macOS-only: renders the SVG at every iconset size via the NSImage-based
# svg2png.swift (same renderer gen-icons.sh uses), then packs them with
# iconutil. The .icns is committed, so this only needs to run when the SVG
# changes.
set -eu

root="$(cd "$(dirname "$0")/.." && pwd)"
svg="$root/build/darwin/AppIcon.svg"
out="$root/build/darwin/AppIcon.icns"
iconset="$(mktemp -d)/AppIcon.iconset"
mkdir -p "$iconset"

render() { swift "$root/scripts/svg2png.swift" "$svg" "$iconset/$1" "$2"; }

render icon_16x16.png        16
render icon_16x16@2x.png     32
render icon_32x32.png        32
render icon_32x32@2x.png     64
render icon_128x128.png     128
render icon_128x128@2x.png  256
render icon_256x256.png     256
render icon_256x256@2x.png  512
render icon_512x512.png     512
render icon_512x512@2x.png 1024

iconutil -c icns "$iconset" -o "$out"
rm -rf "$(dirname "$iconset")"
echo "$out"
