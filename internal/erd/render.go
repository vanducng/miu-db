package erd

import (
	"encoding/json"
	"strings"
)

// Pinned CDN URLs for --cdn (smaller file, needs network); must match the embedded versions.
const (
	cytoscapeCDN = "https://cdn.jsdelivr.net/npm/cytoscape@3.30.2/dist/cytoscape.min.js"
	nhlCDN       = "https://cdn.jsdelivr.net/npm/cytoscape-node-html-label@1.2.2/dist/cytoscape-node-html-label.min.js"
)

type RenderOpts struct {
	CDN          bool   // link libs from CDN instead of inlining
	DefaultTitle string // used when meta.title is empty
}

// RenderHTML produces the self-contained interactive ERD (mirrors er_html.py build_html).
// The payload is HTML-escaped by encoding/json, so it is safe inside the <script> data block.
func RenderHTML(p Payload, opts RenderOpts) (string, error) {
	payload, err := json.Marshal(p)
	if err != nil {
		return "", err
	}

	title := p.Meta.Title
	if title == "" {
		title = opts.DefaultTitle
	}
	if title == "" {
		title = "Database — Entity Relationship Diagram"
	}

	libs := "<script>" + mustAsset("assets/lib/cytoscape.min.js") + "</script>\n" +
		"<script>" + mustAsset("assets/lib/cytoscape-node-html-label.min.js") + "</script>"
	if opts.CDN {
		libs = `<script src="` + cytoscapeCDN + `"></script>` + "\n" + `<script src="` + nhlCDN + `"></script>`
	}

	// Single pass so injected JS/JSON is never re-scanned for placeholders.
	return strings.NewReplacer(
		"__TITLE__", htmlEscape(title),
		"__LIBS__", libs,
		"__PAYLOAD__", string(payload),
	).Replace(mustAsset("assets/erd-template.html")), nil
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	return strings.ReplaceAll(s, ">", "&gt;")
}
