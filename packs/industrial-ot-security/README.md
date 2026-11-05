# OT security pack

Monitoring coverage, patch age, internet exposure and remote access for an industrial control network. It answers: *which control assets are exposed, unmonitored or unpatched, and what would monitoring, patching or isolating do to the posture?*

```bash
./bin/zyntra plan -f packs/industrial-ot-security
```

The data in `fixture/` is invented. Every action is advisory (noop); patching and firewall changes can stop production, so they need two approvers. Use `examples/policy-industrial.yaml` for the approval rules.

The pack follows the zones-and-conduits idea behind IEC 62443 but does not assign security levels, assess a system against the standard, or claim any certification. It reads asset ids, zones and counts only; keep vulnerability detail and credentials out of the exports.

Status: unvalidated against a real asset inventory.

## Rules

- Monitoring coverage must stay at or above 0.9.
- No high-criticality asset unpatched for over 180 days.
- No OT asset reachable from the internet.
- No remote session without approval.
