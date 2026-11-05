# Process safety pack

Lagging and leading process-safety indicators for a process plant. It answers: *where are safety barriers overdue, and what would scheduling proof tests, closing change items or a stand-down do to the indicators?*

```bash
./bin/zyntra plan -f packs/process-safety
```

The data in `fixture/` is invented. Every action is advisory (noop). Standing a unit down is an operating decision that belongs to your safety authority; the pack only records a proposal, and it needs three approvers. Use `examples/policy-industrial.yaml` for the approval rules.

The tier 1 and tier 2 split follows API RP 754, but this pack does not classify events (that needs the standard's release and consequence thresholds, which change between editions), and it is not a PSM, OSHA or RP 754 compliance tool. Tier 3 and 4 indicators are company-defined; the ones here are examples.

Status: unvalidated against a real incident or maintenance system.

## Rules

- No tier 1 events.
- No safety-instrumented device past its proof-test date.
- No overdue management-of-change items.
- No more than two demands on safety systems.
