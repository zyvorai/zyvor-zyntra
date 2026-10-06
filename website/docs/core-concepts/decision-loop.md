---
sidebar_position: 2
---

# Approvals and the decision loop

![Approval lane — propose, approve, execute in Keep, verify](/img/readme-safety.jpg)

1. **Propose** an action (or a pair) from the plan. Zyntra captures a decision record: the input values with their freshness and source health, the model version, the full simulation with bands, the alternatives it considered, the effective policy, and the change it would make (a `GryviaPriority`, `GryviaGPUSharingPolicy` or Job suspend). Proposals that would breach a hard constraint are refused.
2. **Approve** or reject it. The policy decides how many distinct approvers are needed and whether the proposer may be one of them. Pending proposals expire (24 h by default) and approvals hold for a limited time (1 h by default).
3. **Revalidate.** Right before running, Zyntra takes a fresh snapshot and re-simulates. It blocks execution, and records why, if a required input is stale, a constraint would now be breached, the predicted improvement has fallen by more than `maxDrift` (50% by default), the model changed, the approval expired, or it is outside the action's maintenance window. Approvals outside their window wait and run when it opens.
4. **Execute:** `kubectl apply --dry-run=server` by default (`ZYNTRA_EXECUTE=apply` to apply for real).
5. **Observe.** After a real apply Zyntra watches the outcome: `verified` when the success criteria hold for enough consecutive fresh samples, `regressed` when a guardrail KPI worsens past its tolerance or a constraint breaks, `missed` when the window ends without success, `inconclusive` when the data was stale. A regression opens a linked **rollback proposal** that goes through the same approval policy (Gravia priority and MIG policies are deleted, suspended jobs resumed).

Every transition is appended to a hash-chained audit log (`GET /api/v1/audit/verify` checks it). `GET /api/v1/decisions/{id}/export` returns the decision and its audit events signed with Ed25519 (the key lives in `$ZYNTRA_STATE_DIR/decision-signing.key`); `zyntra verify-decision FILE` checks an export offline. State from v0.2 is migrated on first start.

## Policy

`zyntra serve -policy policy.yaml` (or `ZYNTRA_POLICY`) sets approval rules; see [examples/policy.yaml](https://github.com/zyvorai/zyvor-zyntra/blob/main/examples/policy.yaml). Without a file, one approval is enough, as in v0.2.

```yaml
maintenanceWindows:
  weeknights: {days: [mon, tue, wed, thu, fri], start: "22:00", end: "06:00", timezone: Europe/Berlin}
rules:
  - name: high-risk-two-person
    match: {risk: [high]}          # also: actions, adapters
    approvals: 2
    distinctFromProposer: true
    maintenanceWindows: [weeknights]
    approvedExpiry: 12h
  - name: gravia-through-keep
    match: {adapters: [gravia]}
    keep: required                 # required | preferred (default) | off
    requireFresh: true
revalidation: {maxDrift: 0.5}
users:                             # optional local accounts
  - {name: ana, passwordHash: "$2a$12$...", roles: [approver]}
```

Later rules override earlier ones, an action's own `policy:` block overrides both, and for a pair the strictest setting of each kind wins. With `keep: required`, Zyntra blocks the proposal instead of falling back to local execution when Keep is unavailable.

With `ZYNTRA_APPROVAL_MODE=keep`, approved proposals run through **Fabric Keep**:

1. Keep starts a signed `zyntra-executor` agent in a FluxVM sandbox.
2. The agent calls Zyntra's loopback TLS exec endpoint using the brokered `zyntra-exec` credential. Keep holds the egress approval, and Zyntra decides it, so every execution has a Keep receipt and a hash-chained audit entry.
3. Rejected proposals are mirrored into Keep's audit as denied approvals.

If Keep can't start the session before an approval exists, the already-approved proposal runs locally (unless policy says `keep: required`), and the audit records the executor as `zyntra (keep unavailable)`.

```bash
zyntra keep pubkey                       # signer public key (add it to ZYVOR_AGENT_POLICY_TRUSTED_SIGNERS)
ZYNTRA_KEEP_TOKEN=... zyntra keep deploy -url http://127.0.0.1:9096   # sign + deploy; the seed never leaves this machine
zyntra keep credential                   # zyntra-exec descriptor for ZYVOR_AGENT_CREDENTIALS_FILE
```
