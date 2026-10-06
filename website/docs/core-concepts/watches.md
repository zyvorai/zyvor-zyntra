---
sidebar_position: 4
---

# KPI watch inbox

Insights and Ask include a **KPI watch inbox**. Administrators create enabled
above/below threshold rules; proposer, approver and admin identities can
acknowledge open incidents with a note. Viewers and executors can read. Tenant
identities see only rules, incidents, runtime and events belonging to their own
tenant; identity scope cannot be overridden with query parameters.

Watches evaluate during the existing server refresh loop. They create in-app
incidents only: no messages, webhooks, action execution, planner adjustments or
LLM requests. Existing AI endpoints remain read-only. A rule is separate from a
KPI target and does not change the model.

## Rule configuration and API

`GET /api/v1/watches` returns permitted `rules`, `runtime`, newest-first
`incidents`, newest-first `events` and any persistence error. Configuration routes
are admin-only:

```http
PUT /api/v1/watches/queue
Content-Type: application/json
```

```json
{
  "name": "Queue watch",
  "metric": "gravia_pending_jobs",
  "operator": "above",
  "threshold": 10,
  "for_seconds": 300,
  "clear_seconds": 60,
  "max_gap_seconds": 120,
  "enabled": true,
  "expected_version": 0
}
```

The metric must exist in the selected pack. Tenant, metric name and display unit
are taken from the model, never the request. Rule IDs are 1–80 ASCII letters,
digits, dots, underscores or hyphens, starting with a letter/digit. Name is 1–120
bytes; threshold must be finite. Breach/recovery durations are 0–86400 seconds;
maximum observation gap is 1–3600 seconds. Equality is healthy for both operators.
Zero duration transitions on the first valid sample; positive durations require
new, qualifying observations spanning that duration.

Use `expected_version: 0` for creation and the returned version for edits.
Stale versions return HTTP 409. Editing or disabling a rule closes its current
incident with reason `rule changed`, retains the original rule snapshot in that
incident and resets timers. Metric tenant scope cannot change; create a distinct
rule for another tenant. Delete using `DELETE /api/v1/watches/{id}` with
`{"expected_version": 1}`; the current incident closes as `rule deleted`, and
retained history remains visible. Recreating a deleted ID uses version zero and
starts a new lifecycle; existing incident IDs remain distinct.

Acknowledge with `POST /api/v1/watch-incidents/{id}/acknowledge` and
`{"expected_version": 1, "note": "Investigating queue"}`. The note is 1–500 bytes
and the actor comes from authenticated identity. Acknowledgement increments the
incident lifecycle version and records the actor/time/note. It does not resolve
an incident. Repeated, resolved or outdated acknowledgements return 409; hidden
or missing incidents return 404. Incidents resolve only on measured recovery or
explicit rule edits/deletion. Rule mutations and acknowledgements require one
strict JSON object, reject unknown fields/trailing data, and are limited to 8 KiB.

## Observation and lifecycle semantics

Only successfully refreshed live KPIs count. Static model defaults, missing,
warming, held, stale or failed refreshes cannot establish breach or recovery.
The model may retain an old value for display; watches do not treat it as a new
valid observation. A static KPI rule therefore remains unavailable until the
metric is backed by a successfully refreshed source. Repeated/out-of-order or
future observation timestamps never advance timers.

| Runtime status | Meaning |
| --- | --- |
| `disabled` | Rule is disabled |
| `waiting` | No active incident; threshold is healthy or no fresh sample yet |
| `pending` | Breach observed; waiting for sustained duration |
| `open` | Incident active; threshold still breached |
| `recovering` | Healthy observations are spanning recovery duration |
| `unavailable` | No usable observation; active incident, if any, stays open |

Missing/invalid observations or a gap longer than the configured maximum reset
pending breach and recovery timers. Choose a maximum gap at least as large as
normal refresh spacing. Acknowledged incidents stay acknowledged while recovery
is pending. Each recovery/re-breach cycle creates a distinct incident. Runtime
reports the last valid observation/time; incident evidence freezes on resolution
and includes the most recent valid recovery sample. Versions guard configuration
and incident lifecycle changes, not every observation-value update.

This is polling-based evidence, not proof that no transient event occurred
between samples. It does not infer causality or provide external alert delivery.
The UI polls the inbox every 15 seconds. Full model/source changes can make a
rule unavailable; recorded tenant scope stays frozen, preventing a moved metric
from supplying another tenant's incident evidence.

## Durability, retention and restart

Each pack stores `watches.sqlite` in its own state directory. Pure Go SQLite is
already a dependency. One bounded state snapshot is atomically committed with
WAL and `synchronous=FULL`; memory is updated only after a successful commit. A database revision guard
prevents a stale second process from overwriting newer state. Use one running
instance per pack state directory; a revision conflict requires restarting the
stale instance against current durable state.
The database is private (0600), with directories at 0750. In-memory stores are
available for tests; an absent API store returns 503 rather than claiming alerts
are persisted.

Retain at most 100 rules, 500 incidents and 2000 transition/configuration events.
Active incidents are never evicted; oldest resolved records are removed first.
Events are a retained operational timeline, not the signed decision audit chain.
Deleted rules retain their incident snapshots/events within these bounds.

Restart preserves rule configuration and open/acknowledged/resolved incidents.
It resets pending/recovery continuity and observation timestamps: unknown downtime
never counts as sustained evidence. Persistence failures expose an inbox error,
retain the last durable state, and reset duration continuity on the next
successful evaluation. Corrupt JSON or an unsupported persisted schema fails
startup visibly. Back up with SQLite's backup API or stop/checkpoint before
copying; copying only the main file while WAL writes are active can lose data.


## Investigate an incident

Use **Investigate watch-N** in the inbox, including for resolved incidents or
incidents whose rule was deleted. `GET /api/v1/watch-incidents/{id}/investigation`
is available to authenticated readers; another tenant's ID returns 404.
The server derives the target and timing from the retained incident, rather than
accepting a client-supplied metric, tenant or time. Reports use the 168 completed
UTC hours before opening, a 6-hour recent period, and candidate leads from 0–6
hours. The partial opening hour and all later observations are excluded. Even
provider readers compare only metrics belonging to the incident's tenant; the
existing 64-candidate and five full-pair-evidence bounds apply.

The response contains `incident`, `generated_at`, `context`, and `report`.
The JSON download includes all four, with hourly and aligned-pair fingerprints.
Incident lifecycle fields reflect the request-time snapshot. Rule settings and
opening time come from the retained incident snapshot. Calculations use currently
retained history and current metric labels, so this is reconstructed evidence,
not a persisted opening-time report. Retention expiry can remove samples or the
incident itself; missing hours remain missing and may yield a warming result.
A short threshold breach need not produce a sustained hourly shift. Associations
remain exploratory and do not establish root cause. If the target is removed or
its current tenant or display unit differs from the incident snapshot, return 409
rather than reinterpret it. No acknowledgement or lifecycle state is changed.
