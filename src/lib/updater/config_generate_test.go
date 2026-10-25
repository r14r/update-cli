package updater

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func TestConfigShowOption(t *testing.T) {
	o, err := parseOptions([]string{"config", "--show"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.config || !o.configShow {
		t.Fatalf("config --show not selected: %#v", o)
	}
}

func TestConfigGenerateRepository(t *testing.T) {
	old := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	t.Cleanup(func() { os.Stdout = old })
	err = runConfigGenerate([]string{"--repo", "https://github.com/r14r/example-app.git"})
	_ = writer.Close()
	if err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	if result["projectName"] != "example-app" || result["mode"] != "pull" {
		t.Fatalf("wrong generated configuration: %s", output)
	}
	source, ok := result["source"].(map[string]any)
	if !ok || source["repository"] != "https://github.com/r14r/example-app.git" {
		t.Fatalf("wrong repository source: %s", output)
	}
}

func TestConfigGenerateRequiresRepository(t *testing.T) {
	if err := runConfigGenerate(nil); err == nil || !strings.Contains(err.Error(), "--repo") {
		t.Fatalf("expected missing repo error, got %v", err)
	}
}
