# Decision planning workbench

Open **Decide → Decision planning** (`#/planning`). The workbench extends the
existing simulator; it never creates a proposal, approves a change, installs a
model or executes an action. Existing approval and execution revalidation remain
mandatory. Portfolio budgets and capacity are advisory planning constraints, not
new execution policy. Re-propose chosen actions through the existing approval lane.

## Implemented

- Bounded portfolios of one to six actions, including actions affecting the same
  KPI; budget, resource capacity, prerequisite membership and mutual exclusions.
- Financial assumptions: upfront cost, monthly running cost, deduplicated monthly
  benefits, monthly net, 12-month net, simple payback, and 30-day delay estimate.
- Baseline snapshots with a SHA-256 checksum and offline replay without source reads.
- Edge-weight change previews comparing rankings and optional actual-outcome error.
- Time-grid simulation with one-shot persistent effects and delayed propagation.
- Local-date holidays and timestamp-based freezes in existing calendar windows.
- Event-log process analysis and descriptive before/after/control comparisons.
- Batch data-quality previews and configured webhook quarantine gates.
- Horizon-specific forecast validation and empirical coverage.
- Persisted webhook events with idempotency, late-event retention and cursor export.

## Planning configuration

Add `planning.json` beside the pack's `kpis.yaml`. A missing file is allowed;
financial estimates are then marked incomplete, and a budget blocks candidates
with missing cost data. The leadership pack includes explicitly synthetic demo
assumptions; replace them before using estimates to support a real decision.

```json
{
  "currency": "INR",
  "actions": {
    "scale": {
      "resources": {"engineers": 2},
      "requires": ["prepare"],
      "excludes": ["defer"],
      "economics": {
        "cost": 100000,
        "monthly_cost": 20000,
        "benefits": [{"id": "avoided-outsourcing", "monthly": 50000, "evidence": "Supplier quote 2026-11-10"}],
        "assumptions": ["Constant monthly demand for twelve months"]
      }
    }
  }
}
```

All named actions must exist in the pack. Prerequisites must be acyclic. They
mean **included in the portfolio**, not an executable sequence. Resources are
additive declared requirements; omitted capacity keys are unbounded. Cost limits
apply to implementation costs, not lifetime spend. One currency applies to the
whole configuration; there is no exchange-rate conversion. Shared benefit IDs
count once, using the largest estimate, so alternative actions cannot claim the
same saving twice. Distinct IDs must represent distinct benefits.

The search enumerates singles, then pairs, then larger sets in action-ID order.
It returns all evaluated candidates, including blocked/non-improving candidates,
ranked with unblocked candidates first and then by pessimistic improvement minus
existing risk and stale-input penalties. It does not claim a positive-score
candidate exists. Financial benefit does not change the existing KPI scorer.
Default: 3 actions and 2,000 candidates; maximum: 6 actions, 10,000 candidates,
200 model actions, and 250,000 retained KPI simulation cells. `truncated=true` means the search is incomplete, not optimal.
HTTP computation has a 10-second deadline. Stale touched inputs, failed
preconditions, violated invariants and closed windows block a portfolio.

## HTTP and CLI

All routes are under `/api/v1/planning/`, require an existing reader role, and
are denied to tenant-bound accounts. Model snapshots honor data markings.
Economic configuration is omitted when a caller cannot see the full model,
because free-text assumptions do not yet have property-level markings.

| Method | Route | Input |
|---|---|---|
| GET | `snapshot` | Current pack, same pack selector as other API routes |
| POST | `portfolios` | `{"max_actions":3,"max_candidates":2000,"budget":1000000,"capacity":{"review_hours":8}}` |
| POST | `replay` | `{"snapshot":{...},"options":{"max_actions":3}}` |
| POST | `review` | `{"snapshot":{...},"changes":[{"from":"queue","to":"latency","weight":0.4}],"options":{}}` |
| POST | `timeline` | `{"actions":["scale"],"horizon":"24h","step":"1h"}` |
| POST | `process` | `{"events":[{"id":"1","case":"order-1","activity":"received","at":"2026-11-10T08:00:00Z"},...]}` |
| POST | `benefits` | Experiment object below |
| POST | `quality` | `{"contract":"orders","rows":[...],"observed_field":"updated_at"}` |

```sh
zyntra planning plan -f packs/leadership -request options.json
zyntra planning capture -f packs/leadership > snapshot.json
zyntra planning replay -request replay-request.json
zyntra planning review -request review-request.json
zyntra planning process -request events.json
zyntra planning benefits -request experiment.json
```

CLI output is JSON. CLI capture currently conservatively treats live-source
freshness as unverified; use the running server's snapshot for tracked refresh
status. Replay and review do not load the current pack or contact sources.

Snapshots contain the baseline model, planning configuration and freshness flags.
Protect exported files as operational data. Their checksum detects accidental
modification; it is **not a signature or proof of origin**. The snapshot's capture
time determines replay calendar checks. Historical snapshots must be captured
beforehand; existing KPI history alone cannot reconstruct old models or assumptions.

Review accepts optional `actions` and `actual` KPI values for absolute-error
comparison. Download the candidate model and diff as a review artifact, apply
through your existing reviewed configuration deployment, and keep the prior
version for rollback. This is a review preview, not a durable model-approval
workflow. Actual outcomes may include confounding changes.

Timeline points are computed relative to the fixed baseline, never by repeatedly
compounding an action. A direct effect appears once its delay elapses; edges read
the upstream result at their delayed sample. Delays round up to the grid. Limits:
240 intervals, horizon at most 365 days, positive step dividing the horizon, up to
six actions. Every sample includes uncertainty and constraint violations. This
models declared persistent step effects, not demand evolution, recovery curves,
continuous physics, phased execution or scheduling.

## Calendar exceptions

```yaml
calendars:
  change-window:
    days: [mon, tue, wed, thu, fri]
    start: '09:00'
    end: '18:00'
    timezone: Asia/Kolkata
    holidays: ['2026-11-10']
    freezes:
      - start: '2026-12-31T00:00:00+05:30'
        end: '2027-01-02T09:00:00+05:30'
        reason: Financial close
```

A holiday closes the entire local date, including the carry-over portion of an
overnight window. A freeze includes its start and excludes its end. Calendar
exceptions are used by the existing sampling, action and policy window checks.

## Evidence utilities

Process analysis accepts up to 10,000 unique event IDs, orders each case by event
time, reports repeated-activity cases, observed case spans, and mean/P95 time
between activities. Simultaneous events within a case are rejected as ambiguous.
Observed intervals include waiting and processing and are not activity duration.
Incomplete cases are not inferred to be complete. No process-conformance engine
or business-object linkage is claimed.

Benefit measurement request:

```json
{
  "metric":"queue_wait", "unit":"minutes", "higher_is_better":false,
  "treatment_before":[10,12], "treatment_after":[6,8],
  "control_before":[10,12], "control_after":[8,10],
  "confounders":["Demand mix changed"]
}
```

Each period requires 2..10,000 finite values. The report separates observed change
(-4 here) from change after subtracting the control's change (-2 here). Positive
`improvement` respects metric direction. This does not establish causality or
statistical significance; comparison groups and periods must be selected responsibly.

Quality checks use the named pack contract in `data-contracts.json`. They report
required-field completeness, type mismatches, undeclared fields, duplicate
composite keys, stale observation times and future timestamps (>1 minute skew).
Diagnostics name row indexes and fields, not cell values. They identify affected
KPIs. The standalone `quality` route is a preview. Bind a webhook channel to its contract through `ingest-contracts.json` to enforce quarantine during ingestion; see [Input events](INPUT_EVENTS.md).
Enum membership, units and cross-source reconciliation remain future extensions.
Full model clearance is required for contract previews.

## Remaining roadmap work

This change does not implement distributed execution/HA, PostgreSQL state,
independent tenant simulators, automated dead-letter redelivery,
a durable model approval service, or validated
live product integrations. Those require changes to persistence, authorization,
connector contracts and execution fencing; they are not represented as completed
by the workbench. Existing single-writer deployment limits remain in force.

## Validation

Run `go test -race ./...`, `go vet ./...`, `make build`, `npm test --prefix web`,
`make eval`, and `make test-e2e`. New tests cover search bounds, resource/budget
rejections, economic deduplication, cancellation, snapshots, model review,
calendar boundaries, delay propagation, process evidence, quality previews,
authentication, tenant denial and marked-data visibility.
