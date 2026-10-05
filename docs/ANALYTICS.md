# Analytics foundation

Zyntra now keeps longer KPI history and exposes deterministic seasonal analytics
through the existing authenticated `GET /api/v1/ai/insights` endpoint and the
Insights console. Ask accepts questions containing “seasonal”, “backtest”,
“forecast accuracy” or “robust anomalies” and explains the computed projections
with `analytics:<kpi>` grounding references. This does not change plan scores,
simulator weights, approval requirements or execution.

## Durable history

`zyntra serve` opens `analytics.sqlite` in each pack's existing state directory.
SQLite WAL transactions with synchronous FULL commit each recorded refresh before
updating the in-memory history. There is one database per pack, retaining the
latest **43,200 observations per KPI**, with at most one observation per minute.
At minute sampling this covers about 30 days; a slower refresh covers longer.
The history remains bounded in memory as well as on disk. There is no new external
service or Go dependency; this reuses the repository's pure-Go SQLite driver.

On the first startup with an empty database, the old `history.json` is imported
transactionally and kept as a backup. Subsequent startups read SQLite and do not
reimport the JSON. A malformed legacy file or an unavailable database fails
startup visibly. A later failed commit leaves the in-memory history unchanged;
`analytics.persistence_error` appears in the API and console. Restore database
backups using SQLite's backup API or checkpoint and stop the process first; do
not copy only the main database while WAL writes are active.

Failed, warming, held, or unreported live KPI refreshes do not enter history.
Non-finite values and non-increasing timestamps are discarded. Analytics refuses
input magnitudes above 1e100 to keep its arithmetic and JSON responses finite. Historical JSON
has no source-quality metadata; imported values cannot be retroactively verified.
Minute sampling can miss events between samples. The retention is a sample cap,
not a guarantee of thirty days or a substitute for an external telemetry archive.

## Forecast models and evaluation

The analytics report aggregates observations into UTC hourly means and excludes
the newest incomplete hour. It uses only the latest consecutive sequence of
hourly buckets. Missing hours are not filled or extrapolated across. Daily
seasonality therefore means a UTC day, not a pack's local business calendar;
calendar-aware seasonality is future work.

At least **72 completed consecutive hourly buckets** are required. Four model
candidates compete where sufficient training history exists:

- `last-value`: repeat the last observed hourly mean; also the baseline.
- `linear`: fit least-squares trend to the most recent 48 training hours.
- `daily-seasonal`: repeat the observed value from the corresponding hour in
  the last day.
- `weekly-seasonal`: repeat the corresponding hour in the last week; enabled
  with 360 hourly buckets, so the earliest evaluated origin has two weeks.

Each candidate predicts the last 24 hours using 24 rolling origins, one hour
at a time. Every prediction uses only observations before its origin. The lowest
mean absolute error (MAE) wins; ties prefer the simpler model. The API also returns
RMSE, last-value baseline MAE and sample count. The selected model is refitted
using all available completed hours and projects 1, 6, 24 and 168 hours from the
last completed bucket. `at` provides the absolute UTC time for each projection.

The low/high band is the selected model's 90th-percentile **absolute one-hour
backtest error** around the projected value. It is an empirical diagnostic band,
not a calibrated multi-horizon prediction interval or a probability of breach.
Long horizons are especially uncertain. Selection and error estimation use the
same evaluation window; this is not independent test-set performance. Point
projections are marked against the declared target; the model does not learn
physical bounds or causal effects. Daily/weekly candidates repeat recent patterns
and do not combine seasonality with trend.

Inputs currently missing, stale, held or carrying a refresh error are unavailable.
History older than the larger of two minutes or three refresh intervals is also
unavailable. Insufficient or discontinuous history returns `warming` with a
reason and empty projections, rather than invented forecasts.

## Robust anomalies

A completed-hour value is compared with the median of up to 48 preceding hours.
Once eight same-hour daily observations exist, those observations become the
baseline, reducing false positives from normal daily cycles. The score is
`(value - median) / (1.4826 * MAD)`; absolute scores above 3.5 warn and above 7
are critical. A meaningful deviation from a flat baseline gets a finite capped
score. These are statistical deviations, not causal explanations or proof of
business harm. Existing z-score anomalies and linear target forecasts remain
available in the original response fields.

## API compatibility and access

```text
GET /api/v1/ai/insights
  anomalies: existing z-score results
  forecasts: existing linear target forecasts
  analytics:
    persistent, persistence_error?, interval_note
    anomalies: robust anomaly results
    kpis: status, reason?, observations, hourly_samples, last_sample?,
          selected?, backtests, projections
```

The route retains reader authorization and the existing deny-by-default tenant
gate: provider-wide analytics cannot be read by tenant-bound identities. Pack
selection uses `X-Zyntra-Pack` or `?pack=` as before. Tenant Ask requests continue
receiving an object-only snapshot without analytics. AI can explain this report;
it cannot adjust the report's numbers or approve an action.

## Validation

Tests cover daily and weekly selection, a linear trend, flat baselines, a
contaminated anomaly baseline, chronological backtesting, missing-hour gaps,
stale inputs, durable restart, legacy migration, corrupt imports, failed commits,
retention, non-finite values, authorization, Ask grounding and console rendering.
Run `go test -race ./...`, `go vet ./...`, `make build`, `npm test --prefix web`,
`make eval` and `make test-e2e`.

This change delivers the analytics foundation. Document retrieval, natural-language
SQL, process mining, financial exposure and constraint optimization remain
separate extensions; they are not implemented by this PR.
