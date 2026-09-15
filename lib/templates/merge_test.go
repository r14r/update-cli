package templates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMergedGlobalThenLocal(t *testing.T) {
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global.json")
	localPath := filepath.Join(dir, "local.json")
	global := File{SchemaVersion: 1, Templates: []Template{
		{Name: "Go", Description: "global go", Preserve: []string{"global/"}},
		{Name: "Docker", Description: "global docker"},
	}}
	local := File{SchemaVersion: 1, Templates: []Template{
		{Name: "Go", Description: "local go", Preserve: []string{"local/"}},
		{Name: "Python", Description: "local python"},
	}}
	if err := Save(globalPath, global); err != nil {
		t.Fatal(err)
	}
	if err := Save(localPath, local); err != nil {
		t.Fatal(err)
	}
	merged, err := LoadMerged(globalPath, localPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(merged.Templates) != 3 {
		t.Fatalf("template count = %d", len(merged.Templates))
	}
	goTemplate, err := LookupMerged(globalPath, localPath, "go")
	if err != nil {
		t.Fatal(err)
	}
	if goTemplate.Description != "local go" || len(goTemplate.Preserve) != 1 || goTemplate.Preserve[0] != "local/" {
		t.Fatalf("local override not applied: %#v", goTemplate)
	}
	if _, err := os.Stat(localPath); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMergedFallsBackToBuiltinsWithoutFiles(t *testing.T) {
	f, err := LoadMerged(filepath.Join(t.TempDir(), "missing-global.json"), filepath.Join(t.TempDir(), "missing-local.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Templates) == 0 {
		t.Fatal("expected built-in templates")
	}
}
