<p align="center"><img src="docs/public/brand/banner.png" alt="miu-db" width="840"></p>

<p align="center">
  <a href="https://github.com/vanducng/miu-db/releases"><img src="https://img.shields.io/github/v/release/vanducng/miu-db?label=release&color=7c3aed" alt="Release"></a>
  <a href="https://go.dev"><img src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white" alt="Go 1.25"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="MIT"></a>
</p>

# miu-db

A headless database CLI for humans and agents. Saved connections, keyring
secrets, SSH tunnels, bounded queries, schema, ERDs, and MCP for SQLite,
Postgres, MySQL, Snowflake, and BigQuery, with stable JSON on stdout.

**Docs:** [db.miu.sh](https://db.miu.sh) · agents: append `.md` to any docs URL, or read [`llms.txt`](https://db.miu.sh/llms.txt)

## Get started

![miudb demo: add a connection, smoke test, query, schema, ERD, and the activity log](docs/public/demo/miudb.gif)

```sh
curl -fsSL https://db.miu.sh/install.sh | sh
curl -fsSL https://db.miu.sh/skill.md --create-dirs \
  -o ~/.claude/skills/miudb/SKILL.md
```

Then ask your agent:

```text
/miudb set up miudb: list my saved connections, add a SQLite connection for
./app.db if none exists, smoke-test everything, then show me the schema.
```

Or by hand:

```sh
miudb connections add --name app --db-type sqlite --path ./app.db
miudb connections smoke                              # test every saved connection
miudb query run --connection app --sql 'select * from users' -o pretty
miudb schema tree --connection app
miudb mcp serve --transport stdio                    # read-only tools for agent hosts
```

Other installs (Homebrew, Windows, `go install`): [Install](https://db.miu.sh/install/). More commands: [Getting started](https://db.miu.sh/onboarding/).

## Develop

```sh
go build -buildvcs=false -o ./.miu-db/miudb ./cmd/miudb
go test ./...
```

**Agent skill:** [`.agents/skills/miudb/SKILL.md`](.agents/skills/miudb/SKILL.md) (agent-agnostic); `.claude/skills/miudb` is a symlink to it.

## License

MIT. See [LICENSE](LICENSE).
