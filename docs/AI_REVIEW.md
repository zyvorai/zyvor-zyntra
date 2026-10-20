# AI code review (zyntra)

_Generated 2026-10-20 by claude-opus-5-5 from a snapshot of tracked sources; verify before acting._

# Code review: zyntra

## Summary

Zyntra is a Go service with an embedded React console. It models business KPIs as a graph, finds the KPIs that miss target, simulates candidate actions through declared edges, and ranks them. Approved actions run through webhook, file, kubectl or noop adapters, gated by a policy engine with a hash-chained audit trail and Ed25519-signed decision records. Industry behaviour lives in "packs": directories of YAML and fixture CSVs. The repo also ships a Docusaurus site, a Helm chart and marketing assets.

## Bugs and correctness risks

**Quickstart fails as written.** `README.md` and `website/docs/getting-started/quickstart.md` both run:

```
git clone https://github.com/zyvorai/zyvor-zyntra.git && cd zyntra
```

The clone directory is `zyvor-zyntra`, so `cd zyntra` fails on the first command a new user runs.

**Go toolchain version.** `go.mod` declares `go 1.27` and CI requests `1.27.x`. `internal/gaps/gaps_test.go` uses `new(v)` with a non-type operand, which needs a recent toolchain.
- If 1.27 is not yet released where contributors build, local builds will fail.
- I can't confirm availability. Pin to a released version, or document the requirement.

**Module path and repo name disagree.**
- The module and `Chart.yaml` `sources` say `github.com/zyvorai/zyntra`.
- The README, CI badges and website say `zyvorai/zyvor-zyntra`.
- Several card footers (`docs/social/readme/*.html`) use the former, so either links or `go install` will be wrong.

**Direct dependencies marked `// indirect`.** `internal/ai/history_store.go` imports `modernc.org/sqlite` directly, yet `go.mod` marks it indirect. `go mod tidy` will rewrite this. CONTRIBUTING's claim that "the core needs only `gopkg.in/yaml.v3`" is also no longer true.

**`calendar.Compile` can panic on config input** (`internal/calendar/calendar.go`).
- It runs `strings.ToLower(d)[:min(3, len(d))]`, using the byte length of the *original* string.
- Lower-casing can shrink the string. For example, the Kelvin sign `K` is 3 bytes but lowers to `k`, 1 byte, so the slice is out of range.
- The slice can also cut a multi-byte rune.
- Prefix matching accepts any word that starts like a day (`"monkey"` → Monday).
- Fix: validate against the full day names or their exact 3-letter forms.

**Prometheus adapter accepts non-finite values** (`internal/adapters/prometheus/prometheus.go`).
- `strconv.ParseFloat` accepts `"NaN"` and `"+Inf"`, which Prometheus does return.
- These would flow into KPI values and the simulation.
- The adapter also never checks `resp.StatusCode` and reads the body without a size limit.

**Kubernetes adapter omits zero counts** (`internal/adapters/kubernetes/kubernetes.go`). `nodes_ready` is only created when at least one node is Ready. When no nodes are Ready (exactly the outage case), the metric is missing rather than `0`, so a KPI bound to it likely errors instead of reporting the outage.

**Inputs store keeps failed writes in memory** (`internal/inputs/inputs.go`).
- `Ingest` and `SetManual` mutate the in-memory state before `save()`.
- If the write fails, memory and disk diverge and the value survives until restart.
- The temp-file-plus-rename has no fsync.

**Executive overlay errors are silent.** `loadOverlay` in `cmd/zyntra/executive.go` and `Server.overlay` in `internal/api/executive.go` swallow YAML errors and return an empty overlay. A typo in `executive.yaml` silently drops all named decisions and assumptions. In addition:
- The file is re-read on every request.
- `overlay(m)` ignores `m`.
- `handleExecutiveSensitivity` builds the whole plan before validating that `action` was supplied.

**Ignored errors in ontology resolution.** `internal/ontology/resolve.go`'s `propose` ignores the error from `s.save()`.

**Console robustness issues.**
- `useOwner` (`web/src/hooks.ts`) calls `localStorage` without a try/catch. `pack.ts` and `theme.ts` guard this, so a page with blocked storage throws only here.
- In `useApi`, `loading` never returns to `true` when `path` changes, and it stays `true` forever when `path` is `null`.

**Example script builds JSON unsafely.** `examples/rollout/report.sh` builds JSON with `printf '%s'`. A note or site name containing `"` produces invalid JSON. Use `jq -n --arg`.

**Helm receiver "persistence" is not persistent.** In `deploy/helm/zyntra/templates/receiver.yaml`, `receiver.persistence` mounts an `emptyDir`, which is lost when the pod restarts. That defeats the idempotency memory tested in `examples/receiver/main_test.go`.

**Exec TLS files can be left mismatched.** `internal/tlsutil` regenerates the CA whenever the leaf needs renewal, because the CA key is not persisted. It writes the CA before the key and cert, so a partial failure can leave a CA on disk that doesn't sign the leaf.

## Security concerns

**Env-var expansion denylist misses a Zyntra secret.**
- `internal/envx/envx.go` blocks a fixed list of reserved names. `ZYNTRA_DEPLOY_TOKEN` (used by compose and the rollout API) is not on it, so a pack can expand it into a webhook URL or header.
- The denylist design means every future secret is exposed by default.
- `ZYNTRA_ERP_DSN` (which may contain a password) and `ZYNTRA_*_TOKEN` values for other products are also expandable.
- Consider an allowlist declared in `sources.example.yaml` `variables:`.

**Helm installs keep the public default password.** `templates/secret.yaml` generates random API, session, ingest, exec and deploy tokens, but *not* `ZYNTRA_ADMIN_PASSWORD`. A default `helm install` therefore exposes `admin` / `Admin@321` (a public password) behind whatever Service or Ingress you configure. Generate one, or refuse to start without one when not on loopback.

**`lookup` makes secrets unstable under GitOps.** `lookup` returns empty under `helm template` and ArgoCD-style rendering, so randomly generated secrets rotate on every sync. SECURITY.md mentions `existingSecret` for GitOps; the chart could enforce it.

**Exported decisions are verified against their own key.** `decisions.Verify` checks against the public key embedded in the export. Anyone can re-sign a tampered export with a fresh key. It's unclear whether `zyntra verify-decision` accepts an expected-key or fingerprint flag; if not, add one.

**CSRF protection is unverified from this code.** It relies on "every state-changing route is a JSON POST" with `SameSite=Lax`. That only holds if the server rejects non-`application/json` content types. I couldn't verify this from the files shown.

**Minor items.**
- `docs/social/readme/frame.html` assigns a query parameter to `innerHTML`. It's a build tool only, but trivially avoidable.
- `docker-compose.yml` uses an unpinned `ollama/ollama:latest`.
- GitHub Actions are pinned by tag, not SHA.
- The image workflow has `id-token: write` and `packages: write` on a job that also runs for pull requests. Steps are gated, but the permissions are not.

## Test and CI gaps

**Docs site is not built on pull requests.** `pages.yml` builds only on pushes to `main`, with `onBrokenLinks: 'throw'`. Broken docs links are therefore found after merge.

**Missing checks.**
- No TypeScript typecheck or lint step is visible.
- Shell is only checked with `bash -n`, not `shellcheck`.
- There is no `govulncheck` or `go mod tidy` diff check, which would catch the `// indirect` drift.

**Duplicate CI runs.** `ci.yml` triggers on both `push` and `pull_request` with no filter, so every PR branch runs twice.

**Missing tests for the issues above.**
- The calendar panic on non-ASCII day names.
- The Prometheus `NaN`/`Inf` values and non-200 responses.
- The Kubernetes zero-ready case.
- Overlay parse failures.
- `envx` coverage for `ZYNTRA_DEPLOY_TOKEN`.
- `inputs.Store` save-failure atomicity.

**Fragile tests.**
- `TestClusterSizedIngestIsFast` is a wall-clock test and may flake on shared runners.
- In the image workflow, `grep -q '"id": *"shop"'` depends on the JSON formatting of `/api/v1/meta`.

**Asset builds are not reproducible.** They require macOS `sips` and a local Chrome, so CI can't detect when committed JPEGs drift from their HTML sources.

## Documentation gaps

**Product naming is inconsistent:** Gravia vs Gryvia.
- Product spellings: `ZYNTRA_GRAVIA_URL` and "Gravia CRDs" vs `GryviaPriority`, `gryviapriorities` and "Gryvia" in the README.
- Ad hoc variants such as "GravIa" also appear in docs.

**Pack counts and lists are stale.**
- `docs/social/readme/packs.html` says "Six ship in the box"; the repo has eight in the README table, plus `leadership`.
- The README pack table omits `leadership`, which is only mentioned at the bottom.

**Pack READMEs don't follow the CONTRIBUTING template.** CONTRIBUTING requires each pack README to name three gaps and show a `zyntra simulate` trace. Most packs do not (logistics, payments, imaging-ops, sre, virtualization, leadership).

**Virtualization pack readiness is unclear.**
- Its README says "Status: unvalidated", yet the main README lists it as a ready-to-try pack.
- It has no fixture, so it's unclear what `plan -f packs/virtualization` does without `ZYNTRA_MACHINA_URL`.

**Outdated sections.**
- CONTRIBUTING's "Dependencies" section no longer matches `go.mod`: sqlite and `lib/pq` are present.
- `website/docs/core-concepts/decision-loop.md` still describes execution as `kubectl apply` and Gravia objects only. The Approvals UI test explicitly fixed that misconception elsewhere.

**Two parallel card build systems.** `docs/ux/build-readme-cards.sh` (`readme-*.html`) and `docs/social/readme/build.cjs` both exist, with no note on which one owns which README image.

## Top 5 recommended actions (ordered)

1. **Close the secret exposure paths.**
   - Add `ZYNTRA_DEPLOY_TOKEN` (and DSN-style variables) to the `envx` reserved list now. Then move to a per-pack allowlist.
   - Have the Helm chart generate or require `ZYNTRA_ADMIN_PASSWORD` instead of leaving `Admin@321` live.
2. **Fix the first-run experience and repo identity.**
   - Correct `cd zyntra` → `cd zyvor-zyntra` in the README and website.
