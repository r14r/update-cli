package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMigratesSchema5DefaultsInMemory(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	dir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := map[string]any{
		"schemaVersion": 5,
		"projectName":   "demo",
		"source":        map[string]any{"type": "download", "folder": downloads},
		"releaseDir":    "release", "currentDir": "current",
		"no parameter": []string{"help"},
		"setup":        map[string]any{"commands": []string{}},
		"backup":       map[string]any{"directory": "backup", "keep": 3},
		"retention":    map[string]any{"releases": 5},
	}
	b, _ := json.MarshalIndent(old, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Preserve) == 0 {
		t.Fatal("schema migration did not add preserve defaults")
	}
	if cfg.Security.MaxArchiveBytes <= 0 || cfg.Security.MaxEntries <= 0 {
		t.Fatalf("security defaults missing: %#v", cfg.Security)
	}
}

func TestUpgradeWritesSchema6AndBackup(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	dir := filepath.Join(root, ConfigDirName)
	_ = os.MkdirAll(dir, 0o755)
	data := `{"schemaVersion":5,"projectName":"demo","source":{"type":"download","folder":"` + downloads + `"},"releaseDir":"release","currentDir":"current","no parameter":["help"],"setup":{"commands":[]},"backup":{"directory":"backup","keep":3},"retention":{"releases":5}}`
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Upgrade(root)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Changed || r.BackupFile == "" {
		t.Fatalf("unexpected upgrade: %#v", r)
	}
	b, err := os.ReadFile(filepath.Join(dir, ConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if int(got["schemaVersion"].(float64)) != 6 {
		t.Fatalf("schema not upgraded: %v", got["schemaVersion"])
	}
}

func TestLoadSchema6AcceptsHistoricalNoParameterCheck(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	dir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{
  "schemaVersion": 6,
  "projectName": "update-cli",
  "source": {"type": "download", "folder": "` + downloads + `"},
  "releaseDir": "release",
  "currentDir": "current",
  "no parameter": ["check"]
}`
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.NoParameterActions) != 1 || cfg.NoParameterActions[0] != "check" {
		t.Fatalf("no parameter not preserved: %#v", cfg.NoParameterActions)
	}
}

func TestNoParameterCheckMayEnableSetupAfterConfirmedUpdate(t *testing.T) {
	actions, err := normalizedNoParameter(NoParameterConfig{"check", "setup"})
	if err != nil {
		t.Fatal(err)
	}
	want := NoParameterConfig{"check", "setup"}
	if len(actions) != len(want) {
		t.Fatalf("actions = %#v, want %#v", actions, want)
	}
	for i := range want {
		if actions[i] != want[i] {
			t.Fatalf("actions = %#v, want %#v", actions, want)
		}
	}
}

func TestNoParameterCheckAndUpdateRemainInvalid(t *testing.T) {
	_, err := normalizedNoParameter(NoParameterConfig{"check", "update"})
	if err == nil {
		t.Fatal("expected check + update to be rejected")
	}
}

func TestResolveRootFindsProjectConfigurationFromCurrentSubdirectory(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, ConfigFileName), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(root, "current", "nested")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(current); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()

	got, err := ResolveRoot("")
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	want, err = filepath.Abs(want)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("ResolveRoot from current subdirectory = %q, want canonical %q", got, want)
	}
}

func TestLoadAddsGitignoreToExistingPreserveList(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	dir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"schemaVersion":6,"projectName":"demo","source":{"type":"download","folder":"` + downloads + `"},"releaseDir":"release","currentDir":"current","no parameter":["help"],"setup":{"commands":[]},"backup":{"directory":"backup","keep":3},"retention":{"releases":5},"sync":{"preserve":[".env","data/"]}}`
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, value := range cfg.Preserve {
		if value == ".gitignore" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf(".gitignore not added to existing preserve list: %#v", cfg.Preserve)
	}
}

func TestInitDefaultsNoParameterToCheck(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := Init(root, InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.NoParameterActions) != 1 || cfg.NoParameterActions[0] != "check" {
		t.Fatalf("new project no parameter = %#v, want [check]", cfg.NoParameterActions)
	}
	b, err := os.ReadFile(filepath.Join(root, ConfigDirName, ConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	got, ok := raw["no parameter"].([]any)
	if !ok || len(got) != 1 || got[0] != "check" {
		t.Fatalf("persisted no parameter = %#v, want [check]", raw["no parameter"])
	}
}

func TestDockerLifecycleDefaultsAndValidation(t *testing.T) {
	for _, tc := range []struct {
		name, lifecycle string
		wantErr         bool
	}{
		{name: "default", lifecycle: ""},
		{name: "auto", lifecycle: "auto"},
		{name: "disabled", lifecycle: "disabled"},
		{name: "required", lifecycle: "required"},
		{name: "invalid", lifecycle: "foo", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			downloads := t.TempDir()
			dir := filepath.Join(root, ConfigDirName)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			docker := ""
			if tc.lifecycle != "" {
				docker = `,"docker":{"lifecycle":"` + tc.lifecycle + `"}`
			}
			data := `{"schemaVersion":6,"projectName":"demo","source":{"type":"download","folder":"` + downloads + `"},"releaseDir":"release","currentDir":"current","no parameter":["check"]` + docker + `}`
			if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(root, "")
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), `ungültiger Docker-Lifecycle "foo"; erlaubt: auto, disabled, required`) {
					t.Fatalf("expected lifecycle validation error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := tc.lifecycle
			if want == "" {
				want = "auto"
			}
			if cfg.Docker.Lifecycle != want {
				t.Fatalf("Docker lifecycle = %q, want %q", cfg.Docker.Lifecycle, want)
			}
		})
	}
}
