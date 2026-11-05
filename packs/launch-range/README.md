# Launch range pack

Range-slot use, pad turnaround, ground-equipment readiness and holds for a launch range. It answers: *what is holding up the range, and what would a repair, rebooking or an extra window do?*

```bash
./bin/zyntra plan -f packs/launch-range
```

The data in `fixture/` is invented. Every action is advisory (noop). The pack does not make go/no-go decisions, command any vehicle or ground system, or replace range safety; those stay with the launch director and the range authority. Requesting an extra window affects other range users, so it needs two approvers. Use `examples/policy-aviation.yaml` for the approval rules.

Status: unvalidated against a real range-scheduling or ground-systems system.

## Rules

- Critical ground equipment must be ready.
- Equipment ready must stay at or above 0.9.
- Pad turnaround, 90th percentile, must stay at or below 120 hours.
- No technical holds.
