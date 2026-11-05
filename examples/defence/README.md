# Defence live-connector templates

Templates for reading maintenance, supply and GRC exports over HTTP. They are
starting points, tested only against invented servers (`zyntra fake-sources`).
See [docs/DEFENCE.md](../../docs/DEFENCE.md#live-connector-templates).

| File | What it is |
|---|---|
| `kpis-http.yaml` | KPIs from three paged JSON exports, with the variables to bind |
| `ontology-rest.yaml` | Work orders as business objects through the `rest` connector |

Try them with no real system:

```bash
zyntra fake-sources -addr 127.0.0.1:19700 &
export ZYNTRA_MAINT_URL=http://127.0.0.1:19700/defence/maintenance \
       ZYNTRA_SUPPLY_URL=http://127.0.0.1:19700/defence/supply \
       ZYNTRA_GRC_URL=http://127.0.0.1:19700/defence/grc \
       ZYNTRA_MAINT_TOKEN=fake-defence-token ZYNTRA_SUPPLY_TOKEN=fake-defence-token \
       ZYNTRA_GRC_TOKEN=fake-defence-token
zyntra serve -f examples/defence/kpis-http.yaml
```
