# Zyntra — large additive feature batch

Self-contained packages under `internal/` wired into the API and console where they add a new surface. Human approval remains required; dry-run by default; no LLM chooses or runs actions.

## Wired surfaces

- **schedules** — interval recurring plans; `schedules.json`; console Automations → Schedules; due ticks open pending proposals only
- **webhooks** — HMAC-signed outbound destinations; `webhooks.json`; console Automations → Webhooks; fans out on approval events (alongside `ZYNTRA_NOTIFY_URL`)
- **bundles** — versioned pack subset export/import; console Automations → Bundles; secrets never exported
- **templates** — reusable decision skeletons; `GET /api/v1/templates`, `POST …/propose`
- **activity** — principal last-seen; `activity.json`; `GET /api/v1/activity` (admin)
- **policyguard** — optional declarative rule checks at propose time (`Options.PolicyGuardRules`)
- **planning** — decision planning workbench (portfolios, financial assumptions, replay, model review, timelines, evidence); Decide → Decision planning
- **input events** — retained webhook journal with idempotency, late observations and contract quarantine; see `docs/INPUT_EVENTS.md`

## Helper libraries (no duplicate HTTP)

These complement existing packages (`scenario`, `outcome`, `notify`, `ai.Forecasts`, `executive` digest, signed decision export, `review.Similar`, semantic lineage) and are available for callers without forking those routes:

- analytics — hit-rate / outcome summary helpers (distinct from `/api/v1/analytics/*` KPI queries)
- scenarios — compare what-if KPI scores
- outcomes — predicted vs actual absolute error
- lineage — source → KPI → action → decision trail
- precedents — cosine similarity on gap signatures
- digest — one-paragraph executive summary helper
- constraints — hard min/max limits before ranking
- forecast — simple linear time-to-breach
- export — JSON decision record shape for offline verification

All packages include SPDX headers. They respect the product-plan non-negotiables.
