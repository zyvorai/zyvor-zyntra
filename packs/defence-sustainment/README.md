# Defence sustainment pack

Fleet readiness, parts supply and depot maintenance load. It answers: *which
grounded assets are waiting on which parts, and what would expediting
requisitions or reprioritising the depot queue do to the mission-capable rate?*

```bash
./bin/zyntra plan -f packs/defence-sustainment
./bin/zyntra scenario compare -f packs/defence-sustainment parts=expedite_critical_parts depot=reprioritise_depot_queue
```

The data in `fixture/` is invented. Only ids, statuses and counts are read; keep
CUI and precise locations out of the exports. Every action is advisory
(noop): approving records the decision and people make the change.

Status: unvalidated against a real maintenance system.

## Rules

- Mission-capable rate must stay at or above 0.8.
- Not mission-capable, supply must stay at or below 0.05.
- Parts fill rate must stay at or above 0.9.
