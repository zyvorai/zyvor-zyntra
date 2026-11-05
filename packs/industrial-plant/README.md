# Plant operations pack

Availability, yield, reliability and maintenance discipline for a plant. It answers: *which machines and which skipped maintenance are costing run time, and what would a repair, a catch-up or a load shift do to availability?*

```bash
./bin/zyntra plan -f packs/industrial-plant
```

The data in `fixture/` is invented. Every action is advisory (noop). Shifting load changes the run plan, so it needs two approvers and is blocked if yield would fall by more than three points. Use `examples/policy-industrial.yaml` for the two-person rules.

The measures follow the kinds of indicators ISO 22400 defines (availability, quality, MTBF, MTTR, maintenance compliance), but targets are starting points, not the standard, and the pack does not implement it. OEE needs availability, performance and quality multiplied together; this pack reports availability and yield only.

Status: unvalidated against a real MES or CMMS.

## Rules

- Machine availability must stay at or above 0.85.
- First-pass yield must stay at or above 0.95.
- No machine may be down.
- Preventive maintenance completed must stay at or above 0.9.
