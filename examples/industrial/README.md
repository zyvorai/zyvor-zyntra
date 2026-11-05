# Industrial live-connector templates

A template for reading plant and process-safety exports over HTTP instead of
CSV files. It is a starting point, tested only against invented servers
(`zyntra fake-sources`), and not a connector for any named product. See
[docs/INDUSTRIAL.md](../../docs/INDUSTRIAL.md).

| File | What it is |
|---|---|
| `kpis-http.yaml` | Ten KPIs from three paged JSON exports, with the variables to bind. It includes `oee`, a derived KPI over three live inputs |

Try it with no real system:

```bash
zyntra fake-sources -addr 127.0.0.1:19700 &
export ZYNTRA_MES_URL=http://127.0.0.1:19700/industrial/mes \
       ZYNTRA_CMMS_URL=http://127.0.0.1:19700/industrial/cmms \
       ZYNTRA_SAFETY_URL=http://127.0.0.1:19700/industrial/safety \
       ZYNTRA_MES_TOKEN=fake-industrial-token ZYNTRA_CMMS_TOKEN=fake-industrial-token \
       ZYNTRA_SAFETY_TOKEN=fake-industrial-token
zyntra serve -f examples/industrial/kpis-http.yaml
```

Nothing here has been run against a real MES, maintenance or safety system.
Expect to adjust the paths and field names, and report what differs.
