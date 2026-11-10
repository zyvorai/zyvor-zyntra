#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
#
# Build Zyntra.app: SwiftUI host + embedded darwin zyntra + sample pack.
# Does not codesign; use build-dmg.sh for release signing/notarization.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MACOS="$ROOT/macos"
BUILD="$MACOS/build"
APP="$BUILD/Zyntra.app"
ARCHS="${ARCHS:-arm64}"
VERSION="${VERSION:-}"
if [[ -z "$VERSION" ]]; then
  VERSION="$(sed -n 's/^var version = "\(.*\)"/\1/p' "$ROOT/cmd/zyntra/main.go")"
fi

echo "==> version $VERSION"
echo "==> building console + Go binary (darwin)"
make -C "$ROOT" web

mkdir -p "$BUILD/bin"
# Universal optional: ARCHS="arm64 x86_64"
BINS=()
for arch in $ARCHS; do
  out="$BUILD/bin/zyntra-$arch"
  echo "==> go build darwin/$arch"
  CGO_ENABLED=0 GOOS=darwin GOARCH="$arch" go build -trimpath -ldflags="-s -w" \
    -o "$out" "$ROOT/cmd/zyntra"
  BINS+=("$out")
done

ZYNTRA_BIN="$BUILD/bin/zyntra"
if [[ ${#BINS[@]} -eq 1 ]]; then
  cp "${BINS[0]}" "$ZYNTRA_BIN"
else
  lipo -create -output "$ZYNTRA_BIN" "${BINS[@]}"
fi
chmod +x "$ZYNTRA_BIN"

echo "==> swift build (release)"
swift build -c release --package-path "$MACOS" --product Zyntra
SWIFT_BIN="$(swift build -c release --package-path "$MACOS" --show-bin-path)/Zyntra"

echo "==> icon"
"$MACOS/scripts/make-icns.sh" "$BUILD/icon" >/dev/null

echo "==> assemble $APP"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$SWIFT_BIN" "$APP/Contents/MacOS/Zyntra"
# Name must differ in case from Zyntra — APFS is often case-insensitive.
cp "$ZYNTRA_BIN" "$APP/Contents/MacOS/zyntra-engine"
chmod +x "$APP/Contents/MacOS/Zyntra" "$APP/Contents/MacOS/zyntra-engine"
cp "$BUILD/icon/AppIcon.icns" "$APP/Contents/Resources/AppIcon.icns"

# Bundled sample pack (shop fixtures) so first launch is not blank.
rm -rf "$APP/Contents/Resources/sample-pack"
mkdir -p "$APP/Contents/Resources/sample-pack"
cp -R "$ROOT/packs/shop/." "$APP/Contents/Resources/sample-pack/"

sed "s/VERSION_PLACEHOLDER/$VERSION/g" "$MACOS/packaging/Info.plist" > "$APP/Contents/Info.plist"
cp "$MACOS/packaging/Zyntra.entitlements" "$APP/Contents/Resources/Zyntra.entitlements"

# PkgInfo
printf 'APPL????' > "$APP/Contents/PkgInfo"

echo "==> built $APP"
echo "$APP"
