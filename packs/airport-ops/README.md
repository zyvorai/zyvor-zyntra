# Airport operations pack

Turnaround, stands, flow-management slot adherence, baggage delivery and
security queues for an airport operator. It answers: *where is the airside or
terminal flow backing up, and what would a stand reassignment, an extra
baggage team or another security lane do to it?*

```bash
./bin/zyntra plan -f packs/airport-ops
```

The data in `fixture/` is invented. Every action is advisory (noop): it asks
people to reassign, staff or open, and sends nothing to an airport system.
The measures follow the kinds of indicators used in airport collaborative
decision making (turnaround, additional taxi-out time, slot adherence), but
the targets are starting points, not a standard, and the pack does not
implement A-CDM or any milestone messaging. It does not control air traffic or
issue clearances; sequencing and slots stay with the tower and the network
manager. Holding pushbacks needs two approvers and is blocked if it would cost
other flights their slots. Use `examples/policy-aviation.yaml` for the
two-person rules.

Status: unvalidated against a real airport operational database.

## Rules

- Slot adherence must stay at or above 0.9.
- Turnaround, 90th percentile, must stay at or below 60 minutes.
- No stand may be blocked.
- Security queue at open lanes, 90th percentile, must stay at or below 10 minutes.
