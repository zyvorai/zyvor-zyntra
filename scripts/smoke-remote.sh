#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
# smoke-remote.sh — Verify a running Zyntra instance (local or remote)
# ============================================================================
# Checks health, console, auth, live sources, decisions, AI, analytics,
# analytics queries, document knowledge, and a full propose → approve →
# execute round trip (through Fabric Keep when the instance runs keep
# approvals). Execution is whatever the instance is set to (dry-run by
# default: kubectl --dry-run=server).
#
# Sources the host has no credentials for (state "fallback") are reported and
# skipped. Typed actions get their required inputs from the ontology. A
# proposal blocked only because its stale inputs come from those skipped
# sources counts as a pass, like one held for its maintenance window.
#
# Usage:
#   ZYNTRA_URL=http://212.8.248.187:19620 ./scripts/smoke-remote.sh
#   ./scripts/smoke-remote.sh            # HOST/PORT from .deploy-last
#
# The access key comes from ZYNTRA_API_KEY, else it is read over SSH from
# /etc/zyntra/zyntra.env on the deploy host (never printed).
#
# Options:
#   --action ID   action to propose (default: raise-inference-priority when the
#                 model has it, else the top approvable single action)
#   --no-exec     skip the propose/approve round trip
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ACTION="auto"
DO_EXEC=1
while [ $# -gt 0 ]; do
  case "$1" in
    --action) [ $# -ge 2 ] || { echo "--action requires a value" >&2; exit 2; }; ACTION="$2"; shift 2 ;;
    --no-exec) DO_EXEC=0; shift ;;
    --help|-h) sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 2 ;;
  esac
done

last() { awk -F= -v k="$1" '$1 == k {print $2; exit}' "$ROOT/.deploy-last" 2>/dev/null || true; }
LAST_HOST="$(last HOST)"
LAST_PORT="$(last PORT)"
SSH_HOST="${DEPLOY_HOST:-$LAST_HOST}"
SSH_USER="${DEPLOY_USER:-$(last USER)}"
BASE="${ZYNTRA_URL:-}"
if [ -z "$BASE" ] && [ -n "$LAST_HOST" ] && [ -n "$LAST_PORT" ]; then
  BASE="http://${LAST_HOST}:${LAST_PORT}"
fi
[ -n "$BASE" ] || { echo "Set ZYNTRA_URL=http://host:port (or deploy first for .deploy-last)" >&2; exit 2; }
BASE="${BASE%/}"

if [ -z "${ZYNTRA_API_KEY:-}" ] && [ -n "$SSH_HOST" ]; then
  ZYNTRA_API_KEY="$(ssh -o BatchMode=yes -o ConnectTimeout=10 "${SSH_USER:+$SSH_USER@}$SSH_HOST" \
    "sudo sh -c '. /etc/zyntra/zyntra.env && printf %s \"\$ZYNTRA_API_KEY\"'" 2>/dev/null | tr -d '\r' || true)"
fi
[ -n "${ZYNTRA_API_KEY:-}" ] || { echo "Set ZYNTRA_API_KEY (could not read it from the host)" >&2; exit 2; }

echo "Zyntra smoke → ${BASE}"
ZYNTRA_API_KEY="$ZYNTRA_API_KEY" python3 - "$BASE" "$ACTION" "$DO_EXEC" <<'PY'
import http.cookiejar, json, os, sys, time, urllib.error, urllib.request

base, action, do_exec = sys.argv[1], sys.argv[2], sys.argv[3] == "1"
jar = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))

def ok(msg): print(f"  ✅ {msg}")
def fail(msg):
    print(f"  ❌ {msg}", file=sys.stderr)
    sys.exit(1)

def call(method, path, body=None, auth=True, raw=False):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(base + path, data=data, method=method,
                                 headers={"Content-Type": "application/json"} if data else {})
    o = opener if auth else urllib.request.build_opener()
    try:
        with o.open(req, timeout=30) as r:
            b = r.read()
            return r.status, (b.decode() if raw else json.loads(b or b"null"))
    except urllib.error.HTTPError as e:
        b = e.read()
        try:
            return e.code, json.loads(b)
        except ValueError:
            return e.code, b.decode(errors="replace")

c, h = call("GET", "/healthz", auth=False)
if c != 200 or h.get("status") != "ok":
    fail(f"healthz HTTP {c}: {h}")
ok("healthz")

c, html = call("GET", "/", auth=False, raw=True)
if c != 200 or '<div id="root">' not in html:
    fail(f"console HTTP {c}")
ok("console served")

c, meta = call("GET", "/api/v1/meta", auth=False)
if c != 200 or not meta.get("auth_required"):
    fail(f"meta HTTP {c} or auth not required: {meta}")
pack = (meta.get("pack") or {}).get("id", "-")
ok(f"meta: v{meta['version']} · {meta['model']} · pack {pack} · approvals {meta['approval_mode']} · execute {meta['execute_mode']} · AI {meta['ai_mode']}")

c, _ = call("GET", "/api/v1/gaps", auth=False)
if c != 401:
    fail(f"unauthenticated gaps returned {c}, want 401")
c, _ = call("POST", "/api/v1/session", {"operator": "smoke", "token": "wrong-key"})
if c != 401:
    fail(f"wrong key returned {c}, want 401")
ok("auth rejects anonymous and wrong key")

c, s = call("POST", "/api/v1/session", {"operator": "smoke", "token": os.environ["ZYNTRA_API_KEY"]})
if c != 200:
    fail(f"login HTTP {c}: {s}")
c, who = call("GET", "/api/v1/whoami")
if c != 200 or who.get("identity", {}).get("subject") != "smoke":
    fail(f"whoami {c}: {who}")
ok("session cookie login as smoke")

c, inp = call("GET", "/api/v1/inputs")
for m in (inp or {}).get("manual") or []:
    if not m.get("entry"):
        c, r = call("POST", f"/api/v1/kpis/{m['kpi']}/value", {"value": m["value"], "reason": "smoke test: confirm declared value"})
        if c != 200:
            fail(f"manual value {m['kpi']} HTTP {c}: {r}")
        ok(f"manual value {m['kpi']} = {m['value']} recorded (audited)")

deadline = time.time() + 60
while True:
    c, src = call("GET", "/api/v1/sources")
    sources = src.get("sources") or []
    if sources or time.time() > deadline:
        break
    time.sleep(3)
skipped = [x for x in sources if not x["ok"] and x.get("state") == "fallback"]
bad = [x for x in sources if not x["ok"] and x.get("state") != "fallback"]
for x in sources:
    print(f"     {'●' if x['ok'] else '○'} {x['name']:<8} {x['kind']:<8} {x['latency_ms']:>4} ms  {len(x['kpis'])} KPIs" + (f"  {x.get('error','')}" if not x["ok"] else ""))
healthy = len(sources) - len(skipped) - len(bad)
if not sources or bad or not healthy:
    fail(f"{len(bad)}/{len(sources)} sources unhealthy, {healthy} healthy")
ok(f"{healthy}/{len(sources)} sources healthy" + (f", {len(skipped)} not configured on this host: {', '.join(x['name'] for x in skipped)}" if skipped else ""))
unconfigured_kpis = {k for x in skipped for k in x.get("kpis") or []}

c, g = call("GET", "/api/v1/gaps")
if c != 200:
    fail(f"gaps HTTP {c}")
ok(f"gaps: {len(g['gaps'])} KPIs off target (severity {g['severity_total']:.2f})")
c, plan = call("GET", "/api/v1/plan")
if c != 200:
    fail(f"plan HTTP {c}")
ok(f"plan: {len(plan['recommendations'])} ranked actions")
if action == "auto":
    recs = plan["recommendations"]
    ids = [r["action"] for r in recs]
    if "raise-inference-priority" in ids:
        action = "raise-inference-priority"
    else:
        _, g = call("GET", "/api/v1/graph")
        acts = {x.get("id"): x for x in ((g or {}).get("model") or {}).get("actions") or []}
        windowed = lambda a: bool(acts.get(a, {}).get("window") or (acts.get(a, {}).get("policy") or {}).get("maintenance_windows"))
        pick = [r["action"] for r in recs if r["status"] == "pending-approval" and "+" not in r["action"]]
        pick.sort(key=windowed)
        if not pick and do_exec:
            fail("no approvable single action in the plan to smoke-test")
        action = pick[0] if pick else ""
    if do_exec:
        print(f"     smoke action: {action}")

c, d = call("GET", "/api/v1/ai/digest")
if c != 200 or not d.get("text"):
    fail(f"AI digest {c}: {d}")
ok(f"AI digest ({d.get('mode')}): {d['text'][:90]}…")
c, a = call("POST", "/api/v1/ai/ask", {"question": "what should I fix first?"})
if c != 200 or not a.get("text"):
    fail(f"AI ask {c}: {a}")
ok("AI ask answered")

c, ins = call("GET", "/api/v1/ai/insights")
an = (ins or {}).get("analytics") if c == 200 else None
if not isinstance(an, dict):
    fail(f"insights analytics HTTP {c}: missing analytics report")
if an.get("persistence_error"):
    fail(f"analytics history not persisting: {an['persistence_error']}")
counts = {}
for k in (an.get("kpis") or {}).values() if isinstance(an.get("kpis"), dict) else an.get("kpis") or []:
    counts[k.get("status")] = counts.get(k.get("status"), 0) + 1
ok(f"analytics: persistent {an.get('persistent')} · {', '.join(f'{n} {s}' for s, n in sorted(counts.items())) or 'no KPIs'} · {len(an.get('anomalies') or [])} robust anomalies")

c, cat = call("GET", "/api/v1/analytics/catalog")
metrics = [m["id"] for m in (cat or {}).get("metrics") or []] if c == 200 else []
if not metrics:
    fail(f"analytics catalog HTTP {c}: {cat}")
c, q = call("POST", "/api/v1/analytics/query", {"metrics": metrics[:1], "operation": "mean", "window_hours": 24})
if c != 200:
    fail(f"analytics query HTTP {c}: {q}")
row = (q.get("rows") or [{}])[0]
ok(f"analytics query: {len(metrics)} metrics · 24h mean {row.get('id')} = {row.get('value')} from {row.get('samples')} samples")
c, q = call("POST", "/api/v1/analytics/query", {"metrics": metrics[:1], "operation": "mean", "window_hours": 24, "bucket_hours": 6})
if c != 400:
    fail(f"invalid analytics query returned {c}, want 400")
c, a = call("POST", "/api/v1/ai/ask", {"question": f"average {metrics[0]} over the last 24 hours", "scope": "analytics"})
if c != 200 or not a.get("text"):
    fail(f"Ask analytics {c}: {a}")
ok("Ask scope=analytics answered" + (" with a validated query" if a.get("analytics_query") else " (guidance)"))

c, inv = call("POST", "/api/v1/analytics/investigate", {"metric": metrics[0], "window_hours": 168, "recent_hours": 6, "max_lag_hours": 6})
if c != 200 or not isinstance(inv, dict):
    fail(f"investigation HTTP {c}: {inv}")
shift = inv.get("shift") or {}
cmp = inv.get("comparisons") or []
assoc = sum(1 for x in cmp if x.get("status") == "associated")
ok(f"investigation {metrics[0]}: shift {shift.get('status')}{' (' + shift['reason'] + ')' if shift.get('reason') else ''} · "
   f"{len(inv.get('hours') or [])} observed hours · {len(cmp)}/{inv.get('candidate_total')} candidates compared, {assoc} associated")
c, r = call("POST", "/api/v1/analytics/investigate", {"metric": metrics[0], "window_hours": 168, "recent_hours": 6, "max_lag_hours": 6, "candidates": metrics[:1]})
if c != 400:
    fail(f"investigation with target as candidate returned {c}, want 400")
c, a = call("POST", "/api/v1/ai/ask", {"question": f"investigate {metrics[0]}", "scope": "investigation"})
if c != 200 or not a.get("text"):
    fail(f"Ask investigation {c}: {a}")
ok("Ask scope=investigation answered")

doc = f"smoke-{int(time.time())}"
c, r = call("PUT", f"/api/v1/knowledge/documents/{doc}", {"expected_version": 0, "document": {
    "title": "Smoke test note", "source": "smoke-remote.sh", "visibility": "shared",
    "roles": ["viewer", "proposer", "approver"],
    "text": "Reorder zyntrasmoke widgets when stock falls below ten units."}})
if c not in (200, 201):
    fail(f"knowledge create HTTP {c}: {r}")
try:
    c, s = call("GET", "/api/v1/knowledge/search?q=zyntrasmoke&limit=3")
    if c != 200 or not ((s or {}).get("results") or (s or {}).get("citations")):
        fail(f"knowledge search HTTP {c}: {s}")
    c, a = call("POST", "/api/v1/ai/ask", {"question": "When should zyntrasmoke widgets be reordered?", "scope": "documents"})
    if c != 200 or not a.get("document_citations"):
        fail(f"Ask documents {c}: {a}")
    ok(f"knowledge: create, search and cited Ask ({len(a['document_citations'])} citation)")
finally:
    c, r = call("DELETE", f"/api/v1/knowledge/documents/{doc}", {"expected_version": 1})
    if c not in (200, 204):
        print(f"  ⚠️  could not delete smoke document {doc}: HTTP {c}", file=sys.stderr)

c, ks = call("GET", "/api/v1/keep/status")
if c == 200 and ks.get("configured"):
    st = ks.get("status") or {}
    if ks.get("error"):
        fail(f"Keep status error: {ks['error']}")
    k = st.get("keep") or {}
    signers = k.get("trusted_signers")
    ok(f"Keep: mode {k.get('keep_mode')} · signers {len(signers) if isinstance(signers, list) else signers} · sandbox ready {(k.get('fluxvm') or {}).get('ready')} · agent {st.get('agent_version') if st.get('agent_deployed') else 'not deployed'}")

if not do_exec:
    print("  ✨ smoke OK (no exec)")
    sys.exit(0)

def prop_value(obj, name):
    v = (obj.get("props") or {}).get(name)
    return str(v.get("v")) if isinstance(v, dict) else (None if v is None else str(v))

def action_inputs(action_id):
    c, schema = call("GET", "/api/v1/ontology/schema")
    if c != 200:
        return {}
    spec = next((a for a in (schema or {}).get("actions") or [] if a.get("id") == action_id), None)
    inputs = {}
    for inp in (spec or {}).get("inputs") or []:
        if not inp.get("required"):
            continue
        c, objs = call("GET", f"/api/v1/ontology/objects?type={inp['object_type']}")
        reqs = [r for r in spec.get("requires") or [] if r.get("input") == inp["name"]]
        match = next((o["id"] for o in (objs or {}).get("objects") or []
                      if all((not r.get("equals") or prop_value(o, r["property"]) == r["equals"]) and
                             (not r.get("not_equals") or prop_value(o, r["property"]) != r["not_equals"]) for r in reqs)), None)
        if not match:
            fail(f"{action_id} needs a {inp['object_type']} for input {inp['name']!r} meeting {reqs}; none found")
        inputs[inp["name"]] = match
    return inputs

body = {"action": action}
inputs = action_inputs(action)
if inputs:
    body["inputs"] = inputs
    print("     inputs: " + ", ".join(f"{k}={v}" for k, v in inputs.items()))
c, p = call("POST", "/api/v1/proposals", body)
if c not in (200, 201):
    fail(f"propose {action} HTTP {c}: {p}")
if p.get("render_error"):
    fail(f"render error: {p['render_error']}")
pid = p["id"]
ok(f"proposed {action} → {pid} ({', '.join(p.get('kinds') or []) or p.get('template') or 'advisory'})")
c, p = call("POST", f"/api/v1/proposals/{pid}/approve", {"reason": "smoke test"})
if c not in (200, 202):
    fail(f"approve HTTP {c}: {p}")
ok(f"approved (HTTP {c}, status {p.get('status')})")

deadline = time.time() + 240
while p.get("status") not in ("executed", "failed", "blocked") and not p.get("waiting_for_window") and time.time() < deadline:
    time.sleep(3)
    c, p = call("GET", f"/api/v1/proposals/{pid}")
ex = p.get("execution") or {}
keep = p.get("keep") or {}
if keep:
    print(f"     keep: mode {keep.get('mode')} session {keep.get('session_id','-')} approval {keep.get('approval_id','-')}" + (f" error {keep['error']}" if keep.get("error") else ""))
if ex.get("output"):
    print("     " + ex["output"].strip().replace("\n", "\n     "))
if p.get("waiting_for_window"):
    ok(f"held for its maintenance window {(p.get('policy') or {}).get('maintenance_windows')} (approved, not run)")
    print("  ✨ smoke OK")
    sys.exit(0)
stale = set((p.get("revalidation") or {}).get("stale_inputs") or [])
if p.get("status") == "blocked" and stale and stale <= unconfigured_kpis:
    ok(f"blocked by revalidation: stale inputs {', '.join(sorted(stale))} come from sources not configured on this host (approved, not run)")
    print("  ✨ smoke OK")
    sys.exit(0)
if p.get("status") != "executed":
    if p.get("blocked_reasons"):
        fail(f"proposal blocked: {'; '.join(p['blocked_reasons'])}")
    fail(f"proposal ended {p.get('status')}: {ex.get('error') or keep.get('error') or 'timeout'}")
ok(f"executed via {ex.get('kind') or 'kubectl'} ({ex.get('mode')})")

c, au = call("GET", "/api/v1/audit")
trail = [e for e in au.get("events", []) if e.get("proposal") == pid]
print("     audit: " + " → ".join(f"{e['to']}({e['by']})" for e in trail))
if not any(e["to"] == "executed" for e in trail):
    fail("audit trail missing the executed event")
ok("audit trail recorded")
if keep.get("mode") == "keep":
    c, ka = call("GET", f"/api/v1/keep/audit?session_id={keep.get('session_id','')}")
    n = len((ka or {}).get("items") or (ka or {}).get("entries") or []) if c == 200 else 0
    ok(f"Keep audit reachable ({n} entries)") if c == 200 else fail(f"Keep audit HTTP {c}: {ka}")
print("  ✨ smoke OK")
PY
