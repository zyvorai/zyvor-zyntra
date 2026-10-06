---
sidebar_position: 7
---

# Document knowledge and cited Ask

Zyntra can retrieve evidence from manually supplied text and Markdown SOPs,
runbooks, manuals and policies. Retrieval is lexical BM25 over permitted source
chunks. Answers are source excerpts with document ID, revision, source reference,
line range and SHA-256 of the complete source text.

This is a local knowledge foundation, not a PDF/OCR parser or semantic embedding
service. No URL is fetched, no external index is required, and no document can
create a proposal, approve an action, modify plan scores or execute a command.

## Use the console

Open **Intelligence → Ask**. Administrators can add documents through the
**Document knowledge** card, edit their text or reader restrictions, or delete
them. Existing sources can be opened for inspection. Each edit creates a new
revision; stale edits and deletes fail with a version conflict rather than
silently replacing another administrator's changes.

Choose **Document knowledge** in **Answer from** and ask using terminology from
the source, for example “When should inventory be reordered?”. Alternatively,
questions mentioning documents, runbooks, procedures, “according to” or “SOP ”
automatically use document retrieval. Other operational questions retain the
existing KPI/ontology answer path. No match produces an explicit evidence-missing
answer and never silently substitutes a model-generated procedure.

The answer displays numbered excerpts and expandable document citations. Hashes
identify the exact full text revision, not an excerpt's hash or a signed assertion
that a supplied procedure is correct. Source references are administrator-supplied
metadata, displayed as text, not verified origin URLs. Documents are not part of
the signed operational decision audit chain; their revision metadata records the
editing principal and time.

## Visibility and reader roles

Every document belongs to one pack's corpus and has explicit visibility:

| Visibility | Readers |
|---|---|
| `provider` (default) | Deployment-wide readers; tenant identities cannot see it |
| `shared` | Permitted provider and tenant readers |
| `tenant` | Readers of the named tenant; deployment-wide administrators can also inspect it |

An optional `roles` array narrows access to exact role matches (`viewer`,
`proposer`, `approver`, `executor`, `admin`). Empty means every authenticated
reader permitted by visibility. Roles do not inherit one another; list every
role that should read the document. Deployment-wide administrators bypass the
role filter for administration. Machine-only credentials cannot use reader or
admin document routes.

Only deployment-wide administrators can create, update or delete documents.
Tenant accounts can list, read, search and ask over permitted documents but
cannot write documents or read provider-wide KPI analytics. Search filters
visibility and roles before chunking, term-frequency calculation or ranking:
inaccessible documents do not influence snippets, ranking or returned counts.
Requests for an inaccessible document return the same 404 as a missing one.

Fetching an older revision requires permission under **both its original and
current restrictions**. Tightening access therefore blocks old citations too;
loosening current access does not reveal a previously restricted revision.

## Optional model: evidence selection only

With the existing OpenAI-compatible provider configured, only the authorized
retrieval hits and the question are sent to that provider. The model may select
up to three exact quote substrings. The server validates index bounds, quote
length, uniqueness and exact membership in the retrieved evidence. Unsupported
quotes, malformed output, empty selections or provider errors fall back to
local excerpts. The model never writes the answer's factual prose or adds
numbers, steps or document IDs. Selected quotes retain their source chunk's line
range, which may be wider than the quote itself.

Retrieved text is explicitly labelled untrusted in the model prompt. The model
has no tools on this path. Source instructions remain quoted evidence and cannot
cross into action drafting, approval or execution. The existing provider's
endpoint, key and timeout configuration still applies; this feature adds a
10-second context deadline for evidence selection.

## Persistence, limits and deletion

Each served pack stores `knowledge.sqlite` beside its other state. SQLite WAL
transactions use synchronous FULL. The main file is created with mode 0600.
The store keeps up to 500 live documents and 10 MiB of serialized live document
bodies per pack, with a 128 KiB text limit per document. Search queries are
1–2,000 bytes and return at most ten hits; Ask requests retrieve five and show
at most three. Chunks preserve source characters and line numbers, up to 2,000
Unicode characters or about 30 lines per chunk. There are no embeddings,
relevance thresholds, stemming or translation; keyword similarity is not proof
that a passage answers the question. Inspect the cited evidence.

The most recent ten revisions of each live document are retained. Deleting a
document removes its current body and retained revision rows from the accessible
store. Old citations then return 404. A small ID tombstone prevents reusing its
identity for different content. Deletion is not secure erasure of SQLite pages,
WAL files or backups. Revision storage can be larger than the live-body cap.
Back up using SQLite's backup API or stop/checkpoint before copying database
files. Retrieval scans the bounded visible corpus, so this first implementation
is intended for a small operational knowledge collection, not millions of files.

## API

All routes use the existing sign-in and pack selection (`X-Zyntra-Pack` or
`?pack=`). IDs contain 1–80 letters/digits/dots/underscores/hyphens, starting with
a letter or digit; tenant IDs follow the existing tenant identity convention.

| Method | Path | Access |
|---|---|---|
| GET | `/api/v1/knowledge/documents` | Readers; visible metadata only |
| GET | `/api/v1/knowledge/documents/{id}?version=N` | Readers; current or permitted retained revision |
| GET | `/api/v1/knowledge/search?q=inventory&limit=5` | Readers; authorized citations |
| PUT | `/api/v1/knowledge/documents/{id}` | Administrator; versioned create/update |
| DELETE | `/api/v1/knowledge/documents/{id}` | Administrator; versioned deletion |
| POST | `/api/v1/ai/ask` with `scope: documents` | Readers; cited extractive answer |

Creation example (zero means the document must not already exist):

```json
{
  "expected_version": 0,
  "document": {
    "title": "Inventory reorder SOP",
    "source": "Operations handbook, approved revision 7",
    "visibility": "shared",
    "roles": ["viewer", "proposer", "approver"],
    "text": "Reorder when stock falls below ten units.\nAn approver must authorize the purchase order."
  }
}
```

Update sends the current `expected_version` and a complete replacement document;
read restrictions are replaced along with text. Delete sends
`{"expected_version": 1}`. Ask accepts
`{"question":"When should inventory be reordered?","scope":"documents"}`
and returns the existing Answer shape with an additive `document_citations`
array and `document:<id>@<version>#<sha256>:L<start>-L<end>` grounding references.
The full source revision can be fetched with the document route to verify it.

## Validation

Tests cover tenant/provider/role isolation, hidden-corpus ranking independence,
revision permission changes, current and old citations, version conflicts,
concurrent writes, deletion/tombstones, restart, line and Unicode chunking,
private files, hash mismatch rejection, bounds, pack isolation, no-match behavior,
model fabricated-quote rejection, offline fallback and console create/read/Ask.
Run `go test -race ./...`, `go vet ./...`, `make build`, `npm test --prefix web`,
`make eval` and `make test-e2e`.
