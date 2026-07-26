package templates

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLookupIsCaseInsensitive(t *testing.T) {
	value, err := Lookup("fAsTaPi")
	if err != nil {
		t.Fatal(err)
	}
	if value.Name != "FastAPI" || value.Setup == nil || len(value.Setup.Commands) == 0 {
		t.Fatalf("unexpected template: %#v", value)
	}
}

func TestNames(t *testing.T) {
	want := []string{"Laravel", "Django", "FastAPI", "Vue", "Go", "update-and-setup"}
	if got := Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() = %#v, want %#v", got, want)
	}
}

func TestLookupRejectsUnknownTemplate(t *testing.T) {
	if _, err := Lookup("Rails"); err == nil {
		t.Fatal("expected unknown template error")
	}
}

func TestBuiltinContainsUpdateAndSetup(t *testing.T) {
	value, err := Lookup("update-and-setup")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"update", "setup"}
	if !reflect.DeepEqual(value.NoParameter, want) {
		t.Fatalf("NoParameter = %#v, want %#v", value.NoParameter, want)
	}
}

func TestBuiltinSetupTemplatesStartWithDockerDown(t *testing.T) {
	for _, name := range []string{"Laravel", "Django", "FastAPI", "Vue", "Go"} {
		value, err := Lookup(name)
		if err != nil {
			t.Fatal(err)
		}
		if value.Setup == nil || len(value.Setup.Commands) == 0 {
			t.Fatalf("template %s has no setup commands", name)
		}
		if value.Setup.Commands[0] != DockerDownCommand {
			t.Fatalf("template %s first command = %q, want DockerDownCommand", name, value.Setup.Commands[0])
		}
	}
}

func TestEnsureBuiltinsMigratesUnchangedLegacyTemplate(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	current, err := Lookup("Laravel")
	if err != nil {
		t.Fatal(err)
	}
	legacy := clone(current)
	legacy.Setup.Commands = append([]string(nil), legacy.Setup.Commands[1:]...)
	file := File{SchemaVersion: SchemaVersion, Templates: []Template{legacy}}
	if err := Write(path, file); err != nil {
		t.Fatal(err)
	}
	created, updated, err := EnsureBuiltins(path)
	if err != nil {
		t.Fatal(err)
	}
	if created || !updated {
		t.Fatalf("unexpected result: created=%t updated=%t", created, updated)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	laravel, err := lookup(loaded, "Laravel")
	if err != nil {
		t.Fatal(err)
	}
	if laravel.Setup == nil || laravel.Setup.Commands[0] != DockerDownCommand {
		t.Fatalf("legacy template was not migrated: %#v", laravel)
	}
}

func TestEnsureBuiltinsPreservesCustomizedBuiltinWithoutDockerDown(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	file := File{SchemaVersion: SchemaVersion, Templates: []Template{
		{Name: "Laravel", Setup: &SetupConfig{Commands: []string{"custom laravel"}}},
	}}
	if err := Write(path, file); err != nil {
		t.Fatal(err)
	}
	_, _, err := EnsureBuiltins(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	laravel, err := lookup(loaded, "Laravel")
	if err != nil {
		t.Fatal(err)
	}
	if laravel.Setup == nil || !reflect.DeepEqual(laravel.Setup.Commands, []string{"custom laravel"}) {
		t.Fatalf("customized template was changed: %#v", laravel)
	}
}

func TestWriteDefaultsAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	created, err := WriteDefaults(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected templates file to be created")
	}
	file, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if file.SchemaVersion != SchemaVersion || len(file.Templates) != 6 {
		t.Fatalf("unexpected template file: %#v", file)
	}
	created, err = WriteDefaults(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("existing templates file should be preserved")
	}
}

func TestLoadRejectsDuplicateNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	data := `{"schemaVersion":1,"templates":[{"name":"Demo","setup":{"commands":["one"]}},{"name":"demo","setup":{"commands":["two"]}}]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "mehrfach") {
		t.Fatalf("expected duplicate-name error, got %v", err)
	}
}

func TestEnsureBuiltinsPreservesCustomAndAddsMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	file := File{SchemaVersion: SchemaVersion, Templates: []Template{
		{Name: "Laravel", Setup: &SetupConfig{Commands: []string{"custom laravel"}}},
		{Name: "MyTemplate", Setup: &SetupConfig{Commands: []string{"echo custom"}}},
	}}
	if err := Write(path, file); err != nil {
		t.Fatal(err)
	}
	created, updated, err := EnsureBuiltins(path)
	if err != nil {
		t.Fatal(err)
	}
	if created || !updated {
		t.Fatalf("unexpected result: created=%t updated=%t", created, updated)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	laravel, err := lookup(loaded, "Laravel")
	if err != nil {
		t.Fatal(err)
	}
	if laravel.Setup == nil || laravel.Setup.Commands[0] != "custom laravel" {
		t.Fatalf("existing template overwritten: %#v", laravel)
	}
	if _, err := lookup(loaded, "update-and-setup"); err != nil {
		t.Fatalf("missing built-in was not added: %v", err)
	}
	if _, err := lookup(loaded, "MyTemplate"); err != nil {
		t.Fatalf("custom template was lost: %v", err)
	}
}

func TestCatalogMergesGlobalTemplatesAndAllowsOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "templates.json")
	global := File{
		SchemaVersion: SchemaVersion,
		Templates: []Template{
			{Name: "Custom", Description: "global", Setup: &SetupConfig{Commands: []string{"echo custom"}}},
			{Name: "Go", Description: "overridden", Setup: &SetupConfig{Commands: []string{"echo global go"}}},
		},
	}
	if err := Write(path, global); err != nil {
		t.Fatal(err)
	}
	catalog, err := Catalog(path)
	if err != nil {
		t.Fatal(err)
	}
	custom, err := lookup(catalog, "Custom")
	if err != nil || custom.Description != "global" {
		t.Fatalf("unexpected custom template: %#v, %v", custom, err)
	}
	goTemplate, err := lookup(catalog, "Go")
	if err != nil || goTemplate.Description != "overridden" {
		t.Fatalf("global override not applied: %#v, %v", goTemplate, err)
	}
}

func TestEnsureCatalogAddsGlobalTemplateWithoutOverwritingLocal(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "local.json")
	globalPath := filepath.Join(dir, "global.json")
	local := File{SchemaVersion: SchemaVersion, Templates: []Template{
		{Name: "Go", Description: "local", Setup: &SetupConfig{Commands: []string{"echo local"}}},
	}}
	global := File{SchemaVersion: SchemaVersion, Templates: []Template{
		{Name: "Go", Description: "global", Setup: &SetupConfig{Commands: []string{"echo global"}}},
		{Name: "Custom", Description: "new", Setup: &SetupConfig{Commands: []string{"echo custom"}}},
	}}
	if err := Write(localPath, local); err != nil {
		t.Fatal(err)
	}
	if err := Write(globalPath, global); err != nil {
		t.Fatal(err)
	}
	_, updated, err := EnsureCatalog(localPath, globalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected catalog update")
	}
	result, err := Load(localPath)
	if err != nil {
		t.Fatal(err)
	}
	goTemplate, _ := lookup(result, "Go")
	if goTemplate.Description != "local" {
		t.Fatalf("local template overwritten: %#v", goTemplate)
	}
	if _, err := lookup(result, "Custom"); err != nil {
		t.Fatal(err)
	}
}
