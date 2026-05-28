# miudb Go Preview Install

The Go preview binary is named `miudb`. The Python TUI remains `miu-db`.

## Release Install

```bash
go install github.com/vanducng/miu-db/cmd/miudb@v0.2.0-go.4
```

Make sure your Go bin directory is on `PATH`:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

Verify:

```bash
miudb version --output json
miudb commands --output json
```

## Homebrew Install

```bash
brew install vanducng/tap/miudb
```

Upgrade later:

```bash
brew update
brew upgrade miudb
```

## Branch Install

Before the preview tag is available, or when testing the latest branch:

```bash
go install github.com/vanducng/miu-db/cmd/miudb@golang
```

## Local Checkout

```bash
go test ./...
go build -buildvcs=false -o ./.miu-db/miudb ./cmd/miudb
./.miu-db/miudb version --output json
```

`-buildvcs=false` avoids VCS stamping failures in git worktree layouts.

## Existing miu-db Config

The Go preview reads the current Python miu-db config and explicit credential
export:

```bash
miudb connections list \
  --config-dir /Users/vanducng/.config/miu/db \
  --credentials-export /Users/vanducng/.config/miu/db/credentials-export.json \
  --output json
```

Run a bounded query:

```bash
miudb query run \
  --connection agent-deck \
  --sql "select 1 as one" \
  --config-dir /Users/vanducng/.config/miu/db \
  --credentials-export /Users/vanducng/.config/miu/db/credentials-export.json \
  --output json
```

Run a saved-connection health matrix:

```bash
miudb connections smoke \
  --timeout 20s \
  --concurrency 1 \
  --config-dir /Users/vanducng/.config/miu/db \
  --credentials-export /Users/vanducng/.config/miu/db/credentials-export.json \
  --output json
```
