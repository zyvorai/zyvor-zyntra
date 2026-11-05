# Control mapping: NIST SP 800-171 Rev. 2 families

This maps the security controls Zyntra has today to the 14 control families of
NIST SP 800-171 Rev. 2, the set behind a CMMC Level 2 assessment. It is a
working aid for your own System Security Plan. It is **not** an assessment, a
certification, or a claim that Zyntra or your deployment is compliant: that
needs your environment, your policies and an independent assessor.

Status words:

- **provided**: Zyntra implements it; the evidence column points at the code.
- **partial**: Zyntra covers part of it; the note says what is missing.
- **delegated**: Zyntra relies on something you run around it (identity
  provider, ingress, platform).
- **gap**: Zyntra does not do this. You need another control.

Code references are file paths, checked by `internal/docs/controlmap_test.go`
so they cannot silently go stale. Read `SECURITY.md` for the deployment
defaults these rows assume.

## 3.1 Access control (AC)

| Control | Status | Evidence and notes |
|---|---|---|
| Role-based access: viewer, proposer, approver, executor, admin | provided | `internal/auth/auth.go`; routes are gated per role in `internal/api/api.go` |
| Separation of duties on changes | provided | Two-person approval and `distinctFromProposer` in `internal/policy/policy.go`, recorded in `internal/approvals/approvals.go` |
| Least privilege for machine credentials | provided | Service tokens (viewer and proposer only) in `internal/auth/service.go`; connector tokens scoped to tenants and object types, with expiry, in `internal/auth/connector.go` |
| Tenant isolation, deny by default | partial | `internal/api/tenant.go`. The KPI graph is shared across tenants, and admin and executor are deployment-wide (`docs/ONTOLOGY.md`, "Tenants and connector credentials") |
| Limit access to sensitive object properties | partial | The ontology `sensitive` flag is admin-only. There are no clearance levels or markings yet |
| Session lock, session termination | partial | Signed cookie with a 12 hour lifetime (7 days with remember-me) in `internal/auth/auth.go`. There is no idle timeout |
| Control remote access, encrypt remote sessions | delegated | The console listener is plain HTTP; terminate TLS at your ingress (`SECURITY.md`) |
| Control the flow of CUI | partial | Optional `markings:` and `egress:` policy blocks (see DEFENCE.md) hide marked data from lower clearances and limit outbound destinations. Enforced inside one instance only; see "Out of scope today" below |

## 3.2 Awareness and training (AT)

| Control | Status | Evidence and notes |
|---|---|---|
| Security awareness and role training | gap | Organizational. Not something software provides |

## 3.3 Audit and accountability (AU)

| Control | Status | Evidence and notes |
|---|---|---|
| Create and retain audit records tied to a user | provided | Every state change is an event with subject, time and phase in `internal/approvals/approvals.go` |
| Protect audit information from modification | partial | Hash-chained log, checked by `GET /api/v1/audit/verify` (`VerifyChain` in `internal/approvals/approvals.go`, tests in `internal/approvals/audit_file_test.go`). It detects edits; it cannot stop someone deleting the whole file, so back it up and protect `ZYNTRA_STATE_DIR` |
| Non-repudiation of decisions | provided | Ed25519-signed decision export in `internal/decisions/decisions.go`; the key is stored with mode 0600 |
| Correlate and review audit records | partial | Audit and decision views exist in the console. There is no SIEM forwarder and no structured application log: output is standard `log` lines from `cmd/zyntra/main.go` |
| Time stamps | delegated | Zyntra uses the host clock; run NTP on the host |

## 3.4 Configuration management (CM)

| Control | Status | Evidence and notes |
|---|---|---|
| Baseline configuration, change control | provided | Packs and policy are files under version control; every action is a proposal that needs approval |
| Test changes before applying | provided | Dry-run is the default (`ZYNTRA_EXECUTE`, `cmd/zyntra/main.go`); `internal/executor/generic.go` sends webhooks only on apply, with an idempotency key |
| Restrict software and component use | partial | Distroless, read-only image in `Dockerfile` and `deploy/helm/zyntra/values.yaml`. Packs may only read `ZYNTRA_*` variables (`internal/envx/envx.go`) |
| Least functionality | provided | Execution disabled unless set to apply; AI off unless an endpoint is configured; file actions confined to `ZYNTRA_OUTPUT_DIR` |

## 3.5 Identification and authentication (IA)

| Control | Status | Evidence and notes |
|---|---|---|
| Identify and authenticate users | provided | Local bcrypt accounts and OIDC with PKCE (`internal/auth/auth.go`, `internal/auth/oidc.go`) |
| Multi-factor authentication | delegated | Not in the code. Use an OIDC provider that enforces MFA |
| Replay-resistant authentication | partial | OIDC uses a nonce and state cookie (`internal/auth/oidc.go`). Local accounts use a password only |
| Password strength, lockout | partial | Failed sign-ins at `POST /api/v1/session` are counted per account and per client address and lock them for a while, audited and logged (`internal/auth/throttle.go`); on by default, in memory, and not applied to bearer tokens. There is still no password-strength policy; enforce it at the identity provider |
| Default credentials | partial | A built-in `admin` account exists until you set `ZYNTRA_ADMIN_PASSWORD` or define users; the console warns (`SECURITY.md`) |

## 3.6 Incident response (IR)

| Control | Status | Evidence and notes |
|---|---|---|
| Incident handling capability, reporting | gap | Organizational. The audit log and decision export support an investigation |

## 3.7 Maintenance (MA)

| Control | Status | Evidence and notes |
|---|---|---|
| Control maintenance tools and sessions | gap | Organizational and platform |

## 3.8 Media protection (MP)

| Control | Status | Evidence and notes |
|---|---|---|
| Protect and sanitize media, mark CUI | gap | Zyntra stores state in `ZYNTRA_STATE_DIR`; encrypt the volume and sanitize it yourself. There is no CUI marking |

## 3.9 Personnel security (PS)

| Control | Status | Evidence and notes |
|---|---|---|
| Screening, termination | gap | Organizational |

## 3.10 Physical protection (PE)

| Control | Status | Evidence and notes |
|---|---|---|
| Physical access and monitoring | gap | Organizational and facility |

## 3.11 Risk assessment (RA)

| Control | Status | Evidence and notes |
|---|---|---|
| Assess risk, scan for vulnerabilities | partial | `defence-compliance` pack tracks open vulnerability age as a KPI; CI runs `go vet` and tests (`.github/workflows/ci.yml`) but no vulnerability scanner such as govulncheck or trivy |

## 3.12 Security assessment (CA)

| Control | Status | Evidence and notes |
|---|---|---|
| Assess controls, plans of action, monitor | partial | The `defence-compliance` pack models POA&M items and stale evidence. It tracks readiness; it does not assess anything |

## 3.13 System and communications protection (SC)

| Control | Status | Evidence and notes |
|---|---|---|
| Encrypt in transit | partial | TLS 1.2 minimum and a private CA for the Keep listener (`internal/tlsutil/tlsutil.go`); outbound clients verify certificates unless you set `ZYNTRA_ENDPOINT_INSECURE`. The main console listener is HTTP, so terminate TLS at the ingress |
| Boundary protection, deny by default network | partial | Optional `networkPolicy` in `deploy/helm/zyntra/values.yaml`. Optional `egress:` policy restricts webhook, source, AI, Keep and notify destinations (`internal/egress`), checked at dial time; with no `egress:` block, configured URLs are followed as before |
| Protect secrets | provided | Only `${ZYNTRA_*}` variables expand, Zyntra's own credentials are reserved (`internal/envx/envx.go`); credentials are not sent to a different host on redirect or paging (`internal/connector/rest.go`, `internal/notify/notify.go`) |
| Cryptographic key management | partial | Signing key and session secret are generated and kept in the state dir; there is no rotation tooling or HSM support |
| Network disconnect, air-gap | provided | No plan, approval or execution needs a network or a model (`docs/DEFENCE.md`) |

## 3.14 System and information integrity (SI)

| Control | Status | Evidence and notes |
|---|---|---|
| Flaw remediation, malicious code protection | partial | Images are signed with cosign and carry build provenance (`.github/workflows/image.yml`); the image is distroless with a read-only root filesystem. No standalone SBOM file is published, only the buildx attestation |
| Monitor systems, alert on anomalies | partial | KPI watches and the freshness check block approval on stale inputs (`internal/freshness`); there is no host or network intrusion monitoring |
| Input validation | provided | Strict YAML decoding of packs and policy; request bodies are size-capped |

## Out of scope today

Markings, clearance and the egress policy are off unless the policy file turns
them on, and they are enforced inside one instance only. Zyntra is not a
multi-level-secure system and carries no accreditation. Cross-instance
enforcement and certification remain out of scope. Keep CUI out of pack exports
unless markings are configured, and run Zyntra inside the boundary that already
protects your CUI.
