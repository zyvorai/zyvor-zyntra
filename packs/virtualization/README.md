# Virtualization fleet pack (Machina + Atlas)

Local-first decision pack: reads fleet health from Machina and storage headroom from
Zyvor Atlas through Machina's single Prometheus scrape, proposes actions into Machina's
Zyra approvals queue. Nothing here touches a public cloud.

| KPI | Source gauge |
|---|---|
| `hosts_offline` | `machina_platform_hosts_offline` |
| `tasks_pending` | `machina_platform_tasks_pending` |
| `atlas_up` | `machina_atlas_up` |
| `atlas_days_to_full` | `machina_atlas_days_to_full` (absent while capacity is not growing) |
| `atlas_alerts_open` | `machina_atlas_alerts_open` |

Setup: create an operator-role Machina API key, then set `ZYNTRA_MACHINA_URL` and
`ZYNTRA_MACHINA_TOKEN` on the Zyntra host. Execution stays dry-run until
`ZYNTRA_EXECUTE=apply`; approved actions land in Machina as pending Zyra actions, where
Machina's own approval rules (two-person for JIT and network policy) still apply.

Status: unvalidated. Load it with `zyntra` on a Linux host before use.
