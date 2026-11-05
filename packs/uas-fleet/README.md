# Drone and eVTOL fleet pack

Readiness, battery health, firmware currency and landing outcomes for a drone or eVTOL operator. It answers: *which aircraft and batteries are limiting the fleet, and what would retiring batteries, inspections or a firmware update do to readiness and landing safety?*

```bash
./bin/zyntra plan -f packs/uas-fleet
```

The data in `fixture/` is invented. Every action is advisory (noop). A firmware update can change flight behaviour, so it needs two approvers. Use `examples/policy-aviation.yaml` for the approval rules.

Battery limits here are examples; use the manufacturer's. Nothing in the pack is a Part 107, SORA or type-certification requirement, and it does not release an aircraft to fly.

Status: unvalidated against a real fleet-management system.

## Rules

- Aircraft ready to fly must stay at or above 0.8.
- No battery over 300 cycles.
- Failed-landing rate must stay at or below 0.02.
- No aircraft on old firmware.
