---
sidebar_position: 4
---

# macOS app

Optional native client for Mac operators. Docker, Helm, systemd and the plain `zyntra` binary are unchanged; this is an extra install path.

The app is a SwiftUI shell (sidebar, Settings, menus) that starts the bundled engine (`Contents/MacOS/zyntra-engine`, same Go binary as `zyntra`) on `127.0.0.1` and shows the existing React console in WKWebView. Packs and the console itself are the same as the server product. The helper is named `zyntra-engine` so it does not collide with `Zyntra` on case-insensitive APFS.

## Install (release DMG)

1. Download `Zyntra-<version>-macos.dmg` from [GitHub Releases](https://github.com/zyvorai/zyvor-zyntra/releases).
2. Open the DMG and drag **Zyntra** to Applications.
3. Launch Zyntra. First run uses the bundled sample pack (`packs/shop` fixtures).
4. Sign in as `admin` / `Admin@321` when using the default API key `dev` (same as the quickstart). Set your own key in **Settings**.

If Gatekeeper blocks an unsigned local build, right-click → Open once, or build with Developer ID + notarization (below).

## Local build

Needs macOS with Xcode / Swift toolchain, Go, and Node 22 (for `make web`).

```bash
make mac-app          # → macos/build/Zyntra.app
open macos/build/Zyntra.app

make mac-dmg          # → macos/build/Zyntra-<version>-macos.dmg
```

`make mac-dmg` without a signing certificate ad-hoc signs and skips notarization (`SKIP_NOTARIZE=1`).

### Useful env vars

| Variable | Purpose |
|---|---|
| `VERSION` | CFBundle short version and DMG name (default: `var version` in `cmd/zyntra/main.go`) |
| `ARCHS` | Go/`lipo` architectures (default `arm64`; use `arm64 x86_64` for universal) |
| `SIGN_IDENTITY` | `codesign` identity (default: import from `MACOS_CERTIFICATE_P12`, else `-`) |
| `SKIP_NOTARIZE` | `1` to skip `notarytool` |

## Settings

| Setting | Behaviour |
|---|---|
| Pack directory | Same as `zyntra serve -f <dir>` |
| Bundled sample | Used when no pack path is set |
| API key | Stored in Keychain as `ZYNTRA_API_KEY` for the child process |
| Fixed port | `0` = pick a free localhost port each start |

State lives under `~/Library/Application Support/Zyntra/`.

## Notarized release (maintainers)

GitHub Actions workflow: [`macos-release.yml`](https://github.com/zyvorai/zyvor-zyntra/blob/main/.github/workflows/macos-release.yml) (`workflow_dispatch` or published Release).

### Required secrets

**Signing**

| Secret | Value |
|---|---|
| `MACOS_CERTIFICATE_P12` | Base64-encoded Developer ID Application `.p12` |
| `MACOS_CERTIFICATE_PASSWORD` | Password for that `.p12` |

**Notary (either Apple ID or API key)**

| Secret | Value |
|---|---|
| `APPLE_ID` | Apple ID email |
| `APPLE_TEAM_ID` | Team ID |
| `APPLE_APP_SPECIFIC_PASSWORD` | App-specific password |

or

| Secret | Value |
|---|---|
| `APP_STORE_CONNECT_API_KEY_P8` | Contents of the `.p8` key |
| `APP_STORE_CONNECT_KEY_ID` | Key ID |
| `APP_STORE_CONNECT_ISSUER_ID` | Issuer UUID |

Encode the certificate:

```bash
base64 -i DeveloperID.p12 | pbcopy   # macOS → MACOS_CERTIFICATE_P12
```

Without `MACOS_CERTIFICATE_P12`, the workflow still builds an ad-hoc DMG artifact and skips notarization.

### Local notarized DMG

```bash
export SIGN_IDENTITY="Developer ID Application: Example Corp (TEAMID)"
export APPLE_ID=...
export APPLE_TEAM_ID=...
export APPLE_APP_SPECIFIC_PASSWORD=...
make mac-dmg
```

Scripts: [`macos/scripts/build-app.sh`](https://github.com/zyvorai/zyvor-zyntra/tree/main/macos/scripts/build-app.sh), [`build-dmg.sh`](https://github.com/zyvorai/zyvor-zyntra/tree/main/macos/scripts/build-dmg.sh), [`notarize.sh`](https://github.com/zyvorai/zyvor-zyntra/tree/main/macos/scripts/notarize.sh).
