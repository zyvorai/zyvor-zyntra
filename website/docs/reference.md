---
sidebar_position: 3
---

# Reference

Configuration variables, the CLI and the HTTP API. Feature-specific routes are documented on their own pages: [analytics](/docs/core-concepts/analytics), [investigations](/docs/core-concepts/investigations), [watches](/docs/core-concepts/watches), [knowledge](/docs/core-concepts/knowledge) and the [ontology](/docs/core-concepts/ontology).

## Configuration

| Variable | Purpose |
|----------|---------|
| `ZYNTRA_API_KEY` | Break-glass admin access key and Bearer token |
| `ZYNTRA_ADMIN_USER` / `ZYNTRA_ADMIN_PASSWORD` | Built-in local admin when the policy file has no users (default `admin` / `Admin@321`, flagged in the console until changed). `ZYNTRA_DEFAULT_ADMIN=off` removes it, and with no key, OIDC or users the console then runs open (dev only) |
| `ZYNTRA_POLICY` | Policy file (same as `serve -policy`) |
| `ZYNTRA_SESSION_SECRET` | Key for session cookies (defaults to the access key; set it so sign-ins survive key rotation and restarts) |
| `ZYNTRA_OIDC_ISSUER`, `_CLIENT_ID`, `_CLIENT_SECRET` | OpenID Connect sign-in |
| `ZYNTRA_OIDC_ROLE_MAP` | `group=role[+role],…` mapping from IdP groups to Zyntra roles |
| `ZYNTRA_OIDC_REDIRECT_URL`, `_GROUPS_CLAIM`, `_SCOPES`, `_DEFAULT_ROLE` | Optional: callback URL (default derived from the request), groups claim (`groups`), scopes, role for unmapped users |
| `ZYNTRA_LISTEN`, `ZYNTRA_STATE_DIR` | Listen address; directory for decisions, history, the decision signing key and exec TLS |
| `ZYNTRA_DEFAULT_PACK` | With `serve -f a,b` (several packs), which pack requests without `X-Zyntra-Pack` use (default: the first) |
| `ZYNTRA_NETRA_URL` / `_TOKEN` | Netra API (eBPF metrics and health) |
| `ZYNTRA_GRAVIA_URL` / `_TOKEN` | Gryvia API (GPU cluster, quota, costs) |
| `ZYNTRA_FABRIC_URL` / `_USER` / `_PASSWORD` or `_TOKEN` | Fabric host metrics (logs in for a token) |
| `ZYNTRA_ENDPOINT_INSECURE=1` | Accept self-signed certificates on the endpoints above (lab) |
| `ZYNTRA_EXECUTE` | `dry-run` (default) or `apply` |
| `ZYNTRA_OUTPUT_DIR` | Where `file` actions write (default `$ZYNTRA_STATE_DIR/out`) |
| `ZYNTRA_INGEST_TOKEN` | Token that may only POST to `/api/v1/ingest/<channel>` |
| `ZYNTRA_KUBECONFIG` | kubeconfig for execution and the `-kubectl` adapter |
| `ZYNTRA_APPROVAL_MODE` | `local` (default) or `keep` |
| `ZYNTRA_KEEP_URL` / `_TOKEN` | Keep agent runtime (status, sessions, approvals, audit) |
| `ZYNTRA_EXEC_TOKEN`, `ZYNTRA_EXEC_TLS_ADDR` | Token Keep injects, and the loopback TLS listener it calls |
| `ZYNTRA_AI_BASE_URL` / `_API_KEY` / `_MODEL` | Optional OpenAI-compatible model for answer rewriting |
| `ZYNTRA_NOTIFY_URL` / `_TOKEN` / `_ON`, `ZYNTRA_CONSOLE_URL` | Optional webhook (Slack/Teams-compatible `text`) that says a proposal needs a decision; `_ON` lists statuses (default `pending`). Sends action, status and who, never inputs or free text; best effort, never blocks an approval |

## CLI

| Command | What it does |
|---------|--------------|
| `zyntra graph` | KPIs, targets, owners, sources, dependencies and actions |
| `zyntra gaps [-owner NAME]` | KPIs missing target, worst first |
| `zyntra simulate -action ID[+ID]` | Predicted KPI changes with ranges, constraint breaches and the propagation trace |
| `zyntra plan [-owner NAME]` | Actions and pairs ranked with confidence, then those waiting on a precondition and those blocked by constraints or invariants |
| `zyntra pack list\|validate [DIR]` | List packs under `packs/`, or check one against its fixture |
| `zyntra pack draft -industry TEXT -sample FILE [-out DIR]` | Draft a pack from CSV or JSON samples, then validate it |
| `zyntra serve [-policy FILE]` | Console, REST API and SSE pulse |
| `zyntra verify-decision FILE` | Check a signed decision export offline |
| `zyntra hash-password < pw` | bcrypt hash for a local account in the policy file |
| `zyntra keep deploy\|pubkey\|credential` | Fabric Keep executor agent |
| `zyntra fake-sources` | Fake Netra/Gryvia/Fabric/Keep endpoints for development |
| `zyntra exec-token` | Random token for `ZYNTRA_EXEC_TOKEN` |

Common flags: `-f FILE|PACK_DIR`, `-o text|json`, `-prometheus URL`, `-kubectl`, `-kubeconfig FILE`. `serve` also takes `-addr`, `-interval` and `-policy`.

## API

All routes except `/healthz`, `/api/v1/meta`, sign-in and the OIDC redirects need a session cookie or a Bearer key. The role column is the minimum role.

| Method | Path | Role | Returns |
|--------|------|------|---------|
| `GET` | `/healthz` | — | `{"status":"ok"}` |
| `GET` | `/api/v1/meta` | — | Version, host, model, pack, source health, modes and sign-in methods |
| `POST`/`DELETE` | `/api/v1/session` | — | Sign in (`{operator, token}` or `{username, password}`) and out; `GET /api/v1/whoami` returns subject and roles |
| `GET` | `/api/v1/auth/oidc/login`, `/callback` | — | OIDC sign-in redirects (when configured) |
| `GET` | `/api/v1/graph`, `/gaps`, `/plan`, `/sources`, `/freshness`, `/policy` | viewer | Model with version and constraints, gaps, ranked and blocked actions, source health, per-KPI freshness, effective policy. `/gaps` and `/plan` take `?owner=` |
| `POST` | `/api/v1/simulate` | viewer | `{"action":"a"}`, `{"action":"a+b"}`, `{"actions":[…]}` or `{"custom":{...}}` |
| `GET` | `/api/v1/kpis/{id}/history` | viewer | Recorded values |
| `POST` | `/api/v1/kpis/{id}/value` | proposer | Enter a value for a `manual` KPI (`{"value":2,"reason":"..."}`), audited |
| `GET` | `/api/v1/inputs` | viewer | Manual KPIs and webhook-in channels with their last entry |
| `POST` | `/api/v1/ingest/{channel}` | ingest | JSON document for a `webhook-in` channel (ingest token or admin) |
| `GET`/`POST` | `/api/v1/ai/status`, `/digest`, `/insights`, `/ask`, `/explain` | viewer | Grounded AI (`/digest?owner=&window=` for one owner's shift note) |
| `GET` | `/api/v1/ai/edges`, `/ai/contradictions` | viewer | Proposed edges (not in the model) and pack README rules the plan breaks |
| `POST` | `/api/v1/ai/pack-draft` | proposer | `{"industry":"…","samples":[{"name":"stock.csv","content":"…"}]}` returns drafted files, refusals and validation; nothing is written |
| `GET` | `/api/v1/proposals/{id}/explanation`, `/proposals/{id}/similar`, `/similar?action=` | viewer | Why a run landed or missed (409 before a verdict), and precedents |
| `GET` | `/api/v1/proposals`, `/proposals/{id}` | viewer | Approval inbox |
| `POST` | `/api/v1/proposals` | proposer | Propose `{"action":"a"}` or `{"actions":["a","b"]}` |
| `POST` | `/api/v1/proposals/{id}/approve`, `/reject` | approver | Record an approval (202 until the quorum is met) or reject |
| `POST` | `/api/v1/exec/{id}` | executor | Revalidate and run an approved proposal (Keep's broker uses the exec token) |
| `GET` | `/api/v1/decisions`, `/decisions/{id}`, `/decisions/{id}/export` | viewer | Decision records, one with its audit events, signed export |
| `GET` | `/api/v1/audit`, `/audit/verify` | viewer | Hash-chained audit trail and its verification |
| `GET` | `/api/v1/keep/status`, `/approvals`, `/receipts`, `/audit` | viewer | Fabric Keep views (`POST /keep/approvals/{id}` needs approver) |
| `GET` | `/api/v1/events` | viewer | Server-sent `pulse` events every interval |

## Develop

```bash
make check      # gofmt, vet, unit tests, build
make test-e2e   # CLI + API + console smoke test against fake sources
zyntra pack validate packs/shop   # check a pack against its fixture
cd web && ZYNTRA_DEV_API=http://127.0.0.1:8080 npm run dev   # console with hot reload
make docker     # container image (docker or podman)
make helm-lint  # lint the chart and render it with every option on
```
