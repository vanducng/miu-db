# Bundled third-party assets

These minified bundles are embedded (`go:embed`) into the binary for fully-offline
ERD rendering. Refresh with `go generate ./internal/erd` (see `assets.go`).

| File | Library | Version | License |
|---|---|---|---|
| `lib/cytoscape.min.js` | [Cytoscape.js](https://github.com/cytoscape/cytoscape.js) | 3.30.2 | MIT |
| `lib/cytoscape-node-html-label.min.js` | [cytoscape-node-html-label](https://github.com/kaluginserg/cytoscape-node-html-label) | 1.2.2 | MIT |

`erd-template.html` is derived from the `diagram` skill's `er_html.py` `_TEMPLATE`
(MIT, © vanducng).

Both libraries are MIT-licensed; their copyright notices are retained inside the
minified files. No copyleft obligations.
