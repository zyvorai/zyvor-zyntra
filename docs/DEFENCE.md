# Defence packs

Three packs model the sustainment, compliance and equipment-health work that
defence organisations and their contractors run. They are advisory: every
action is `noop`, so approving records the decision and people make the change.
Nothing here classifies, targets or engages anything.

| Pack | Answers | Based on |
|---|---|---|
| `defence-sustainment` | Which grounded assets wait on which parts, and what would expediting or reprioritising do to the mission-capable rate? | Predictive maintenance and supply-risk work such as DLA's forecasting and Army fleet-maintenance AI |
| `defence-compliance` | Which controls are held up by open plan items or stale evidence? | NIST SP 800-171 Rev. 2 as used by CMMC Level 2 |
| `defence-sensors` | Which sites are thin because a sensor or link is down or out of spares? | Sensor and link health in fused-sensor networks |

All fixture data is invented. Check the control list and the Rev. 2 / Rev. 3
transition against the current CMMC rule and NIST publications before relying
on `defence-compliance`.

## Run them

```bash
make run-defence     # all three packs with examples/policy-defence.yaml
./bin/zyntra plan -f packs/defence-sustainment
```

## Policy

`examples/policy-defence.yaml` requires two approvers on every proposal, three
on high-risk ones, never lets the proposer approve their own, and blocks
approval on stale inputs. Add local users or an OIDC role map for the roles you
need.

## Data markings and clearance

A `markings:` block in the policy file turns on ordered data markings:

```yaml
markings:
  levels: [unclassified, cui, cui-specified]   # least restricted first
users:
  - {name: ana, passwordHash: "...", roles: [viewer], clearance: cui}
```

- Mark a KPI or action in `kpis.yaml` (`marking: cui`) or an ontology type or
  property in `ontology.yaml`. Unmarked means the first level.
- An account's `clearance` is the highest level it may see. With OIDC, set
  `ZYNTRA_OIDC_CLEARANCE_MAP="group=level,group2=level"`; a user in several
  mapped groups gets the highest. Everyone else gets the first level.
- Clearance, not role, decides: an administrator without the clearance does not
  see marked data. The break-glass access key and service tokens carry no
  clearance, so they see only unmarked data.
- Below its clearance, a caller does not see marked KPIs at all: not in the
  graph, gaps, plans, forecasts, briefs, watches, the source list or the AI
  snapshot. Edges, constraints and any action that depends on a hidden KPI go
  with it, so a plan can never name or price something the caller cannot see.
  Marked ontology types and properties are filtered on every read.
- A proposal is hidden from a caller who cannot see all of its actions: it is
  missing from lists, decisions, audit and exports and answers 404 by id, and
  it cannot be approved by that caller.
- Markings are validated at start: an unknown level, a marking with no
  `markings:` block, or a user clearance outside the levels stops Zyntra.
- Session cookies now carry the clearance and are signed over it. Existing
  sessions end once after an upgrade and users sign in again.

Limits: this separates what people can see inside one Zyntra instance. It is
not a multi-level-secure system and makes no accreditation claim. Marked data
is still stored together, and a marked item's existence may be inferable from
timing or from counts outside the filtered views. Packs do not ship marked
items, because a marking needs the policy block; mark the ones you need. The
console does not yet display marking labels.

## Air-gapped use

No plan, approval or execution needs a model or a network. To use the AI
helpers on a closed network, point `ZYNTRA_AI_BASE_URL` at a local
OpenAI-compatible endpoint (see "Air-gapped model" in `REFERENCE.md`). Decisions
sit in a hash-chained audit log (`/api/v1/audit/verify`) and can be exported
with an Ed25519 signature.

## What Zyntra does not do yet

CUI marking, classification levels, ITAR data-residency enforcement and any
FedRAMP or CMMC certification. The packs read ids, statuses and counts only;
keep CUI out of the exports. Those controls would be core features, not packs.
