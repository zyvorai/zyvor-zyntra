# Retained input events and quarantine

Webhook ingestion now retains a bounded event journal in the existing
`inputs.json` state file alongside the latest values. It remains a single-writer
store. Writes sync a temporary file, atomically rename it, and sync its directory
before acknowledging success. A failed write before rename does not advance
in-memory state. Existing state files load without migration commands.

## Delivery and ordering

Send the existing JSON body to `POST /api/v1/ingest/{channel}`. Optional headers:

- `Idempotency-Key`: up to 200 characters, unique within the channel. Retries with
  the same body reuse the original sequence. A different body, or a different
  explicitly supplied observation time, returns HTTP 409.
- `X-Zyntra-Observed-At`: RFC3339 timestamp. Defaults to server receipt time. An
  observation more than one minute in the future is rejected. An observation
  at or before the latest accepted time is retained as `late` and cannot replace
  the latest value or refresh its freshness.

A 202 response includes `receipt.sequence`, `receipt.status` and
`receipt.duplicate`. A duplicate does not trigger another refresh. The body hash
covers the normalized JSON document. The journal retains up to **1,000 events or
8 MiB of event body data across the pack**, whichever is reached first. Individual
events are capped at 1 MiB. Idempotency lasts only while the matching event is
retained; this is not an unbounded deduplication guarantee.

The existing `Ingest` Go API also records events, with no idempotency key. Manual
values use the same atomic persistence path but are not webhook events.

## Data-contract gate

Bind channels explicitly in `ingest-contracts.json`, next to the pack:

```json
{
  "orders": {
    "contract": "order-export",
    "records_field": "records",
    "observed_field": "updated_at"
  }
}
```

The named contract must exist in `data-contracts.json`. `records_field`, when
present, is one top-level property holding an array of objects or one object.
Without it, the body itself is the record or array. The contract checks required
values, types, unknown fields, composite uniqueness, completeness, freshness and
future observation timestamps. Invalid rule configuration also fails closed.

A failed batch is retained with status `quarantined`, returns HTTP 422, and never
changes the latest value or triggers a refresh. Diagnostics omit cell values.
The raw payload remains retained for authorized administrator investigation.
Correct a rejected source batch and send it using a new idempotency key. Reusing
the old key preserves the original receipt and does not reprocess it.

Channels without a binding retain their existing generic ingestion behavior.
This gate validates incoming batches; it does not infer units or reconcile
multiple sources. A rejected batch does not immediately invalidate a previous
good observation: that observation ages under its existing freshness policy.

## Replay/export

`GET /api/v1/inputs/{channel}/events?after=0&limit=100` returns events in sequence
order, `next`, `oldest` and `truncated`. `after` is exclusive and `limit` is 1..200.
Keep `next` as the consumer's checkpoint. If a prior cursor has expired,
`truncated=true`; do not mistake the remaining records for a complete history.
The oldest sequence is global to the pack, not necessarily to that channel.

Reading events never reapplies an observation. An external replay consumer can
process the retained documents with their original observation times. This is
not automatic delivery retry, an exactly-once message broker or a reconstructed
historical KPI/model timeline. For model replay use the planning snapshot.

Only administrators with visibility of every KPI using the channel can read its
event payloads. Tenant accounts cannot access this provider route. The Signals
page offers administrators a retained-event view. Payloads are not rendered in
the console table; the authenticated API supports authorized export.

Use the state volume's existing backup process. No PostgreSQL backend, shared
writer coordination or high-availability claim is introduced.
