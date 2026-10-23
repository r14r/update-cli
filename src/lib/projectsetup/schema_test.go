package projectsetup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSchemaVersion(t *testing.T) {
	if SchemaVersion != 2 {
		t.Fatalf("SchemaVersion = %d, want 2", SchemaVersion)
	}
}

func TestSchemaJSONIsValidAndCanonical(t *testing.T) {
	data, err := SchemaJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Fatal("SchemaJSON returned invalid JSON")
	}
	text := string(data)
	for _, needle := range []string{`"schemaVersion"`, `"tasks"`, `"run"`, `"command"`, `"dockerCompose"`} {
		if !strings.Contains(text, needle) {
			t.Fatalf("schema missing %s", needle)
		}
	}
	for _, needle := range []string{`"update"`, `"source"`, `"sync"`, `"keepOnSetupError"`, `"healthcheck"`} {
		if !strings.Contains(text, needle) {
			t.Fatalf("schema missing project update setting %s", needle)
		}
	}
	for _, forbidden := range []string{`"security"`, `"defaultUser"`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("host/user setting must not be project-overridable: %s", forbidden)
		}
	}
}

func TestSaveSchemaCreatesParentAndWritesSameSchema(t *testing.T) {
	target := filepath.Join(t.TempDir(), "nested", "update-cli.schema.json")
	got, err := SaveSchema(target)
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(target)
	if got != abs {
		t.Fatalf("path = %q, want %q", got, abs)
	}
	written, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	want, err := SchemaJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(want) {
		t.Fatal("saved schema differs from viewed schema")
	}
}
