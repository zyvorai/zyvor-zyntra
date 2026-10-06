---
sidebar_position: 1
---

# The model and packs

## What to know before you rely on it

- **The simulator is a model, not a measurement.** It is a deterministic model over the edge weights and effects you supply. Its ranges come from the uncertainty you declare, not from data. `zyntra calibrate` backtests the edge weights against decisions that ran, and suggests a corrected direct effect for an action from decisions that ran it alone. It suggests corrections but never applies them.
- **The newer packs carry declared starting weights**, not measured ones. Calibrate them against your own outcomes before you lean on the ranking.
- **Changes execute only after human approval**, and in dry-run by default.
- **Sources only read.** The AI layer explains and forecasts; it never picks or runs an action.
- **Release state.** Released versions are listed on the [GitHub releases page](https://github.com/zyvorai/zyntra/releases); anything newer is unreleased, so check it before you pin a version.

## The model

A model file ([examples/kpis.yaml](https://github.com/zyvorai/zyvor-zyntra/blob/main/examples/kpis.yaml)) has three parts:

```yaml
kpis:
  - {id: capacity_headroom, unit: "%", owner: platform, value: 8, target: 20, direction: higher}
edges:      # +10% in `from` causes weight * 10% in `to`; must be acyclic
  - {from: capacity_headroom, to: queue_wait_minutes, weight: -0.8, why: free GPUs drain the queue}
actions:
  - id: preempt_batch_to_spot
    adapter: zynera
    risk: medium
    effects: [{kpi: capacity_headroom, change: 0.8}]
```

- **Gap severity** is the relative shortfall against target (`0.4` = 40% off), multiplied by the KPI's criticality weight (`critical` 4, `high` 2, `normal` 1, `low` 0.5). The plan minimises the weighted sum.
- **Propagation** runs in topological order with interval arithmetic: every effect and edge carries a low/nominal/high band, so predictions come with a range. Effects are relative (a fraction) or `mode: absolute` (in the KPI's unit, so a KPI at zero can move), can `saturate` and take a `delay`. Results are clamped to `min`/`max`; KPIs without bounds can't drop below zero.
- **Hard constraints** (`constraints:`) set a floor, ceiling or `mustNotWorsen` on a KPI. Critical KPIs with a target are constraints automatically. An action that would breach one, even at the pessimistic end of its band, is listed as blocked instead of ranked.
- **Score** = weighted gap reduction in the **pessimistic** case (every edge and effect at the bad end of its declared uncertainty) − risk penalty (low 0, medium 0.05, high 0.15) − 0.1 per stale input. `plan` shows both the nominal gain and the worst case. An action that only wins when every edge holds is marked *optimistic only* and ranks below the ones that win anyway. Zyntra also tries pairs of actions that touch different KPIs and keeps a pair only if it beats both actions alone; a pair whose members push the same KPI in opposite directions (by 1% or more) is flagged *works against itself* and drops to medium confidence. Each recommendation has a confidence (high, medium, low).
- **Preconditions and invariants.** An action whose precondition does not hold (or whose input is stale) is ranked after the approvable ones with status `precondition-failed` and cannot be proposed. An action whose simulation worsens an invariant KPI by more than `max_worsen`, at the nominal or pessimistic end, is blocked like a constraint breach.

## Model reference (core)

```yaml
kpis:
  - id: app_availability
    value: 99.95
    target: 99.9
    direction: higher
    criticality: critical         # weights severity; critical + target = hard floor
    min: 0
    max: 100                      # results are clamped to bounds
    freshness: {maxAge: 1m, required: true}   # required: block execution when stale
constraints:
  - {kpi: host_memory_percent, ceiling: 90, why: OOM above 90%}
  - {kpi: gravia_available_gpus, mustNotWorsen: true}
edges:
  - {from: queue, to: latency, weight: 0.05, confidence: 0.6, provenance: learned, delay: 5m}
actions:
  - id: raise-inference-priority
    effects:
      - {kpi: gpu_queue_wait_min, change: -0.35, uncertainty: 0.3, delay: 5m}
      - {kpi: gpu_utilization, change: 10, mode: absolute, saturation: 30}
    execute: {template: gravia.priority, params: {name: inference-critical, value: "900000"}}
    rollback: {template: gravia.priority-delete, params: {name: inference-critical}}  # optional; derived for Gravia templates
    outcome:                       # how to judge the action after it runs
      window: 20m
      samples: 3                   # consecutive fresh samples that must pass
      successCriteria: [{kpi: gpu_queue_wait_min, op: met}]   # met | < | <= | > | >= with value
      guardrails: [app_availability]
      tolerance: 0.05
    policy: {approvals: 2, keep: required, requireFresh: true, maintenanceWindows: [weeknights]}
```

## Model reference (packs and ontology additions)

```yaml
timezone: Asia/Kolkata            # default for calendars (a pack sets it in pack.yaml)
calendar: shop-hours              # default calendar for every KPI
calendars:                        # weekly windows; end before start wraps midnight
  shop-hours: {start: "09:00", end: "21:30"}
  buy-hours: {days: [mon, tue, wed, thu, fri, sat], start: "10:00", end: "17:00"}
kpis:
  - id: daily_sales
    unitClass: currency           # percent | count | currency | duration | ratio
    currency: INR                 # ISO code; shown as the unit
    calendar: shop-hours          # outside the window the last in-window value is held
    source: {kind: file, file: fixture/pos.csv, field: "*.amount"}
  - id: cashiers_open
    source: {kind: manual, staleAfter: 12h}    # entered in the console, audited
  - id: line_temp
    source: {kind: webhook-in, name: plant-gateway, field: temp_c, staleAfter: 10m}
actions:
  - id: markdown_capped
    title: Mark down dead stock 10%   # title is an alias for name
    adapter: webhook                  # webhook | file | noop, or a core adapter with execute
    window: evening                   # approved runs wait for this calendar window
    approvers: 2                      # raises the policy quorum for this action
    preconditions: [{kpi: stockout_rate, worse_than: 0.02}]   # or better_than
    invariants: [{kpi: gross_margin, max_worsen: 0.03}]       # relative fraction
    compensate: reverse_markdown      # linked undo, offered as the rollback; never auto-run
    webhook:
      method: POST
      url: ${ZYNTRA_POS_URL}/markdowns
      headers: {Authorization: "Bearer ${ZYNTRA_POS_TOKEN}"}
      body: {percent: 10, gap: "gap:dead_stock_days", now: "kpi:dead_stock_days"}
  - id: reorder_fast_movers
    adapter: file
    file:
      path: "po/{{.Stamp}}-reorder.md"    # relative, inside ZYNTRA_OUTPUT_DIR
      content: |
        Stockout {{pct (index .KPIs "stockout_rate").Value}} on {{.Date}}
```

| Source field | Meaning |
|---|---|
| `kind` | `file`, `http`, `sheet`, `webhook-in`, `manual`, or a core kind (`prometheus`, `kubernetes`, `metrics`, `json`, `netra`, `gravia`, `fabric`, `keep`) |
| `file` / `url` | Path relative to the model (or pack) directory / URL; `${ZYNTRA_*}` is expanded |
| `format` | `csv`, `json`, `yaml` or `prometheus`; guessed from the extension or content type |
| `field` | Field path into the document, as for `json` sources; `*.amount` collects a column |
| `where` | Keep only rows whose columns equal these values |
| `agg` | `sum` (default), `avg`, `min`, `max`, `count`, `first`, `last` |
| `denominator` | Second field path; the KPI is `field / denominator` |
| `headers` | Request headers for `http` and `sheet` |
| `name` | The `webhook-in` channel (`POST /api/v1/ingest/<name>`) |
| `staleAfter` | Age after which a `webhook-in` or `manual` value is stale |

Webhook bodies resolve `kpi:<id>` to the live value and `gap:<id>` to the gap (value, target, severity). File content is a Go template with `.Action`, `.KPIs`, `.Gaps`, `.Date`, `.Time` and `.Stamp`, plus `pct` and `num`; a missing key is an error, not an empty string.

Edge `confidence` below 1 widens the band; `provenance: learned` marks the relationship as assumed in the trace. A KPI's freshness defaults to three refresh intervals; values older than that are `stale`, never-fetched ones `missing`, and KPIs without a source `static`.

## Live values

Add a `source` to any KPI and pass the adapter flags:

```yaml
- id: p99_inference_latency
  value: 420            # fallback
  target: 300
  direction: lower
  source:
    kind: prometheus
    query: 1000 * histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{job="inference"}[5m])) by (le))
- id: gpu_nodes
  value: 8
  source: {kind: kubernetes, metric: nodes_ready}   # nodes_total | nodes_ready | gpu_allocatable | cpu_allocatable
```

```bash
zyntra gaps -f examples/prometheus-kpis.yaml -prometheus http://prometheus:9090 -kubectl
```

## Live sources (Netra, Gryvia, Fabric, Keep)

`kind: metrics` scrapes Prometheus text (Netra's `/metrics`). `kind: json` reads a field from a JSON endpoint; `kind: netra`, `gravia`, `fabric` and `keep` (the `gravia` kind reads Gryvia) are shorthands that also pick the endpoint. Zyntra ships no eBPF code of its own; it consumes the Netra agent.

```yaml
- id: tcp_retransmits_per_s
  source: {kind: metrics, endpoint: netra, metric: netra_tcp_retransmissions, agg: sum, rate: true}
- id: ebpf_health_score
  source: {kind: netra, path: /api/v1/ebpf/health, field: summary.healthScore}
- id: root_disk_percent
  source: {kind: fabric, path: /api/v1/system/info, field: "filesystems.#(mountpoint=/).usage_percent"}
```

Fields support `a.b.0`, `list.#` (count), `list.#(k=v)` (count matches), `list.#(k=v).f` (field of the first match) and `list.*.f`, plus `scale`, `agg: sum|avg|max|min` and `rate`. See [packs/gpu](https://github.com/zyvorai/zyvor-zyntra/blob/main/packs/gpu) for a full lab model (20 KPIs) that uses every v0.3 field.

## Packs (any industry)

The engine knows nothing about GPUs or shops. A **pack** is a directory of files: `pack.yaml` (id, owners, timezone, calendars), `kpis.yaml`, `sources.example.yaml`, a README and a `fixture/` of sample exports. [packs/shop](https://github.com/zyvorai/zyvor-zyntra/blob/main/packs/shop) runs a shop from CSV exports with no Kubernetes in the loop:

```bash
zyntra pack list
zyntra pack validate packs/shop
zyntra plan -f packs/shop                                   # ranks a reorder, refuses a markdown that breaks the margin invariant
zyntra simulate -f packs/shop -action reorder_fast_movers   # prints the dry-run purchase order
zyntra gaps -f packs/shop -owner floor
```

**Generic sources.** `file` (CSV, JSON, YAML or Prometheus text, reloaded on change), `http` and `sheet` (the same over HTTP), `webhook-in` (a gateway POSTs JSON to `/api/v1/ingest/<channel>` with `ZYNTRA_INGEST_TOKEN`) and `manual` (entered in the console, audited). Rows can be filtered, aggregated and divided:

```yaml
- id: stockout_rate               # share of SKUs with nothing on hand
  unitClass: ratio
  source: {kind: file, file: fixture/stock.csv, field: "#(on_hand=0)", denominator: "#"}
- id: daily_sales
  currency: INR
  calendar: shop-hours            # outside the window the last in-window value is held
  source: {kind: file, file: fixture/pos.csv, field: "*.amount"}   # summed
- id: erp_open_pos
  source: {kind: http, url: "${ZYNTRA_ERP_URL}/po?status=open", headers: {Authorization: "Bearer ${ZYNTRA_ERP_TOKEN}"}, field: "#"}
```

Every source reports `ok`, `stale`, `error` or `fallback`. Only `${ZYNTRA_*}` variables are expanded, and they stay unexpanded in rendered proposals.

**Generic actions.** Besides kubectl and Keep, an action can be a `webhook` (dry-run prints method, URL, headers and body; apply sends it with an `Idempotency-Key` and records the status and response hash), a `file` (written under `ZYNTRA_OUTPUT_DIR`, never overwritten) or `noop` (people do it; the approval is recorded). Actions can also declare:

```yaml
  window: buy-hours                                  # approved runs wait for the window
  approvers: 2                                       # two-person
  preconditions: [{kpi: stockout_rate, worse_than: 0.02}]   # fail closed: shown, scored, not approvable
  invariants: [{kpi: gross_margin, max_worsen: 0.03}]       # a simulated break blocks the action
  compensate: cancel_open_po                         # linked on the proposal as the undo; never auto-run
```

After an apply, the decision record compares predicted and actual per KPI and marks each a hit or a miss. Every audit event for an action also carries `payload_sha256` (what was approved and sent) and, after a webhook, `response_sha256`; both are inside the hash chain, so `GET /api/v1/audit/verify` fails if either is altered.

**Test inbox.** `examples/receiver` (built as `bin/zyntra-receiver`) accepts webhook deliveries, stores one JSON file per delivery, answers a repeated `Idempotency-Key` with `200 {"duplicate":true}` instead of recording it twice, and redacts `Authorization`, `Cookie` and `X-Api-Key`. Point a pack's URLs at it during a pilot (`make run-shop` does) and open `http://127.0.0.1:9099` to see what landed. Shipped packs: [shop](https://github.com/zyvorai/zyvor-zyntra/blob/main/packs/shop), [gpu](https://github.com/zyvorai/zyvor-zyntra/blob/main/packs/gpu) (the lab model), [manufacturing](https://github.com/zyvorai/zyvor-zyntra/blob/main/packs/manufacturing), [logistics](https://github.com/zyvorai/zyvor-zyntra/blob/main/packs/logistics), [payments](https://github.com/zyvorai/zyvor-zyntra/blob/main/packs/payments) and [imaging-ops](https://github.com/zyvorai/zyvor-zyntra/blob/main/packs/imaging-ops) (capacity and flow only: no patient data, no clinical decisions). The newer ones are starting points: their weights are declared, not measured. See [docs/PRODUCT_PLAN.md](https://github.com/zyvorai/zyvor-zyntra/blob/main/docs/PRODUCT_PLAN.md) for the pack catalog and build order.
