# Semantic contracts, lineage and decision confidence

Zyntra already owns the decision loop: sense, find the gap, simulate, rank,
approve, act and verify. This feature makes the evidence behind that loop
explicit without turning Zyntra into a warehouse or allowing an AI model to
make operational decisions.

## Data contracts

A pack may add `data-contracts.json`.

```json
[
  {
    "name": "orders",
    "owner": "commerce",
    "source": {"kind": "sql", "reference": "orders"},
    "object_type": "Order",
    "schema": [
      {"name": "order_id", "type": "string", "required": true},
      {"name": "amount", "type": "number"}
    ],
    "quality": {
      "freshness": "5m",
      "completeness": 0.999,
      "unique": ["order_id"]
    },
    "kpis": ["fulfilment_latency", "revenue"]
  }
]
```

Contracts contain no credentials. DSNs, tokens and passwords stay in the
existing connector/environment configuration.

## API

After applying `patches/api-wiring.diff`:

- `GET /api/v1/semantic/catalog` — permission-safe contract catalog.
- `GET /api/v1/semantic/lineage` — source → contract → object type → KPI graph.
- `GET /api/v1/semantic/resources` — stable `zyntra://...` resources for agents.
- `POST /api/v1/semantic/contracts/validate` — strict contract validation.
- `POST /api/v1/semantic/confidence` — explainable evidence confidence.

The confidence endpoint expects normalized values:

```json
{
  "freshness": 0.95,
  "data_quality": 0.99,
  "ontology_confidence": 0.90,
  "calibration": 0.87,
  "edge_confidence": 0.91,
  "prediction_accuracy": 0.93
}
```

The score is a weighted geometric mean, so one weak evidence dimension cannot
be hidden by several strong dimensions. It is evidence metadata only: it does
not rank, approve or execute actions.

## Agent resources

The semantic layer advertises protocol-neutral stable resources:

`zyntra://objects`, `zyntra://relationships`, `zyntra://kpis`,
`zyntra://lineage`, `zyntra://incidents`, `zyntra://plans`,
`zyntra://simulations`, and `zyntra://decisions`.

A later MCP transport can expose these URIs without changing the underlying
semantic model or decision engine.
