---
title: Deployment
description: Release, Homebrew, and documentation deployment process.
---

# Deployment

## Branches

- `main` is the Go implementation and the active development branch.
- `python-version` preserves the previous Python implementation.

## CLI Release

Releases are tag-driven. Push a version tag to run GoReleaser:

```bash
git tag -a v0.2.0-go.6 -m "miudb v0.2.0-go.6"
git push origin v0.2.0-go.6
```

The release workflow builds Darwin and Linux archives, publishes a GitHub
release, and updates the Homebrew tap when the `HOMEBREW_TAP_GITHUB_TOKEN`
repository secret is configured.

## Homebrew Install

```bash
brew install vanducng/tap/miudb
brew upgrade vanducng/tap/miudb
miudb version --output json
```

## Documentation Deploy

Documentation deploys from `main` through GitHub Pages:

```bash
zensical build --clean
```

The docs workflow writes `miu-db.vanducng.dev` into `site/CNAME`, uploads the
generated `site/` directory, and deploys it through `actions/deploy-pages`.
