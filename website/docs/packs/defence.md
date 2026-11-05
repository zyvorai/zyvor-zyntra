---
sidebar_position: 1
---

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

## Live connector templates

`examples/defence/` shows how to read maintenance, supply and GRC exports over
HTTP instead of CSV files:

- `kpis-http.yaml`: KPIs from three paged JSON exports (`next`, `items`,
  `retries` on `http` sources; see [the source fields](/docs/core-concepts/model#model-reference-packs-and-ontology-additions)).
- `ontology-rest.yaml`: business objects from the same export through the
  `rest` connector.

They are templates, not connectors for any product: change the URLs, list
paths, paging links and field names to your systems. They are tested against
invented exports served by `zyntra fake-sources` (under `/defence`, bearer
token `fake-defence-token`) in the shapes common to REST APIs: a nested list
with a `links.next` URL, OData's `value` and `@odata.nextLink`, and a list with
`meta.next`. **Nothing has been run against a real maintenance, supply or GRC
system**, and none of them is GCSS-Army, DLA or any other named system. Expect
to adjust, and report what differs. Credentials stay in `${ZYNTRA_*}`
variables; each source reads only the host it started on.

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
OpenAI-compatible endpoint (see "Air-gapped model" in [Reference](/docs/reference)). Decisions
sit in a hash-chained audit log (`/api/v1/audit/verify`) and can be exported
with an Ed25519 signature.

## Egress controls

An `egress:` block in the policy file limits where Zyntra may send data or
fetch it from. It covers action webhooks, `http` and `sheet` KPI sources, the AI
endpoint, Fabric Keep and notifications.

```yaml
egress:
  default: deny                 # for any purpose no rule covers
  destinations:
    private: [10.0.0.0/8, "*.internal"]
  rules:
    - {name: webhooks-stay-private, purposes: [webhook, ai], allow: [private]}
```

- An entry is a host name, `*.suffix` (subdomains only), an IP or CIDR, or `*`.
- Purposes are `webhook`, `source`, `ai`, `keep` and `notify`. A purpose that a
  rule covers may reach only the groups those rules allow; others follow `default`.
- Checks happen before a connection is made. For a destination allowed only by
  network, the name is resolved once and the connection goes to an allowed
  address, so a DNS answer cannot redirect it. A redirect to another host is refused.
- At start, configured webhook and source URLs are checked, and a forbidden
  host stops Zyntra with a message naming the action or KPI. URLs whose
  `${ZYNTRA_*}` variables are not set yet are checked when used.
- A guarded client connects directly and ignores proxy environment variables.
- Without an `egress:` block nothing is restricted. `examples/policy-defence.yaml`
  restricts everything to private networks.

This controls where traffic may go. It does not look inside the data, and it
does not stop an approver from approving an action whose destination you
allowed.

## What Zyntra does not do yet

Data-residency rules that depend on what the data is (egress controls only
look at the destination), enforcement across separate instances, and any
FedRAMP or CMMC certification. Markings and clearance are enforced inside one
Zyntra instance; this is not a multi-level-secure system. The packs read ids,
statuses and counts only; keep CUI out of the exports.

For how Zyntra's existing controls line up with the NIST SP 800-171 families,
including the gaps, see [the control mapping](/docs/packs/control-mapping).
