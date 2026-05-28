# miudb CLI Contract

The Go track exposes `miudb` as the primary binary. `miu-db` remains the Python
entry point.

## Output Modes

Every agent-facing command supports `--output json`. Commands that stream can
also support `--output jsonl`. Human table output can be added after the JSON
contract is stable.

Rules:

- stdout is data.
- stderr is diagnostics, progress, and human hints.
- JSON output uses compact objects by default.
- Secrets are never serialized.
- Query rows are arrays by default, with column metadata carrying names/types.
- Bounded query output is the default; commands return `page.next_cursor` when
  more data can be fetched.

## JSON Envelope

Successful commands return:

```json
{
  "ok": true,
  "api_version": "miudb.cli/v1",
  "kind": "query.result",
  "command": "query run",
  "request_id": "req_...",
  "summary": {},
  "data": {},
  "page": {},
  "stats": {},
  "artifacts": [],
  "warnings": []
}
```

Errors return the same envelope shape with `ok: false` and an `error` object.

## Protocol Modes

`miudb serve --protocol jsonrpc` uses JSON-RPC 2.0 responses plus server-to-
client notifications for call events.

`miudb serve --protocol ndjson` uses one line-delimited envelope per request,
response, and event. Raw human progress is never written to stdout in either
mode.

## Command Catalog

`miudb commands --output json` returns the command catalog. `miudb describe
<command> --output json` returns one command descriptor with inputs, outputs,
side effects, stability, and examples.
