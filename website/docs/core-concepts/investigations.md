---
sidebar_position: 5
---

# KPI investigations

The Insights console's **Investigate a KPI** panel detects sustained level
movement and explores associations between hourly changes of permitted KPIs.
Choose a target, window, recent period and maximum candidate lead. Ask also
accepts `scope: "investigation"` with a question naming exactly one metric ID or
full name, for example `investigate p99_inference_latency`.

Ask uses fixed settings: 168 completed UTC hours, 6 recent hours and candidate
leads from 0 through 6 hours. Only the target is extracted from the question;
use the Insights form or structured API to set other parameters. The settings
and exact window are always returned and displayed. No LLM translates, rewrites
or receives an investigation. Operational, document and analytics-query Ask
scopes keep their existing behavior.

## Structured API

`POST /api/v1/analytics/investigate` requires a reader identity and follows pack
selection. It accepts one strict JSON object, limited to 8 KiB:

```json
{
  "metric": "p99_inference_latency",
  "window_hours": 168,
  "recent_hours": 6,
  "max_lag_hours": 6,
  "candidates": ["gravia_pending_jobs", "netra_flow_count"]
}
```

Use IDs from `GET /api/v1/analytics/catalog`; example IDs must exist in your
selected pack. Unknown fields, trailing data, unknown/hidden IDs, duplicate
candidates and a candidate equal to the target fail with HTTP 400. Limits:

| Parameter | Bounds |
| --- | --- |
| `window_hours` | 24–720 completed hours |
| `recent_hours` | 3–24, with at least 12 remaining baseline hours |
| `max_lag_hours` | 0–12 inclusive; 0 tests the same hour only |
| `candidates` | Up to 64 distinct permitted metrics; omitted or empty selects all other permitted metrics, capped at the first 64 sorted IDs |

Tenant identities can select only their own tenant's KPIs, for target and
candidates. Query parameters cannot override identity. Provider sources, owners,
other tenants' names, values and model metadata are excluded. The history copy
is bounded to the authorized target and selected candidates. Catalog freshness
checks read the latest point without copying every retained series.

## Hourly evidence and sustained shifts

The exact window is `[start, end)`, where `end` is the current UTC hour's start
and `start = end - window_hours`. The current partial hour and future samples
are excluded. Samples within each hour receive equal weight; hour means receive
equal weight in the analysis. Missing hours remain missing. Values must be
finite with magnitude at most 1e100; an invalid observation in the selected
window makes that metric unavailable instead of silently repairing it.

All requested recent hours must be observed. The baseline uses the longest
consecutive sequence immediately before the recent period, within the window,
and needs at least 12 hours. A baseline gap can shorten the baseline; a recent
gap produces `warming`. Returned baseline bounds/counts show the actual history
used, which may be shorter than the requested window.

The detector compares recent median with baseline median. Its threshold is:

```
max(3 × 1.4826 × baseline MAD,
    abs(baseline median) × 0.000001,
    0.000000000001)
```

MAD is the median absolute deviation from the baseline median. The two floors
avoid treating insignificant arithmetic noise in a flat series as a shift.
At least `ceil(0.8 × recent_hours)` recent hour means must exceed the threshold
in the same direction, and the median difference must exceed it too. Results
are `shift` (up/down), `stable`, `warming` or `unavailable`. The report includes
medians, delta, threshold, actual/required supporting hours and hourly sample
counts. It does not estimate the precise onset time or a probability of change.

## Lagged change relationships

For each candidate, calculate differences only between consecutive observed
hour means. Pair candidate change at `t - lag` with target change at `t`, by
actual UTC timestamps. A missing hour removes adjacent changes; array positions
are never used to pair observations and gaps are never bridged.

Each eligible lag requires at least 24 aligned pairs and non-negligible variance
in both change vectors. Pearson correlation is calculated after normalizing each
vector independently to prevent arithmetic overflow. Constant deltas—including
perfectly shared linear level trends—cannot establish a change relationship.
The strongest absolute correlation wins; numerical ties within 1e-12 prefer the
smaller lag. An absolute coefficient of at least 0.6 is labelled `associated`;
other eligible results are `weak`. Insufficient pairs or variance are `warming`.
A 24-hour window can support shift detection but cannot yield the 24 change
pairs required for a relationship; larger windows are normally needed.

The report lists every examined candidate, its status, warning, best coefficient,
lead, pair count, eligible-lag count, target change-time bounds and fingerprint.
The UI highlights up to five associations and exposes all diagnostics. Full
aligned changes for the first five scored comparisons are included as `evidence`,
so their correlations can be independently recalculated. The displayed pair
table shows the latest 12 pairs; JSON export retains all included pairs.

## Export and interpretation

Download investigation JSON from Insights or Ask. It contains the exact query,
window, target hourly means, shift calculation, candidate diagnostics, included
pair evidence and SHA-256 fingerprints of the JSON-encoded sorted hourly/pair
records. The UI shows the most recent 24 observed target hours; export includes
all target hours. Fingerprints identify computed evidence, are unsigned, and do
not replace source telemetry or a tamper-resistant archive.

Associations are **exploratory, not causes or statistical significance**. Searching
multiple lags and KPIs increases the chance of a high coefficient. Seasonal
cycles, shared sources, cumulative counters, outliers and operational schedules
can mislead. Shift detection is not seasonality-adjusted; missing or sparse
samples can also bias hour means. A positive lead describes observed timing and
does not prove prediction. Review incident timelines and source lineage before
interpreting a relationship. This feature changes no model edge, proposal,
action, planner score or execution policy. No schema migration or dependency.
