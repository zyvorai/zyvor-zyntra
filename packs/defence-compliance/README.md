# Defence compliance pack

Evidence-based readiness against NIST SP 800-171 Rev. 2, the control set behind
a CMMC Level 2 assessment (110 controls; the fixture holds 10 as a sample). It
answers: *which controls are held up by open plan items or stale evidence, and
what would closing items or re-collecting evidence do to readiness?*

```bash
./bin/zyntra plan -f packs/defence-compliance
```

The data in `fixture/` is invented. This pack tracks readiness; it does not
certify anything. The control list and the Rev. 2 / Rev. 3 transition should
be checked against the current CMMC rule and NIST publications before use.
Every action is advisory (noop). The emergency patch needs two approvers.

Status: unvalidated against a real GRC tool.

## Rules

- Controls implemented must stay at or above 1.
- CUI store access anomaly rate must stay at or below 0.
- Oldest open critical vulnerability must stay at or below 15.
