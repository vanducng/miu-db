# Research: Agent-Friendly CLI Output and Command Contracts

_Date: 2026-05-28; Mode: deep; Queries: 10_

## TL;DR
- **Recommendation:** Use a stable JSON envelope for one-shot CLI commands, JSON Lines/NDJSON for streams, and JSON-RPC 2.0 over stdio for long-lived protocol mode. Add a `schema` or `describe` command that emits command metadata, JSON Schemas, stability, side effects, and examples for agents.
- **Runner-up:** Copy the GitHub CLI/kubectl pattern: human output by default, `--output json`, `--fields`, `--jq`, and template output. This is good for humans and scripts, but weaker for agents unless paired with schemas and error contracts.
- **Avoid:** Plain tables, free-form prose, or CSV-style machine output as the primary agent interface. They are compact but brittle, hard to version, and poor for nested database results.

## The Question
What output formats and CLI description rules should the Go `miu-db` use so coding agents can inspect databases, run queries, understand errors, manage token volume, and compose reliable shell workflows?

## Evaluation Criteria
- **Parse reliability:** Agents and scripts should not regex human prose.
- **Token efficiency:** Output should support field selection, row limits, summaries, and artifact links.
- **Streaming:** Long queries and schema scans need incremental events without corrupting stdout.
- **Discoverability:** Agents need machine-readable command descriptions, output schemas, examples, and side-effect markers.
- **Safety:** Secrets must be redacted, mutating commands must be explicit, and SQL execution must report risk/context.
- **Debuggability:** Errors must carry stable codes, human messages, hints, and enough structured detail.
- **Ecosystem:** Prefer formats supported by shell tools, Go libraries, Neovim Lua, and common agent runtimes.
- **Stability:** Agents need versioned output contracts and deprecation rules.

## Options Considered
- **Option A: JSON envelope + JSONL streams + JSON-RPC serve mode** - one schema family for CLI, streaming, and Neovim/agent protocol.
- **Option B: gh/kubectl style output controls** - `--output json|yaml|table`, `--fields`, `--jq`, `--template`.
- **Option C: MCP-like structured tool result contract** - model every CLI command as a tool with input/output schemas and structured results.
- **Option D: Human/table/Markdown-first output** - optimize readability and token compactness, with JSON as secondary.

## Comparison Matrix

| Criterion | A: JSON envelope + JSONL + JSON-RPC | B: gh/kubectl style controls | C: MCP-like tool contract | D: Human/table/Markdown-first |
|---|---|---|---|---|
| Parse reliability | Strong. Stable envelope and schemas. | Strong if JSON is explicit, weaker in defaults. | Strong for agents. | Weak unless very constrained. |
| Token control | Strong with `--fields`, pages, summaries, artifact links. | Good with `--json fields` and `--jq`. | Strong if schemas include output shaping. | Medium for Markdown, poor for automation. |
| Streaming | Strong with JSON Lines events. | Mixed; depends on command. | Strong if transported over JSON-RPC/MCP. | Weak. |
| Discoverability | Strong if `schema`/`describe` exists. | Medium; help text plus field discovery. | Strong. | Weak. |
| Human UX | Good if human default remains. | Strong. | Medium; can be verbose. | Strong. |
| Agent UX | Strong. | Medium-strong. | Strong, but heavier. | Weak-medium. |
| Implementation complexity | Medium. | Medium. | Medium-high. | Low initially, high later. |
| Long-term stability | Strong if versioned. | Medium; templates can lock internals. | Strong. | Weak. |

## Per-Option Deep Dive

### Option A: JSON Envelope + JSONL Streams + JSON-RPC Serve Mode
- **Strengths:** One mental model covers one-shot CLI calls, long-running query events, and Neovim protocol. JSON-RPC 2.0 already defines request IDs, methods, responses, and error objects. JSON Lines gives a simple stream rule: each line is one valid JSON value.
- **Weaknesses:** Needs careful schema discipline from day one. Pretty JSON is easy for humans but expensive for tokens unless compact/field-selected modes exist.
- **Dealbreakers:** Bad if the tool only targets humans. This project explicitly targets agents, so that is not a blocker.
- **Real-world users/patterns:** MCP uses JSON-RPC and structured tool results; many CLIs expose JSON for automation; JSON Lines is common for logs/events.
- **Security notes:** All envelopes need redaction at serialization time. Do not let full connection URLs or passwords reach `stderr`, debug logs, or protocol events.

Recommended envelope:

```json
{
  "ok": true,
  "api_version": "miu-db.cli/v1",
  "kind": "query.result",
  "command": "query run",
  "request_id": "01J...",
  "summary": {
    "connection": "prod-readonly",
    "database": "analytics",
    "rows_returned": 100,
    "truncated": true,
    "elapsed_ms": 842,
    "token_estimate": 6200
  },
  "data": {
    "columns": [
      {"name": "id", "type": "INTEGER", "nullable": false}
    ],
    "rows": [
      [1]
    ]
  },
  "page": {
    "limit": 100,
    "next_cursor": "call_123:100"
  },
  "artifacts": [],
  "warnings": []
}
```

Recommended error envelope:

```json
{
  "ok": false,
  "api_version": "miu-db.cli/v1",
  "kind": "error",
  "command": "query run",
  "request_id": "01J...",
  "error": {
    "code": "connection.auth_failed",
    "message": "Authentication failed for connection prod-readonly.",
    "hint": "Run `miu-db connections test prod-readonly --output json`.",
    "retryable": false,
    "safe_to_retry": true,
    "details": {
      "connection": "prod-readonly",
      "driver": "postgres"
    }
  }
}
```

### Option B: gh/kubectl Style Output Controls
- **Strengths:** Proven CLI ergonomics. GitHub CLI supports JSON fields plus `--jq` and templates. kubectl exposes JSON, YAML, JSONPath, custom columns, and wide output. AWS CLI supports JSON, YAML, YAML streaming, text, table, and output suppression.
- **Weaknesses:** These tools are human-first and script-friendly, but not automatically agent-friendly. Agents still need schemas, side-effect markers, error kinds, and token/row controls.
- **Dealbreakers:** If this is the only pattern, command meaning remains trapped in help text. Agents need a structured command catalog.
- **Real-world users/patterns:** `gh`, `kubectl`, Docker, AWS CLI.
- **Security notes:** Template output can accidentally expose hidden fields if the raw object contains secrets. Secret fields must be omitted or redacted before formatting.

Use this pattern as the surface:

```text
--output table|json|jsonl|yaml|markdown|off
--fields name,type,status
--jq '.data.rows[] | {id: .[0]}'
--limit 100
--cursor <cursor>
--no-color
--quiet
--verbose
```

But keep Option A as the contract behind these flags.

### Option C: MCP-Like Structured Tool Contract
- **Strengths:** MCP formalizes structured tool results, output schemas, and error separation in a way directly aligned with agents. It also recognizes resource links, which fits large query results written to files.
- **Weaknesses:** MCP itself may be too heavy for the first CLI. If adopted too early, you may design for a host protocol instead of a good standalone CLI.
- **Dealbreakers:** Bad as the only interface if users want simple shell commands. Good as a compatibility target.
- **Real-world users/patterns:** MCP servers, agent tool runtimes.
- **Security notes:** MCP guidance emphasizes input validation, access control, rate limits, output sanitization, user confirmation for sensitive operations, and timeouts. Those map directly to DB tools.

Borrow these ideas:

- Every command has input and output JSON Schema.
- Result has structured content plus optional human text.
- Large data can be returned as a resource/file link instead of inlining everything.
- Tool execution errors are distinct from protocol errors.

### Option D: Human/Table/Markdown-First Output
- **Strengths:** Great for terminals. Markdown can be more token-efficient than pretty JSON for small summaries.
- **Weaknesses:** Poor primary contract for agents. Tables are ambiguous with nested values, long text, nulls, binary data, multi-result sets, and SQL errors.
- **Dealbreakers:** Database query results are nested, typed, paginated, and often large. Human-first output alone is not stable enough.
- **Real-world users/patterns:** Many older CLIs, psql-style output, Markdown reports.
- **Security notes:** Human output is more likely to hide important metadata or leak unstructured details.

Use Markdown only for explicit summary/report commands:

```text
miu-db query summarize --format markdown
```

Do not use it as the default agent format.

## Recommended Contract

### Output Modes

Every command should support:

```text
--output table|json|jsonl|yaml|markdown|off
```

Rules:

- If stdout is a TTY: default to `table` or compact human text.
- If stdout is not a TTY: default to `json`.
- `serve` mode: JSON-RPC 2.0 messages over stdio.
- Streaming commands: `jsonl` where each line is a self-contained event.
- Diagnostics, progress, prompts, and logs always go to stderr.
- ANSI color only for TTY, disabled by `NO_COLOR`, `TERM=dumb`, or `--no-color`.

### Standard Envelope

All JSON command output should include:

- `ok`: boolean.
- `api_version`: stable output contract version.
- `kind`: machine-readable result type, such as `query.result`, `schema.tree`, `connection.list`.
- `command`: command path.
- `request_id`: generated ID for logs/protocol correlation.
- `summary`: small token-efficient summary.
- `data`: primary structured payload.
- `page`: `limit`, `next_cursor`, `has_more`.
- `stats`: elapsed time, row counts, byte counts, token estimate when useful.
- `artifacts`: file/resource links for large outputs.
- `warnings`: structured warning list.
- `error`: only when `ok=false`.

### SQL Result Payload Rules

For query results:

- Always include column metadata separately from rows.
- Rows should be arrays for compactness, with `columns` defining names and types.
- Add `--rows object|array` for agent readability vs token compactness.
- Default max rows should be bounded.
- Include `truncated`, `next_cursor`, and `export_path` when not all data is returned.
- Include multiple result sets as `results: [{columns, rows, stats}]`.
- Include `token_estimate` and `bytes_estimate` for agent budget management.
- For very large output, write artifacts and return metadata plus path, not full rows.

### JSONL Event Rules

Use JSONL for query/watch/worker streams:

```jsonl
{"event":"call.started","call_id":"call_123","query_hash":"sha256:..."}
{"event":"call.columns","call_id":"call_123","columns":[{"name":"id","type":"INTEGER"}]}
{"event":"call.rows","call_id":"call_123","offset":0,"rows":[[1],[2]]}
{"event":"call.done","call_id":"call_123","rows_returned":2,"elapsed_ms":40}
```

Rules:

- One JSON object per line.
- Include `event`, `call_id`, and monotonic sequence number.
- Never interleave human progress into stdout.
- Stderr can contain human progress if useful.
- The final event must be `done`, `cancelled`, or `error`.

### Command Description Rules For Agents

Add a machine-readable command catalog:

```text
miu-db schema --output json
miu-db describe query run --output json
miu-db commands --output json
```

Each command description should include:

- `name`: command path, such as `query run`.
- `summary`: one sentence.
- `description`: longer guidance.
- `stability`: `stable|experimental|internal`.
- `mutates`: boolean.
- `idempotent`: boolean.
- `requires_connection`: boolean.
- `requires_confirmation`: boolean.
- `supports_output`: list of output modes.
- `default_limit`: row/object limit.
- `max_limit`: hard cap.
- `input_schema`: JSON Schema.
- `output_schema`: JSON Schema.
- `error_codes`: stable error kinds and retryability.
- `examples`: realistic examples with expected output shape.
- `side_effects`: DB writes, local config writes, keyring writes, tunnel opens.
- `security`: secret handling, SQL execution risk, permission notes.
- `token_notes`: how to keep output small.

Example:

```json
{
  "name": "query run",
  "summary": "Run SQL against a saved connection.",
  "stability": "stable",
  "mutates": false,
  "side_effects": ["opens_connection", "may_create_tunnel", "writes_query_history"],
  "input_schema": {
    "type": "object",
    "required": ["connection", "sql"],
    "properties": {
      "connection": {
        "type": "string",
        "description": "Saved connection name."
      },
      "sql": {
        "type": "string",
        "description": "SQL statement to execute."
      },
      "max_rows": {
        "type": "integer",
        "default": 100,
        "description": "Maximum rows returned inline."
      }
    }
  },
  "output_schema_ref": "miu-db.schema/query-result-v1.json",
  "examples": [
    {
      "command": "miu-db query run --connection local --sql 'select 1' --output json",
      "purpose": "Run a small query and parse the result envelope."
    }
  ]
}
```

## Failure Modes

| Option | Mode | Symptom | Mitigation | Recovery cost |
|---|---|---|---|---|
| A | Envelope too verbose | Agents waste tokens on metadata | Add `--compact`, `--fields`, summaries, and artifact links | Low |
| A | Schema churn | Agents break after field rename | Version schemas and deprecate fields before removal | Medium |
| A | JSONL stream incomplete | Agent waits forever | Always emit terminal event; support timeouts and call status | Medium |
| B | `--jq`/templates expose raw internals | Secrets or unstable fields leak | Redact before formatting; define stable public objects | High |
| B | Field discovery is weak | Agents guess field names | `--fields ?` and `describe` command | Low |
| C | MCP assumptions leak into CLI | Shell UX becomes awkward | Keep MCP as adapter layer, not core CLI design | Medium |
| D | Human output parsed by agents | Fragile regex and hallucinated parsing | Make JSON the non-TTY default | Low |

## Migration Paths

- **From plain CLI to agent CLI:** add `--output json`, stable envelopes, error codes, and stdout/stderr separation first.
- **From JSON CLI to streaming protocol:** add `--output jsonl` events for long operations, then reuse event objects in JSON-RPC.
- **From CLI to MCP:** generate MCP tool schemas from the same command catalog and JSON Schemas.
- **From verbose JSON to token-efficient JSON:** add `--fields`, row arrays, summaries, `--limit`, cursor paging, and artifact links.

## Operational Notes

- `stdout` is data. `stderr` is diagnostics, progress, warnings, and human hints.
- Exit code `0` means the requested command completed. Nonzero means no valid success envelope is guaranteed unless `--output json` is explicitly documented to emit errors to stdout.
- In `--output json`, prefer emitting structured error envelope to stdout and concise diagnostic to stderr. This lets agents parse failure details without losing conventional exit status.
- Mutating commands should require `--yes` or `--confirm` when not attached to a TTY.
- SQL execution commands should expose `--readonly`, `--max-rows`, `--timeout`, and `--dry-run` where technically meaningful.
- Query commands should default to bounded row counts and make unbounded exports explicit.

## Performance Under Realistic Load

- JSON object arrays are readable but token-heavy. For rows, array rows plus column metadata are much smaller.
- Pretty JSON should be TTY-only or explicit. Piped JSON should be compact by default.
- JSONL lets agents process early rows/events without waiting for the full query.
- Large query output should become a file artifact with metadata returned inline.
- Field selection should happen before serialization so unnecessary fields do not consume time or tokens.

## Decision Reversibility

This design is reversible if versioned early. The risky part is not choosing JSON; it is shipping unversioned fields that agents learn to depend on. Add `api_version`, schema IDs, and deprecation rules before the first agent-facing release.

## Recommendation
Implement the agent CLI contract before the implementation plan is finalized:

1. Define the JSON envelope and error envelope.
2. Define JSONL events for `query run --stream`, `serve`, worker progress, and schema scans.
3. Define the command catalog and `describe`/`schema` command.
4. Define query-result compactness rules: columns plus array rows, bounded inline rows, cursors, token estimates, artifacts.
5. Define stdout/stderr, exit code, color, paging, prompt, and confirmation rules.
6. Add tests that run representative commands and validate JSON Schema.

This should be treated as part of the Go core architecture, not CLI polish. It directly affects protocol design, Neovim integration, agent usage, and result pagination.

## References
- JSON Lines: https://jsonlines.org/
- JSON-RPC 2.0 Specification: https://www.jsonrpc.org/specification
- GitHub CLI formatting: https://cli.github.com/manual/gh_help_formatting
- kubectl get output formats: https://kubernetes.io/docs/reference/kubectl/generated/kubectl_get/
- kubectl JSONPath support: https://v1-32.docs.kubernetes.io/docs/reference/kubectl/jsonpath/
- Docker CLI formatting: https://docs.docker.com/engine/cli/formatting/
- AWS CLI output formats: https://docs.aws.amazon.com/cli/latest/userguide/cli-usage-output-format.html
- MCP tool results and output schemas: https://modelcontextprotocol.io/specification/2025-06-18/server/tools
- JSON Schema annotations: https://json-schema.org/understanding-json-schema/reference/annotations
- Command Line Interface Guidelines: https://clig.dev/
- CLI Spec: https://clispec.dev/
- OpenCLI proposal: https://opencli.org/
- usage CLI specification: https://usage.jdx.dev/spec/

## Open Questions
- Should `miu-db --output json` emit structured errors to stdout on nonzero exit, or reserve stdout only for success and write machine-readable errors to stderr?
- Should the command catalog follow clispec/OpenCLI/usage directly, or use a small `miu-db` schema that can later be transformed into those formats?
- Should SQL rows default to compact arrays or object rows for first release? Arrays save tokens; objects are easier for agents to reason about.
- What maximum inline token/row budget should be the default for query results?
