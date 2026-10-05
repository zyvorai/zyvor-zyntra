# Read-only KPI analytics queries

Choose **KPI analytics** in Ask, select a metric from the permitted catalog and
ask, for example, `average alpha_latency over the last 24 hours` or
`daily trend alpha_latency over the past 7 days`. With no gateway configured,
local parsing requires an exact metric ID or full name, one calculation, and a
window expressed as `last N hours` or `past N days`. The default window is 24
hours. Unsupported or ambiguous phrasing returns guidance, without a result.

An optional configured AI gateway translates broader phrasing into a structured
query. Only the caller's minimal metric catalog is sent; history, sources and
provider metadata are not sent. Unknown fields, unauthorized or duplicate metric
IDs, unsupported operations and invalid bounds are rejected. Invalid translation
falls back to the explicit local grammar. Numbers and answer prose are generated
by deterministic code; no query answer is rewritten by the model. A valid model
translation can still misinterpret intent, so review the returned query and window.

## API

Both endpoints require a reader identity and honor the selected pack:

- `GET /api/v1/analytics/catalog`: permitted IDs, names, display units, generic
  current-data warnings, supported operations and limits.
- `POST /api/v1/analytics/query`: execute a validated structured query. No AI
  dependency. Unknown JSON fields, trailing data and bodies above 8 KiB fail.
- `POST /api/v1/ai/ask` with `scope: "analytics"`: translate and execute; the
  answer includes `analytics_query` on success. Guidance responses omit it.
  Document and operational-model Ask retain their existing behavior.

```json
{
  "metrics": ["alpha_latency"],
  "operation": "trend",
  "window_hours": 168,
  "bucket_hours": 24
}
```

A query selects 1–5 distinct authorized metrics and a 1–720 hour window. Results
are independent per metric; there are no mixed-unit sums, rankings or joins.
Tenant identities see only KPIs whose tenant exactly matches theirs. Query
parameters cannot override this scope. The catalog contains no source URLs,
owners, model version, provider KPIs or other tenants' metrics. Machine-only roles
cannot use these reader routes.

| Operation | Calculation |
| --- | --- |
| `latest` | Last observed historical sample in the window |
| `mean` | Arithmetic sample-weighted mean |
| `min` / `max` | Minimum / maximum observed value |
| `change` | Last minus first; at least two samples required |
| `trend` | Sample-weighted means in 1–168 hour buckets anchored at window start |

`bucket_hours` is required only for trend and must not exceed the window; it is
rejected for every other operation. Time is UTC. The window includes both
endpoints; a sample at the query end belongs to the final bucket. Empty buckets
are omitted and no interpolation, extrapolation or missing-as-zero substitution
occurs. Missing history and insufficient change samples return `value: null`.
Trend returns buckets and no single scalar. Non-finite samples are discarded;
numeric overflow returns an error instead of invalid JSON.

## Evidence and limits

The response echoes the validated query and exact window start/end. Each metric
includes its unit, sample count, first/last observations and SHA-256 of the JSON
encoding of its sorted, selected samples. The hash is a fingerprint, not a signed
attestation or a replacement for retained raw telemetry. Ask displays the method,
sample bounds, fingerprint and bucket means directly from the result.

Current stale, held, unavailable or old-history states produce a generic warning;
valid historical samples can still answer historical questions. `latest` does not
promise a live reading. Retention and missed-refresh gaps can leave only part of
the requested window observed. Means weight samples equally, not elapsed time;
counts represent observations, not business transactions. These queries do not
infer causality, predict values, execute SQL, query arbitrary databases, or run
approved actions. No new persistence format or dependency is introduced.
