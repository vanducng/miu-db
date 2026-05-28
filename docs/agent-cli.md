# Agent CLI Usage

Use JSON output for all agent calls:

```bash
miudb commands --output json
miudb describe query run --output json
miudb connections list --output json
miudb connections smoke --timeout 12s --concurrency 4 --output json
miudb query run --connection agent-deck --sql "select 1" --output json
```

Rules:

- stdout is machine-readable JSON.
- stderr is diagnostics only.
- `ok: false` carries structured error information.
- Results are bounded by `--limit`.
- Use `query fetch-page --cursor <cursor>` to continue a truncated one-shot
  query result.
- Use `connections smoke` when an agent needs a full saved-connection health
  matrix. It returns one JSON envelope containing every per-connection result,
  per-type pass/fail counts, redacted error messages, and retry hints.
