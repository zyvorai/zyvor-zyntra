# SRE pack

Availability, error budget, latency, incident flow, on-call load and
post-incident follow-up for a platform team. It answers: *which services are
burning their error budget, which incidents still have unfinished follow-ups,
and what would freezing deploys or chasing overdue items do?*

```bash
./bin/zyntra plan -f packs/sre
./bin/zyntra scenario compare -f packs/sre chase=chase_overdue_followups freeze=freeze_risky_deploys
./bin/zyntra ontology impact -f packs/sre Incident:inc:inc-103
```

Follow-up tracking is modelled as `FollowUp` objects linked to the `Incident`
they came from, with `open_followups`, `overdue_followups` and
`postmortems_pending` KPIs. Zyntra does not own the tracker: the export
carries an `overdue` column and a person closes the items there.

Only ids, severities, timings and owners are read; keep customer data and
incident narratives out of the exports. Follow-up owners are marked
`sensitive`. Every action is advisory (noop): approving records the decision
and a person makes the change.
