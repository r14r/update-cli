package projectsetup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefineSetupYAMLWithOpenAICompatibleProvider(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "setup.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\ngo test ./...\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	refined := `schemaVersion: 2
project:
  name: Demo
  type: go
workflows:
  setup:
    tasks: [test]
tasks:
  test:
    steps:
      - id: test
        name: Tests
        go:
          action: test
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		payload, _ := json.Marshal(req)
		if !strings.Contains(string(payload), "ORIGINAL setup.sh") || !strings.Contains(string(payload), "DETERMINISTIC") {
			t.Fatalf("prompt missing inputs: %s", payload)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "```yaml\n" + refined + "```"}}}})
	}))
	defer server.Close()
	t.Setenv("UPDATE_CLI_AI_PROVIDER", "openai-compatible")
	t.Setenv("UPDATE_CLI_AI_BASE_URL", server.URL)
	t.Setenv("UPDATE_CLI_AI_MODEL", "test-model")
	t.Setenv("UPDATE_CLI_CONFIG_PATH", t.TempDir())
	out, result, err := RefineSetupYAMLWithAI(context.Background(), dir, scriptPath, refined)
	if err != nil {
		t.Fatal(err)
	}
	if result.Model != "test-model" || result.Provider != "openai-compatible" {
		t.Fatalf("result=%#v", result)
	}
	if !strings.Contains(out, "schemaVersion: 2") || strings.Contains(out, "```") {
		t.Fatalf("output=%q", out)
	}
}

func TestWithAIRequiresModel(t *testing.T) {
	t.Setenv("UPDATE_CLI_AI_PROVIDER", "ollama")
	t.Setenv("UPDATE_CLI_AI_MODEL", "")
	t.Setenv("UPDATE_CLI_AI_CONFIG", filepath.Join(t.TempDir(), "missing.json"))
	if _, err := loadAIConfig(); err == nil || !strings.Contains(err.Error(), "UPDATE_CLI_AI_MODEL") {
		t.Fatalf("err=%v", err)
	}
}

func TestRefineSetupYAMLRejectsInvalidAIManifest(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "setup.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\ngo test ./...\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "schemaVersion: 2\nunknown: true\n"}}}})
	}))
	defer server.Close()
	t.Setenv("UPDATE_CLI_AI_PROVIDER", "openai-compatible")
	t.Setenv("UPDATE_CLI_AI_BASE_URL", server.URL)
	t.Setenv("UPDATE_CLI_AI_MODEL", "test-model")
	t.Setenv("UPDATE_CLI_CONFIG_PATH", t.TempDir())
	_, _, err := RefineSetupYAMLWithAI(context.Background(), dir, scriptPath, "schemaVersion: 2\ntasks:\n  setup:\n    steps:\n      - shell: echo ok\n")
	if err == nil || !strings.Contains(err.Error(), "AI-Ergebnis wurde verworfen") {
		t.Fatalf("err=%v", err)
	}
}
