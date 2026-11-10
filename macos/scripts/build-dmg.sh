#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
#
# Build, codesign (Developer ID), package DMG, notarize, staple.
# Skip notarization when SKIP_NOTARIZE=1 (local unsigned/adhoc builds set SIGN_IDENTITY=-).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MACOS="$ROOT/macos"
BUILD="$MACOS/build"
VERSION="${VERSION:-}"
if [[ -z "$VERSION" ]]; then
  VERSION="$(sed -n 's/^var version = "\(.*\)"/\1/p' "$ROOT/cmd/zyntra/main.go")"
fi
ARCHS="${ARCHS:-arm64}"
SIGN_IDENTITY="${SIGN_IDENTITY:-}"
SKIP_NOTARIZE="${SKIP_NOTARIZE:-0}"
ENTITLEMENTS="$MACOS/packaging/Zyntra.entitlements"

echo "==> build app"
ARCHS="$ARCHS" VERSION="$VERSION" "$MACOS/scripts/build-app.sh"
APP="$BUILD/Zyntra.app"

codesign_all() {
  local identity="$1"
  echo "==> codesign ($identity)"
  # Nested helper first, then the app bundle.
  codesign --force --options runtime --timestamp \
    --sign "$identity" \
    "$APP/Contents/MacOS/zyntra-engine"
  codesign --force --options runtime --timestamp \
    --entitlements "$ENTITLEMENTS" \
    --sign "$identity" \
    "$APP"
  codesign --verify --deep --strict --verbose=2 "$APP"
  spctl --assess --type execute --verbose=4 "$APP" || true
}

if [[ -z "$SIGN_IDENTITY" ]]; then
  if [[ -n "${MACOS_CERTIFICATE_P12:-}" && -n "${MACOS_CERTIFICATE_PASSWORD:-}" ]]; then
    echo "==> importing Developer ID certificate from env"
    CERT_DIR="$(mktemp -d)"
    CERT_PATH="$CERT_DIR/cert.p12"
    printf '%s' "$MACOS_CERTIFICATE_P12" | base64 --decode > "$CERT_PATH"
    KEYCHAIN="$CERT_DIR/zyntra.keychain-db"
    KEYCHAIN_PW="$(openssl rand -hex 16)"
    security create-keychain -p "$KEYCHAIN_PW" "$KEYCHAIN"
    security set-keychain-settings -lut 21600 "$KEYCHAIN"
    security unlock-keychain -p "$KEYCHAIN_PW" "$KEYCHAIN"
    security import "$CERT_PATH" -P "$MACOS_CERTIFICATE_PASSWORD" -A -t cert -f pkcs12 -k "$KEYCHAIN"
    security list-keychain -d user -s "$KEYCHAIN" $(security list-keychain -d user | sed -e s/\"//g)
    security set-key-partition-list -S apple-tool:,apple: -s -k "$KEYCHAIN_PW" "$KEYCHAIN"
    SIGN_IDENTITY="$(security find-identity -v -p codesigning "$KEYCHAIN" | awk -F'"' '/Developer ID Application/{print $2; exit}')"
    if [[ -z "$SIGN_IDENTITY" ]]; then
      echo "build-dmg: no Developer ID Application identity in imported keychain" >&2
      exit 1
    fi
    trap 'security delete-keychain "$KEYCHAIN" 2>/dev/null || true; rm -rf "$CERT_DIR"' EXIT
  else
    echo "==> no SIGN_IDENTITY / certificate; ad-hoc sign for local testing"
    SIGN_IDENTITY="-"
    SKIP_NOTARIZE=1
  fi
fi

codesign_all "$SIGN_IDENTITY"

DMG_NAME="Zyntra-${VERSION}-macos.dmg"
DMG_PATH="$BUILD/$DMG_NAME"
STAGE="$BUILD/dmg-stage"
rm -rf "$STAGE" "$DMG_PATH"
mkdir -p "$STAGE"
cp -R "$APP" "$STAGE/Zyntra.app"
ln -s /Applications "$STAGE/Applications"

echo "==> create DMG $DMG_PATH"
hdiutil create -volname "Zyntra" -srcfolder "$STAGE" -ov -format UDZO "$DMG_PATH"

if [[ "$SIGN_IDENTITY" != "-" ]]; then
  codesign --force --timestamp --sign "$SIGN_IDENTITY" "$DMG_PATH"
fi

CHECKSUM="$DMG_PATH.sha256"
shasum -a 256 "$DMG_PATH" | awk '{print $1}' > "$CHECKSUM"
echo "sha256: $(cat "$CHECKSUM")"

if [[ "$SKIP_NOTARIZE" != "1" ]]; then
  "$MACOS/scripts/notarize.sh" "$DMG_PATH"
else
  echo "==> skipping notarization (SKIP_NOTARIZE=1 or ad-hoc sign)"
fi

echo "==> artifacts"
ls -la "$DMG_PATH" "$CHECKSUM"
echo "$DMG_PATH"
