package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConnectionReadsLegacyFolderPath(t *testing.T) {
	var c Connection
	if err := json.Unmarshal([]byte(`{"name":"a","db_type":"sqlite","folder_path":"team/project"}`), &c); err != nil {
		t.Fatal(err)
	}
	if c.Group != "team/project" {
		t.Fatalf("Group = %q, want team/project (from legacy folder_path)", c.Group)
	}
}

func TestConnectionPrefersGroupOverLegacy(t *testing.T) {
	var c Connection
	if err := json.Unmarshal([]byte(`{"name":"a","group":"new","folder_path":"old"}`), &c); err != nil {
		t.Fatal(err)
	}
	if c.Group != "new" {
		t.Fatalf("Group = %q, want new", c.Group)
	}
}

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
