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
  `retries` on `http` sources; see "Source fields" in `REFERENCE.md`).
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
