# Aviation operations pack

Punctuality, cancellations, turnaround and crew duty headroom for an airline
operations control centre. It answers: *where is the day slipping, and what
would a standby crew, an extra ground team or a tail swap do to on-time
performance?*

```bash
./bin/zyntra plan -f packs/aviation-ops
```

The data in `fixture/` is invented. Every action is advisory (noop): it asks
people to roster, staff or swap, and changes nothing in an operations system.
Swapping a tail needs two approvers and is blocked if it would put more crews
near their duty limit. Use `examples/policy-aviation.yaml` for the two-person
rules.

Targets are starting points, not a regulatory or industry standard. The pack
does not decide crew duty limits or dispatch an aircraft; legal limits and
release decisions stay with your operations and airworthiness procedures.

Status: unvalidated against a real operations control system.

## Rules

- On-time rate must stay at or above 0.85.
- Cancellation rate must stay at or below 0.02.
- Turnaround, 90th percentile, must stay at or below 60 minutes.
- No more than one crew within four hours of its duty limit.
