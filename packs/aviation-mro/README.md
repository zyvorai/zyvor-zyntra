# Aviation maintenance pack

Fleet availability, work-order backlog, deferred defects and parts supply for a
maintenance organisation. It answers: *which aircraft are down because of a
part or a queue, and what would an expedite, an extra shift or a deferral do to
availability?*

```bash
./bin/zyntra plan -f packs/aviation-mro
```

The data in `fixture/` is invented. Every action is advisory (noop). Deferring
a defect is an airworthiness decision that belongs to your approved
procedures and certifying staff; the pack only records a proposal, needs two
approvers, and is blocked if it would add to the defects already past due. Use
`examples/policy-aviation.yaml` for the two-person rules.

Targets are starting points, not a regulatory or industry standard. Nothing
here is an airworthiness, Part 145 or CAMO approval, and the pack does not
release an aircraft to service.

Status: unvalidated against a real maintenance information system.

## Rules

- Aircraft availability must stay at or above 0.9.
- No deferred defect may be past its due date.
- Parts fill rate must stay at or above 0.9.
- No AOG-critical part may have none on hand.
