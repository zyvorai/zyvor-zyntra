#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
#
# Submit a DMG (or app zip) to Apple notarytool and staple on success.
#
# Auth (one of):
#   APPLE_ID + APPLE_TEAM_ID + APPLE_APP_SPECIFIC_PASSWORD
#   APP_STORE_CONNECT_API_KEY_P8 + APP_STORE_CONNECT_KEY_ID + APP_STORE_CONNECT_ISSUER_ID
set -euo pipefail

TARGET="${1:?usage: notarize.sh <path-to-dmg-or-zip>}"
if [[ ! -f "$TARGET" ]]; then
  echo "notarize: not a file: $TARGET" >&2
  exit 1
fi

AUTH_ARGS=()
if [[ -n "${APP_STORE_CONNECT_API_KEY_P8:-}" && -n "${APP_STORE_CONNECT_KEY_ID:-}" && -n "${APP_STORE_CONNECT_ISSUER_ID:-}" ]]; then
  KEY_DIR="$(mktemp -d)"
  KEY_PATH="$KEY_DIR/AuthKey_${APP_STORE_CONNECT_KEY_ID}.p8"
  printf '%s\n' "$APP_STORE_CONNECT_API_KEY_P8" > "$KEY_PATH"
  chmod 600 "$KEY_PATH"
  AUTH_ARGS=(
    --key "$KEY_PATH"
    --key-id "$APP_STORE_CONNECT_KEY_ID"
    --issuer "$APP_STORE_CONNECT_ISSUER_ID"
  )
  trap 'rm -rf "$KEY_DIR"' EXIT
elif [[ -n "${APPLE_ID:-}" && -n "${APPLE_TEAM_ID:-}" && -n "${APPLE_APP_SPECIFIC_PASSWORD:-}" ]]; then
  AUTH_ARGS=(
    --apple-id "$APPLE_ID"
    --team-id "$APPLE_TEAM_ID"
    --password "$APPLE_APP_SPECIFIC_PASSWORD"
  )
else
  echo "notarize: set Apple ID + app-specific password, or App Store Connect API key env vars" >&2
  exit 1
fi

echo "==> notarytool submit $TARGET"
xcrun notarytool submit "$TARGET" "${AUTH_ARGS[@]}" --wait

echo "==> staple $TARGET"
xcrun stapler staple "$TARGET"
xcrun stapler validate "$TARGET"
echo "==> notarized and stapled"
