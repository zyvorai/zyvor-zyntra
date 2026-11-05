# Defence sensors pack

Health of a sensor network and its links: uptime, track latency, track quality,
false-track load and spares. It answers: *which sites are thin because a sensor
is down or out of spares, and what would a repair crew or a recalibration do to
coverage?*

```bash
./bin/zyntra plan -f packs/defence-sensors
```

The data in `fixture/` is invented. This pack watches equipment only: it does
not classify, target or engage anything, and its actions ask people to service
equipment. Every action is advisory (noop). Recalibration takes a sensor
offline, so it needs two approvers and is blocked if coverage would fall by
more than ten points.

Status: unvalidated against a real sensor-management system.

## Rules

- Sensor uptime must stay at or above 0.95.
- Link availability must stay at or above 0.95.
