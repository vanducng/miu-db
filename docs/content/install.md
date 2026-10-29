---
title: Install
description: Install miudb on macOS, Linux, or Windows, pin a version, and upgrade in place.
---

The binary is `miudb`, a single static Go build with no runtime dependencies.

## Install

| Platform | Command |
|---|---|
| macOS, Linux | `curl -fsSL https://db.miu.sh/install.sh \| sh` |
| Windows (PowerShell) | `irm https://db.miu.sh/install.ps1 \| iex` |
| Homebrew | `brew install vanducng/tap/miudb` |
| Go 1.25+ | `go install github.com/vanducng/miu-db/cmd/miudb@latest` |

Both scripts detect OS and architecture, verify the release checksum, and honor:

| Env | Default | Effect |
|---|---|---|
| `MIUDB_VERSION` | `latest` | Install a specific tag, for example `v0.10.6` |
| `MIUDB_INSTALL_DIR` | `/usr/local/bin`, Windows `%LOCALAPPDATA%\miudb\bin` | Target directory. On Unix, uses `sudo` when it is not writable, else falls back to `~/.local/bin` |

With `go install`, put `$(go env GOPATH)/bin` on `PATH`.

Windows zips (`miudb_windows_x86_64.zip`, `miudb_windows_arm64.zip`) are also on
[GitHub Releases](https://github.com/vanducng/miu-db/releases):

```powershell
$version = "v0.10.6"
$asset = "miudb_windows_x86_64.zip"
Invoke-WebRequest "https://github.com/vanducng/miu-db/releases/download/$version/$asset" -OutFile $asset
Expand-Archive $asset -DestinationPath ".\miudb" -Force
.\miudb\miudb.exe version
```

## Verify

```sh
miudb version
miudb commands
```

## Upgrade

```sh
miudb upgrade                    # latest release, checksum verified, replaces the binary
miudb upgrade --check            # only report whether a newer version exists
miudb upgrade --version v0.10.6  # a specific tag
```

Homebrew installs refuse self-upgrade; use `brew upgrade miudb`.

## Agent skill

```sh
curl -fsSL https://db.miu.sh/skill.md --create-dirs \
  -o ~/.claude/skills/miudb/SKILL.md
```

Use `~/.agents/skills/miudb/SKILL.md` for Codex, Cursor, and other agents. The
same file lives in the repo at `.agents/skills/miudb/SKILL.md`.

## From a checkout

```sh
go build -buildvcs=false -o ./.miu-db/miudb ./cmd/miudb
./.miu-db/miudb version
```

`-buildvcs=false` avoids VCS stamping failures in git worktrees.
