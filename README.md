# miudb

Go preview of miu-db as a headless database CLI for agents and Neovim.

Install the preview release:

```bash
go install github.com/vanducng/miu-db/cmd/miudb@v0.2.0-go.3
```

Or with Homebrew:

```bash
brew install vanducng/tap/miudb
```

Run locally from source:

```bash
go test ./...
go build -buildvcs=false -o ./.miu-db/miudb ./cmd/miudb
./.miu-db/miudb commands --output json
```

See `docs/go-install.md` and `docs/golang-architecture.md`.
