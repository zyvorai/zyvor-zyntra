# Industrial, drone and launch-range packs

Five packs cover plant operations, process safety, control-network security,
drone and eVTOL fleets, and launch ranges. They are advisory: every action is
`noop`, so approving records the decision and people make the change. Nothing
here operates a plant, commands a vehicle, makes a go/no-go call, or stands in
for a safety authority.

| Pack | Answers | Based on |
|---|---|---|
| `industrial-plant` | Which machines and skipped maintenance are costing run time, and what is OEE? | ISO 22400-style availability, performance, yield, MTBF, MTTR and maintenance-compliance measures; OEE is a [derived KPI](REFERENCE.md#derived-kpis) |
| `process-safety` | Where are safety barriers overdue? | The tier 1 to 4 idea in API RP 754; tiers 3 and 4 are company-defined and the ones here are examples |
| `industrial-ot-security` | Which control assets are exposed, unmonitored or unpatched? | The zones-and-conduits idea behind IEC 62443; no security levels are assigned |
| `uas-fleet` | Which aircraft and batteries limit a drone or eVTOL fleet? | Readiness, battery-cycle, firmware and landing-outcome tracking common to fleet tools |
| `launch-range` | What is holding up the range? | Slot use, pad turnaround, ground-equipment readiness and holds |

All fixture data is invented. Targets and weights are starting points, not a
standard. The research behind these packs was mostly vendor material and
secondary summaries, so check any figure you adopt against the standard itself
and your regulator. In particular, none of this is an ISO 22400, API RP 754, OSHA
PSM, IEC 62443, Part 107, SORA or range-safety requirement, assessment or
certification, and the process-safety pack does not classify events against
RP 754's thresholds.

## Run them

```bash
make run-industrial     # all five packs with examples/policy-industrial.yaml
./bin/zyntra plan -f packs/process-safety
```

## What the packs use

They read CSV files with the source features from the aviation work (see
"Source fields" in `REFERENCE.md`): `agg: p90` for percentiles and `where`
comparisons such as `due_in_days: "<0"`, `cycles: ">300"` and `state: "!=ready"`.
`timeField` and `within` are available for "in the last 24 hours" counts when
your export carries timestamps; the fixtures do not use them, because fixed
timestamps in a sample file would age out.

## Policy

`examples/policy-industrial.yaml` requires two approvers on every proposal and
three on high-risk ones (load shifts, patching, firewall changes, stand-downs),
never lets the proposer approve their own, blocks approval on stale inputs, and
defines an example plant-stop window. Combine it with the `markings:` and
`egress:` blocks in `DEFENCE.md` when asset or incident data is sensitive.

## What it does not do yet

Event classification against RP 754 thresholds, security-level assessment,
regulatory reporting, and connectors for named historians, CMMS or fleet
products. Certification of any kind is out of scope.
