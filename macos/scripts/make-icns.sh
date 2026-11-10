#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
#
# Build AppIcon.icns from docs/social/zyvor-mark.svg (or a provided PNG).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT_DIR="${1:-$ROOT/macos/build/icon}"
SVG="${SVG:-$ROOT/docs/social/zyvor-mark.svg}"
mkdir -p "$OUT_DIR"
ICONSET="$OUT_DIR/AppIcon.iconset"
rm -rf "$ICONSET"
mkdir -p "$ICONSET"

MASTER="$OUT_DIR/master-1024.png"

if [[ -f "${MASTER_PNG:-}" ]]; then
  cp "$MASTER_PNG" "$MASTER"
elif command -v qlmanage >/dev/null 2>&1 && [[ -f "$SVG" ]]; then
  # qlmanage writes <basename>.png into -o dir
  qlmanage -t -s 1024 -o "$OUT_DIR" "$SVG" >/dev/null
  src="$(find "$OUT_DIR" -maxdepth 1 -name '*.png' ! -name 'master-1024.png' | head -1)"
  if [[ -n "$src" ]]; then
    mv "$src" "$MASTER"
  fi
fi

if [[ ! -f "$MASTER" ]]; then
  # Fallback: solid brand square via sips + a tiny PPM converted by sips is awkward;
  # synthesize a PNG with Python's stdlib only (no Pillow).
  python3 - "$MASTER" <<'PY'
import struct, zlib, sys
path = sys.argv[1]
w = h = 1024
# Brand ink #111111 with signal orange #ff5a15 circle
def pixel(x, y):
    cx, cy, r = 512, 512, 360
    dx, dy = x - cx, y - cy
    if dx*dx + dy*dy <= r*r:
        return (255, 90, 21, 255)
    return (17, 17, 17, 255)

raw = bytearray()
for y in range(h):
    raw.append(0)
    for x in range(w):
        raw.extend(pixel(x, y))

def chunk(tag, data):
    return struct.pack('>I', len(data)) + tag + data + struct.pack('>I', zlib.crc32(tag + data) & 0xffffffff)

png = b'\x89PNG\r\n\x1a\n'
png += chunk(b'IHDR', struct.pack('>IIBBBBB', w, h, 8, 6, 0, 0, 0))
png += chunk(b'IDAT', zlib.compress(bytes(raw), 9))
png += chunk(b'IEND', b'')
open(path, 'wb').write(png)
PY
fi

sizes=(16 32 128 256 512)
for s in "${sizes[@]}"; do
  sips -z "$s" "$s" "$MASTER" --out "$ICONSET/icon_${s}x${s}.png" >/dev/null
  double=$((s * 2))
  sips -z "$double" "$double" "$MASTER" --out "$ICONSET/icon_${s}x${s}@2x.png" >/dev/null
done

iconutil -c icns "$ICONSET" -o "$OUT_DIR/AppIcon.icns"
echo "$OUT_DIR/AppIcon.icns"
