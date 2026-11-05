# Aviation live-connector templates

A template for reading flight-operations and maintenance exports over HTTP
instead of CSV files. It is a starting point, tested only against invented
servers (`zyntra fake-sources`), and not a connector for any named product. See
[docs/AVIATION.md](../../docs/AVIATION.md).

| File | What it is |
|---|---|
| `kpis-http.yaml` | Eight KPIs from three paged JSON exports, with the variables to bind. It uses `where` comparisons, `p90`, and a `timeField`/`within` trailing window |

Try it with no real system:

```bash
zyntra fake-sources -addr 127.0.0.1:19700 &
export ZYNTRA_OPS_URL=http://127.0.0.1:19700/aviation/ops \
       ZYNTRA_MRO_URL=http://127.0.0.1:19700/aviation/mro \
       ZYNTRA_OPS_TOKEN=fake-aviation-token ZYNTRA_MRO_TOKEN=fake-aviation-token
zyntra serve -f examples/aviation/kpis-http.yaml
```

Nothing here has been run against a real operations-control or maintenance
system. Expect to adjust the paths and field names, and report what differs.
