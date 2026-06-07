package erd

import "embed"

// Renderer assets, bundled for fully-offline output. Refresh with:
//
//	go generate ./internal/erd
//
//go:generate sh -c "curl -fsSL https://cdn.jsdelivr.net/npm/cytoscape@3.30.2/dist/cytoscape.min.js -o assets/lib/cytoscape.min.js && curl -fsSL https://cdn.jsdelivr.net/npm/cytoscape-node-html-label@1.2.2/dist/cytoscape-node-html-label.min.js -o assets/lib/cytoscape-node-html-label.min.js"
//go:embed assets/erd-template.html assets/lib/cytoscape.min.js assets/lib/cytoscape-node-html-label.min.js
var assetsFS embed.FS

func mustAsset(p string) string {
	b, err := assetsFS.ReadFile(p)
	if err != nil {
		panic("erd: missing embedded asset " + p + ": " + err.Error())
	}
	return string(b)
}
