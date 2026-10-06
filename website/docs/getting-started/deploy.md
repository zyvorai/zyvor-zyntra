---
sidebar_position: 3
---

# Deploy

Three ways to run it, same binary inside each. Dry-run is the default everywhere; set `ZYNTRA_EXECUTE=apply` only when the targets are real and approvers are named.

## Binary on a host (systemd)

```bash
./scripts/deploy-remote.sh 212.8.248.187 sus                 # gpu pack, live Netra/Gryvia/Fabric/Keep, smoke
./scripts/deploy-remote.sh 212.8.248.187 sus --pack shop     # any pack in packs/
./scripts/smoke-remote.sh                                     # re-run the smoke test against .deploy-last
./scripts/deploy-remote.sh 212.8.248.187 sus --uninstall
```

The script cross-compiles locally, validates the chosen pack, ships `packs/` and `examples/` to `/etc/zyntra`, and installs `zyntra.service` serving `/etc/zyntra/packs/<pack>`. It writes `/etc/zyntra/zyntra.env` (root:zyntra, 0640) from credentials already on the host (the Netra k8s secret, the Gravia API key, Fabric's admin password, the Keep token) and generates an ingest token; none are printed. It also adds the `zyntra-exec` credential and exec CA to Keep (after backing up Keep's env file) and signs the executor agent on your workstation. Options: `--pack`, `--port`, `--exec-port`, `--no-keep`, `--skip-web`, `--dry-run`, `--skip-smoke`. This is the install to use for kubectl actions and the Kubernetes adapters.

The smoke test logs in, posts any missing manual values, checks sources, gaps, plan and AI, then [analytics](/docs/core-concepts/analytics) (history persisting), [analytics queries](/docs/core-concepts/analytics#read-only-kpi-analytics-queries) (catalog, a 24-hour mean, rejection of an invalid query, Ask with `scope: "analytics"`), [investigations](/docs/core-concepts/investigations) (a 168-hour investigation of the first catalog metric, rejection of the target as its own candidate, Ask with `scope: "investigation"`) and [document knowledge](/docs/core-concepts/knowledge) (create, search and cited Ask on a temporary document it then deletes). Finally it proposes and approves one action. It prefers an action without a maintenance window; if the action it picks is held for its window, that counts as a pass and it says so.

- Sources the host has no credentials for (state `fallback`, shown as `not configured`, for example Gravia or Keep) are listed and skipped. Any other unhealthy source, or no healthy source at all, fails the run.
- Typed ontology actions get their required inputs from `/api/v1/ontology/schema`: the first object of the input's type that meets the action's `requires` checks. For `raise-inference-priority` in the gpu pack that is a `Service` with `tier: inference`.
- An approved proposal is revalidated before it runs. If it ends `blocked` only because its stale inputs come from skipped sources (for example `keep_sandbox_ready` on a host without Keep), that counts as a pass and it says so. Any other block fails with the proposal's `blocked_reasons`.

On the lab host, Keep sandboxes cannot reach the egress broker, so approved actions run locally and the audit records the executor as `zyntra (keep unavailable)`. The smoke test shows this line on purpose; set `keep: required` in the policy to block instead.

## Container (Docker or Podman)

Published images: `ghcr.io/zyvorai/zyntra` for linux/amd64 and linux/arm64. `:edge` and `:0.4.0-dev` track `main`, `:sha-<commit>` pins a build, and release tags add `:<version>`, `:<major>.<minor>` and `:latest`. Each image carries an SBOM, SLSA provenance and a keyless cosign signature:

```bash
docker pull ghcr.io/zyvorai/zyntra:edge
cosign verify ghcr.io/zyvorai/zyntra:edge \
  --certificate-identity-regexp 'https://github.com/zyvorai/zyntra/.github/workflows/image.yml@.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
gh attestation verify oci://ghcr.io/zyvorai/zyntra:edge -R zyvorai/zyntra
```

For an air-gapped site, `docker save` the pinned image and load it on the inside; nothing in it calls out.

```bash
make docker                                          # zyntra:<version>, docker or podman
docker run --rm -p 8080:8080 -e ZYNTRA_API_KEY=dev ghcr.io/zyvorai/zyntra:edge                 # shop pack
docker run --rm -p 8080:8080 -e ZYNTRA_API_KEY=dev ghcr.io/zyvorai/zyntra:edge serve -f packs/gpu
ZYNTRA_API_KEY=$(openssl rand -hex 24) docker compose up --build                    # shop + test receiver
```

The image is distroless, runs as uid 65532, and keeps state (proposals, audit chain, inputs, file actions) in the `/var/lib/zyntra` volume, so a read-only root filesystem works. It contains `zyntra`, `zyntra-receiver`, `packs/` and `examples/`. It does **not** contain kubectl. In a pod, Kubernetes connectors read through the service account, and approved `gravia.*` actions run through the Kubernetes API instead (server-side apply of Gryvia priorities and GPU sharing policies, a merge patch on Gryvia AI jobs, a delete only of objects labelled `app.kubernetes.io/managed-by=zyntra`); Helm `kubernetes.actions=true` grants exactly those writes. Outside a pod, kubectl actions and the Netra/Gravia Kubernetes adapters need the binary install. Packs with file, http, sheet, webhook-in and manual sources and webhook, file or noop actions work as is. To run your own pack, mount it and point `-f` at it (`-v $PWD/packs/mine:/app/packs/mine:ro ... serve -f packs/mine`).

[docker-compose.yml](https://github.com/zyvorai/zyntra/blob/main/docker-compose.yml) runs the shop pack with the receiver standing in for the POS and ERP (`http://127.0.0.1:9099`), both containers read-only with all capabilities dropped. Add `ZYNTRA_EXECUTE=apply` to watch an approved markdown arrive in the inbox.

## Kubernetes (Helm or plain manifest)

```bash
helm upgrade --install zyntra oci://ghcr.io/zyvorai/charts/zyntra --version 0.4.0 \
  -n zyntra --create-namespace --set pack=shop            # image ghcr.io/zyvorai/zyntra:0.4.0-dev
# or from a checkout: helm upgrade --install zyntra deploy/helm/zyntra ... --set image.repository=registry.internal/zyntra
kubectl -n zyntra port-forward svc/zyntra 8080:8080
kubectl -n zyntra get secret zyntra-auth -o jsonpath='{.data.ZYNTRA_API_KEY}' | base64 -d
```

The chart ([deploy/helm/zyntra](https://github.com/zyvorai/zyntra/blob/main/deploy/helm/zyntra/values.yaml)) runs one replica with a `Recreate` strategy, because proposals and the audit chain are file state with a single writer. It generates the API key, session secret, ingest token and exec token once and keeps them across upgrades, or uses `auth.existingSecret`. It also provides: a PVC for state (kept on uninstall), probes on `/healthz`, a non-root pod with a read-only root filesystem and no service account token, inline `policy` (or `policyExistingSecret`, a Secret's `policy.yaml`, for a policy with generated service-token hashes), extra `env`, optional Ingress and NetworkPolicy, and an optional test receiver (`receiver.enabled`). `modelPath` together with `extraVolumes` serves a pack from a ConfigMap or volume instead of the image.

Without Helm, use the rendered [deploy/kubernetes/zyntra.yaml](https://github.com/zyvorai/zyntra/blob/main/deploy/kubernetes/zyntra.yaml); its header shows the one `kubectl create secret` it needs. Regenerate it with `make k8s-manifest`; CI fails if it drifts from the chart.

For a k3s host with no registry, `deploy-k8s.sh` builds the image there with podman, imports it into containerd, installs the chart on a NodePort and runs the smoke test:

```bash
./scripts/deploy-k8s.sh 212.8.248.187 sus --pack shop     # http://212.8.248.187:30962
make deploy-k8s HOST=212.8.248.187 PACK=shop
```

Options: `--namespace`, `--node-port`, `--execute dry-run|apply`, `--no-receiver`, `--no-smoke`. The image tag is `<version>-<git sha>`, so each deploy rolls the pod.
