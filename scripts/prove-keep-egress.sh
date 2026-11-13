#!/usr/bin/env bash
# Prove Keep agent egress is configured (and optionally that a session reaches the broker).
#   KEEP_ENV=/etc/zyvor-fabricd/zyvor-fabric-agent.env ./scripts/prove-keep-egress.sh
# With ZYNTRA_URL + token, also checks recent audit is not stuck on "zyntra (keep unavailable)".
set -euo pipefail
KEEP_ENV="${KEEP_ENV:-/etc/zyvor-fabricd/zyvor-fabric-agent.env}"
KEEP_URL="${ZYNTRA_KEEP_URL:-http://127.0.0.1:9096}"

pass=0
ok() { echo "ok   $*"; pass=$((pass + 1)); }
bad() { echo "FAIL $*"; exit 1; }

[[ -r "$KEEP_ENV" ]] || bad "$KEEP_ENV missing"
adv=$(sed -n 's/^ZYVOR_AGENT_EGRESS_ADVERTISE_HOST=//p' "$KEEP_ENV" | head -1)
[[ -n "$adv" ]] && ok "advertise host=$adv" || bad "ZYVOR_AGENT_EGRESS_ADVERTISE_HOST unset — re-run deploy / configure_keep"
creds=$(sed -n 's/^ZYVOR_AGENT_CREDENTIALS_FILE=//p' "$KEEP_ENV" | head -1)
[[ -n "$creds" && -r "$creds" ]] && ok "credentials file $creds" || bad "credentials missing"
grep -q 'zyntra-exec' "$creds" 2>/dev/null && ok "zyntra-exec credential present" || echo "warn zyntra-exec not in credentials"

code=$(curl -s -m 3 -o /dev/null -w '%{http_code}' "$KEEP_URL/healthz" || true)
[[ "$code" == 200 ]] && ok "Keep healthy at $KEEP_URL" || bad "Keep not healthy ($code) at $KEEP_URL"

if [[ -n "${ZYNTRA_URL:-}" ]]; then
  tok="${ZYNTRA_TOKEN:-}"
  [[ -n "$tok" ]] || bad "ZYNTRA_URL set but ZYNTRA_TOKEN missing"
  # Prefer evidence that Keep path ran; do not fail the config check if lab still falls back.
  aud=$(curl -fsS -m 10 -H "Authorization: Bearer $tok" "$ZYNTRA_URL/api/v1/audit?limit=20" 2>/dev/null || true)
  if echo "$aud" | grep -q 'zyntra (keep unavailable)'; then
    echo "warn recent audit still shows 'zyntra (keep unavailable)' — guest may still lack a route to the broker"
    echo "     fix: tap+netns + dnsmasq, or confirm $adv is reachable from the node22-agent guest"
  elif echo "$aud" | grep -qi keep; then
    ok "audit mentions Keep (no keep-unavailable label in last page)"
  else
    echo "warn no recent Keep-related audit lines (run an approve under ZYNTRA_APPROVAL_MODE=keep)"
  fi
fi

echo "config checks passed ($pass). Lab prove: approve one action and confirm audit by≠'zyntra (keep unavailable)'."
