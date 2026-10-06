---
sidebar_position: 1
---

# Quickstart

Zyntra is one Go binary with an embedded console. Building from source needs **Go** and
**Node 22** (for the console in `web/`). No cluster, database or model is required.

## A ranked plan from a CSV

```bash
git clone https://github.com/zyvorai/zyntra.git && cd zyntra
make build                                          # console (web/, Node 22) and the Go binary
./bin/zyntra plan -f packs/shop                     # a ranked plan from CSV fixtures, no cluster needed
```

`plan` ranks a reorder and refuses a markdown that would break the shop's margin invariant.
Try the other commands against the same pack:

```bash
./bin/zyntra gaps -f packs/shop                                 # KPIs missing target, worst first
./bin/zyntra simulate -f packs/shop -action reorder_fast_movers # predicted change to every KPI, with ranges
./bin/zyntra pack validate packs/shop                           # check a pack against its fixture
```

## Open the console

```bash
ZYNTRA_API_KEY=dev ./bin/zyntra serve -f packs/shop   # console and API on :8080
```

Open **http://127.0.0.1:8080** and sign in as **admin / Admin@321**. That account exists only
while the policy file defines no users, and the console shows a banner until you set
`ZYNTRA_ADMIN_PASSWORD`. See [Console and sign-in](./console.md) for every page and role.

## Or run the container

```bash
docker run --rm -p 8080:8080 -e ZYNTRA_API_KEY=dev ghcr.io/zyvorai/zyntra:edge
```

Images are multi-arch (amd64 and arm64) and cosign-signed, with an SBOM and provenance. For
Compose, Helm, k3s and systemd installs, see [Deploy](./deploy.md).

## Packs for your industry

| Industry | Pack | Try it |
|---|---|---|
| Retail | `packs/shop` | `./bin/zyntra plan -f packs/shop` |
| Manufacturing | `packs/manufacturing` | `./bin/zyntra plan -f packs/manufacturing` |
| Logistics | `packs/logistics` | `./bin/zyntra plan -f packs/logistics` |
| Payments | `packs/payments` | `./bin/zyntra plan -f packs/payments` |
| AI infrastructure | `packs/gpu` | `./bin/zyntra plan -f packs/gpu` |
| Imaging operations | `packs/imaging-ops` | `./bin/zyntra plan -f packs/imaging-ops` |

A pack is a directory of files: `pack.yaml`, `kpis.yaml`, `sources.example.yaml`, a README
and a `fixture/` of sample exports. `zyntra pack draft -industry TEXT -sample FILE` drafts one
from your own export. The newer packs carry declared starting weights, not measured ones:
calibrate them against your own outcomes before you lean on the ranking. See
[The model and packs](../core-concepts/model.md).

## Next steps

- Connect live sources (Prometheus, SQL, REST, Kubernetes, webhooks): [The model and packs](../core-concepts/model.md)
- Set an approval policy and SSO: [Approvals and the decision loop](../core-concepts/decision-loop.md)
- Talk to us about production: [book a demo](https://zyvor.dev/schedule?utm_source=github-pages&utm_medium=zyntra&utm_campaign=docs_quickstart) or [start a 30-day PoC](https://zyvor.dev/poc?utm_source=github-pages&utm_medium=zyntra&utm_campaign=docs_quickstart)
