package erd

import (
	"strings"
	"testing"
)

func TestRenderHTMLSelfContained(t *testing.T) {
	p := Payload{
		Schema: []Table{{Name: "users", Columns: []Column{{Name: "id", UDT: "bigint", Ord: 1}}}},
		Meta:   Meta{Title: "T"},
	}
	out, err := RenderHTML(p, RenderOpts{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<title>T</title>", `id="data"`, `"users"`, "cytoscape"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(out, `src="http`) {
		t.Error("inlined output must have no external script src")
	}
	for _, ph := range []string{"__TITLE__", "__LIBS__", "__PAYLOAD__"} {
		if strings.Contains(out, ph) {
			t.Errorf("unreplaced placeholder %s", ph)
		}
	}
}

func TestRenderHTMLCDN(t *testing.T) {
	out, err := RenderHTML(Payload{Schema: []Table{}, Meta: Meta{}}, RenderOpts{CDN: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `src="`+cytoscapeCDN+`"`) {
		t.Error("cdn mode must link cytoscape from jsdelivr")
	}
	if strings.Contains(out, "<script>function") || len(out) > 60_000 {
		t.Error("cdn mode must not inline the bundle")
	}
}

func TestRenderHTMLTitleFallbackAndEscape(t *testing.T) {
	out, _ := RenderHTML(Payload{Schema: []Table{}}, RenderOpts{DefaultTitle: "fallback-db"})
	if !strings.Contains(out, "<title>fallback-db</title>") {
		t.Error("should use DefaultTitle when meta.title empty")
	}
	esc, _ := RenderHTML(Payload{Meta: Meta{Title: "a<b>&c"}}, RenderOpts{CDN: true})
	if !strings.Contains(esc, "a&lt;b&gt;&amp;c") {
		t.Error("title must be html-escaped")
	}
}
