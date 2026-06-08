package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConnectionMarshalsGroupNotFolderPath(t *testing.T) {
	out, err := json.Marshal(Connection{Name: "a", DBType: "sqlite", Group: "team/project"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `"group":"team/project"`) {
		t.Fatalf("marshal missing group key: %s", s)
	}
	if strings.Contains(s, "folder_path") {
		t.Fatalf("marshal must not emit legacy folder_path: %s", s)
	}
}

func TestRedactedConnectionExposesGroup(t *testing.T) {
	red := RedactedConnection(Connection{Name: "a", DBType: "sqlite", Group: "team/project"})
	if red["group"] != "team/project" {
		t.Fatalf("redacted group = %v, want team/project", red["group"])
	}
	if _, ok := red["folder_path"]; ok {
		t.Fatal("redacted output must not include folder_path")
	}
}

func TestConnMatches(t *testing.T) {
	webA := Connection{Name: "web", Group: "team-alpha"}
	webB := Connection{Name: "web", Group: "team-beta"}
	bare := Connection{Name: "api"}
	cases := []struct {
		conn Connection
		spec string
		want bool
	}{
		{webA, "web", true},             // bare name matches across groups
		{webA, "team-alpha/web", true},  // exact group/name
		{webA, "team-beta/web", false},  // wrong group
		{webB, "team-beta/web", true},   // same name, different group
		{bare, "api", true},             // groupless by bare name
		{webA, "team-alpha/api", false}, // wrong name
		{webA, "team/sub/web", false},
		{Connection{Name: "web", Group: "team/sub"}, "team/sub/web", true}, // nested group path
	}
	for _, c := range cases {
		if got := connMatches(c.conn, c.spec); got != c.want {
			t.Errorf("connMatches(%+v, %q) = %v, want %v", c.conn, c.spec, got, c.want)
		}
	}
}
