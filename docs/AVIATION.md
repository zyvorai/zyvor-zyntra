# Aviation packs

Three packs model the operations-control, maintenance and airport work of an
airline, a maintenance organisation and an airport operator. They are advisory: every action is `noop`, so
approving records the decision and people make the change. Nothing here
dispatches or releases an aircraft, controls air traffic, sets crew duty limits, or approves
airworthiness.

| Pack | Answers | Based on |
|---|---|---|
| `aviation-ops` | Where is the day slipping, and what would a standby crew, an extra ground team or a tail swap do to on-time performance? | Departure punctuality, turnaround and crew-duty measures common to airline operations control |
| `aviation-mro` | Which aircraft are down behind a part or a queue, and what would an expedite, an extra shift or a deferral do to availability? | Fleet availability, work-order backlog, deferred-defect and parts-supply measures common to maintenance organisations |
| `airport-ops` | Where is the airside or terminal flow backing up, and what would a stand reassignment, an extra baggage team or another security lane do to it? | Turnaround, additional taxi-out time and slot-adherence measures of the kind used in airport collaborative decision making (A-CDM) |

All fixture data is invented. Targets and weights are starting points, not a
regulatory or industry standard. This work was researched mostly from vendor
material, so check any figure you adopt against your regulator and your own
approved procedures. In particular, nothing in these packs is a Part 145, CAMO,
EASA or FAA requirement, and none of it is an airworthiness approval.

## Run them

```bash
make run-aviation     # all three packs with examples/policy-aviation.yaml
./bin/zyntra plan -f packs/aviation-ops
```

## What the packs use

They read CSV files with the source features added for this work (see "Source
fields" in `REFERENCE.md`):

- `agg: p90` for the 90th-percentile turnaround and work-order age (nearest
  rank).
- `where` comparisons such as `dep_delay_min: ">60"`, `headroom_h: "<4"` and
  `due_in_days: "<0"`, to count long delays, crews near a limit and overdue
  deferred defects.
- `timeField` and `within` are available for "in the last 24 hours" counts when
  your export carries timestamps. The fixtures do not use them, because fixed
  timestamps in a sample file would age out.

To point a pack at your own systems, keep the column names listed in each
`sources.example.yaml` and replace the files in `fixture/` with exports, or
change the `file` sources to `http` ones (paged and retrying; see
`examples/defence/` for the shapes). Nothing has been run against a real
operations-control, crew-management or maintenance system.

## Policy

`examples/policy-aviation.yaml` requires two approvers on every proposal and
three on high-risk ones (a tail swap, a defect deferral), never lets the
proposer approve their own, blocks approval on stale inputs, and expires
approvals after one to two hours. Add local users or an OIDC role map for your
roles. To keep sensitive KPIs from lower clearances, combine it with the
`markings:` block described in `DEFENCE.md`.

## What it does not do yet

A-CDM milestone messaging and runway-capacity or ATC-delay measures (the
airport pack covers turnaround, stands, slot adherence, baggage and security
queues only), drone and eVTOL fleets and launch ranges are in
[INDUSTRIAL.md](INDUSTRIAL.md). Real connectors for named operations or
maintenance systems are not included. Certification of any kind is out of scope.
