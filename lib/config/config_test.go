package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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
	if cfg.Source.DefaultUser != DefaultRepositoryUser {
		t.Fatalf("source.defaultUser = %q, want %q", cfg.Source.DefaultUser, DefaultRepositoryUser)
	}
}

func TestUpgradeWritesSchema9AndBackup(t *testing.T) {
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
	if int(got["schemaVersion"].(float64)) != 9 {
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

func TestNoParameterUpdateMayDisableSetup(t *testing.T) {
	actions, err := normalizedNoParameter(NoParameterConfig{"update", "no-setup"})
	if err != nil {
		t.Fatal(err)
	}
	want := NoParameterConfig{"update", "no-setup"}
	if !reflect.DeepEqual(actions, want) {
		t.Fatalf("actions = %#v, want %#v", actions, want)
	}
}

func TestNoParameterNoSetupRequiresUpdate(t *testing.T) {
	if _, err := normalizedNoParameter(NoParameterConfig{"no-setup"}); err == nil {
		t.Fatal("expected no-setup without update to be rejected")
	}
	if _, err := normalizedNoParameter(NoParameterConfig{"update", "setup", "no-setup"}); err == nil {
		t.Fatal("expected setup + no-setup to be rejected")
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

func TestInitDefaultsNoParameterToUpdateWithoutSetup(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := Init(root, InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	wantActions := []string{"update", "no-setup"}
	if !reflect.DeepEqual(cfg.NoParameterActions, wantActions) {
		t.Fatalf("new project no parameter = %#v, want %#v", cfg.NoParameterActions, wantActions)
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
	if !ok || len(got) != 2 || got[0] != "update" || got[1] != "no-setup" {
		t.Fatalf("persisted no parameter = %#v, want [update no-setup]", raw["no parameter"])
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

func TestSchema6RepositoryMigratesToPullMode(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"schemaVersion":6,"projectName":"demo","source":{"type":"repository","repository":"https://example.invalid/demo.git"},"releaseDir":"release","currentDir":"current","no parameter":["check"]}`
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != ModePull {
		t.Fatalf("mode = %q, want %q", cfg.Mode, ModePull)
	}
}

func TestModeSourceCompatibilityValidation(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	if _, err := Init(root, InitOptions{ProjectName: "demo", Mode: ModePull, SourceType: "download", Folder: downloads}); err == nil || !strings.Contains(err.Error(), "mode pull") {
		t.Fatalf("expected pull/download validation error, got %v", err)
	}
	if _, err := Init(root, InitOptions{ProjectName: "demo", Mode: ModeUpdate, SourceType: "repository", Repository: "https://example.invalid/demo.git"}); err == nil || !strings.Contains(err.Error(), "mode update") {
		t.Fatalf("expected update/repository validation error, got %v", err)
	}
}

func TestInitPullModeWithRepository(t *testing.T) {
	root := t.TempDir()
	cfg, err := Init(root, InitOptions{ProjectName: "demo", Mode: ModePull, SourceType: "repository", Repository: "https://example.invalid/demo.git"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != ModePull || cfg.Source.Type != "repository" {
		t.Fatalf("unexpected config: mode=%q source=%#v", cfg.Mode, cfg.Source)
	}
}

func TestNormalizeRepositorySpec(t *testing.T) {
	tests := []struct {
		name        string
		spec        string
		defaultUser string
		want        string
	}{
		{"full URL", "https://github.com/r14r/git-cli", "r1r", "https://github.com/r14r/git-cli.git"},
		{"full URL with git", "https://github.com/r14r/git-cli.git", "r1r", "https://github.com/r14r/git-cli.git"},
		{"user repo", "r14r/ollama-cli", "r1r", "https://github.com/r14r/ollama-cli.git"},
		{"repo only default", "ollama-cli", "r1r", "https://github.com/r1r/ollama-cli.git"},
		{"repo only configured user", "ollama-cli", "custom-user", "https://github.com/custom-user/ollama-cli.git"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeRepositorySpec(tt.spec, tt.defaultUser)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeRepositorySpecRejectsInvalidValues(t *testing.T) {
	for _, spec := range []string{"", "https://gitlab.com/r14r/demo", "a/b/c", "bad user/repo"} {
		if _, err := NormalizeRepositorySpec(spec, DefaultRepositoryUser); err == nil {
			t.Fatalf("invalid repository %q accepted", spec)
		}
	}
}

func TestInitWritesDefaultRepositoryUser(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := Init(root, InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Source.DefaultUser != DefaultRepositoryUser {
		t.Fatalf("source.defaultUser = %q, want %q", cfg.Source.DefaultUser, DefaultRepositoryUser)
	}
	body, err := os.ReadFile(filepath.Join(root, ConfigDirName, ConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"defaultUser": "`+DefaultRepositoryUser+`"`) {
		t.Fatalf("config.json does not contain source.defaultUser: %s", body)
	}
}

func TestCheckCurrentConfig(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	if _, err := Init(root, InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads}); err != nil {
		t.Fatal(err)
	}
	result, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid || result.MigrationNeeded || result.SchemaVersion != SchemaVersion {
		t.Fatalf("unexpected check result: %#v", result)
	}
}

func TestCheckReportsMigrationWithoutWriting(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ConfigFileName)
	legacy := `{
  "schemaVersion": 6,
  "projectName": "demo",
  "source": {"type": "download", "folder": "/tmp"},
  "releaseDir": "release",
  "currentDir": "current"
}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	result, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid || !result.MigrationNeeded || result.SchemaVersion != 6 || result.CurrentSchema != SchemaVersion {
		t.Fatalf("unexpected check result: %#v", result)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("config --check must not modify config.json")
	}
}

func TestUpgradeKeepsBackupInsideUpdateCLIConfigDirectory(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	configDir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configDir, ConfigFileName)
	data := `{"schemaVersion":6,"projectName":"demo","mode":"update","source":{"type":"download","folder":"` + downloads + `"},"releaseDir":"release","currentDir":"current","no parameter":["check"],"setup":{"commands":[]},"backup":{"directory":"backup","keep":3},"retention":{"releases":5},"sync":{"preserve":[".gitignore"]},"security":{"allowHttp":false,"maxArchiveBytes":2147483648,"maxUncompressedBytes":8589934592,"maxFileBytes":2147483648,"maxEntries":100000,"maxCompressionRatio":200},"docker":{"lifecycle":"auto"},"healthcheck":{}}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := Upgrade(root)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Changed || r.ConfigFile != path || r.BackupFile == "" {
		t.Fatalf("unexpected upgrade result: %#v", r)
	}
	if filepath.Dir(r.BackupFile) != configDir {
		t.Fatalf("backup dir = %q, want %q", filepath.Dir(r.BackupFile), configDir)
	}
}

func TestLegacyTopLevelDefaultUserIsAcceptedAndCanonicalized(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.json")
	data := `{"schemaVersion":8,"projectName":"demo","mode":"update","defaultUser":"legacyuser","source":{"type":"download","folder":"$HOME/Downloads"},"releaseDir":"release","currentDir":"current","backup":{"directory":"backup","keep":3},"retention":{"releases":5},"security":{"allowHttp":false,"maxArchiveBytes":2147483648,"maxUncompressedBytes":8589934592,"maxFileBytes":2147483648,"maxEntries":100000,"maxCompressionRatio":200},"docker":{"lifecycle":"auto"}}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	fc, err := readConfigFile(root, path)
	if err != nil {
		t.Fatal(err)
	}
	fc, changed, err := migrate(fc)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("legacy top-level defaultUser should require canonicalization")
	}
	if fc.DefaultUser != "" {
		t.Fatalf("legacy defaultUser not cleared: %q", fc.DefaultUser)
	}
	if fc.Source == nil || fc.Source.DefaultUser != "legacyuser" {
		t.Fatalf("source.defaultUser = %#v, want legacyuser", fc.Source)
	}
}

func TestRepairProjectConfigMigratesLegacyAndRemovesUnknownFields(t *testing.T) {
	root := t.TempDir()
	globalDir := t.TempDir()
	t.Setenv(globalConfigDirEnv, globalDir)
	if err := os.MkdirAll(filepath.Join(root, ConfigDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	local := `{
  "schemaVersion": 8,
  "projectName": "demo",
  "defaultUser": "r1r",
  "mode": "broken",
  "source": {"type":"download","folder":"$HOME/Downloads","obsolete":true},
  "releaseDir":"release",
  "currentDir":"current",
  "setup":{"commands":[],"keepRsyncOnError":"true","old":1},
  "docker":{"lifecycle":"invalid"},
  "unknownTop":"remove-me"
}`
	if err := os.WriteFile(filepath.Join(root, ConfigDirName, ConfigFileName), []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}
	global := `{"defaultUser":"globaluser","mode":"explode","source":{"type":"wat"},"sync":{"preserve":[".env","../unsafe"],"obsolete":true},"backup":{"keep":-2},"retention":{"releases":-3},"security":{"maxEntries":0},"docker":{"lifecycle":"wat"},"healthcheck":{"type":"wat","timeoutSeconds":-5},"unknownGlobal":1}`
	if err := os.WriteFile(filepath.Join(globalDir, ConfigFileName), []byte(global), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := RepairProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Local.Changed {
		t.Fatal("expected local config repair")
	}
	if result.Global == nil || !result.Global.Changed {
		t.Fatal("expected global config repair")
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Source.DefaultUser != "r1r" {
		t.Fatalf("default user=%q", cfg.Source.DefaultUser)
	}
	if cfg.Mode != ModeUpdate {
		t.Fatalf("mode=%q", cfg.Mode)
	}
	if !cfg.KeepRsyncOnSetupError {
		t.Fatal("keepRsyncOnError was not coerced")
	}
	body, _ := os.ReadFile(filepath.Join(root, ConfigDirName, ConfigFileName))
	var repaired map[string]any
	if err := json.Unmarshal(body, &repaired); err != nil {
		t.Fatal(err)
	}
	if _, exists := repaired["defaultUser"]; exists {
		t.Fatalf("legacy top-level defaultUser still present: %s", body)
	}
	for _, bad := range []string{"unknownTop", "obsolete"} {
		if strings.Contains(string(body), bad) {
			t.Fatalf("repaired local config still contains %q: %s", bad, body)
		}
	}
	globalBody, _ := os.ReadFile(filepath.Join(globalDir, ConfigFileName))
	if strings.Contains(string(globalBody), `"defaultUser":`) && !strings.Contains(string(globalBody), `"source"`) {
		t.Fatalf("legacy global defaultUser not migrated: %s", globalBody)
	}
	if strings.Contains(string(globalBody), "unknownGlobal") {
		t.Fatalf("unknown global field not removed: %s", globalBody)
	}
	for _, bad := range []string{"explode", `"type": "wat"`, "../unsafe", `"keep": -2`, `"releases": -3`, `"maxEntries": 0`, `"lifecycle": "wat"`, `"timeoutSeconds": -5`} {
		if strings.Contains(string(globalBody), bad) {
			t.Fatalf("invalid global value %q was not repaired: %s", bad, globalBody)
		}
	}
}

func TestResolveRootFromCurrentPrefersParentOverNestedRuntimeState(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{
		filepath.Join(root, ConfigDirName),
		filepath.Join(root, "current", ConfigDirName),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	current := filepath.Join(root, "current")
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
	gotCanonical, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatal(err)
	}
	if gotCanonical != want {
		t.Fatalf("ResolveRoot from polluted current = %q (canonical %q), want parent %q", got, gotCanonical, want)
	}
}

func TestRepairProjectConfigMigratesLegacyKeepRsyncOnErrorLocations(t *testing.T) {
	root := t.TempDir()
	global := t.TempDir()
	t.Setenv(globalConfigDirEnv, global)
	if err := os.MkdirAll(filepath.Join(root, ConfigDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{
  "schemaVersion": 9,
  "projectName": "demo",
  "source": {"type": "download", "folder": "."},
  "releaseDir": "release",
  "currentDir": "current",
  "backup": {"directory": "backup", "keep": 3},
  "retention": {"releases": 5},
  "sync": {"preserve": [".env"], "keepRsyncOnError": true},
  "security": {"maxArchiveBytes": 2147483648, "maxUncompressedBytes": 8589934592, "maxFileBytes": 2147483648, "maxEntries": 100000, "maxCompressionRatio": 200}
}`
	path := filepath.Join(root, ConfigDirName, ConfigFileName)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := RepairProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Local.Changed {
		t.Fatal("expected repair to persist canonical setup policy")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `"keepRsyncOnError": true`) || !strings.Contains(text, `"setup"`) {
		t.Fatalf("canonical setup.keepRsyncOnError missing after repair:\n%s", text)
	}
	if strings.Contains(text, `"sync": {\n    "keepRsyncOnError"`) {
		t.Fatalf("legacy sync.keepRsyncOnError remained after repair:\n%s", text)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.KeepRsyncOnSetupError {
		t.Fatal("repaired policy was not effective")
	}
}

func TestLoadResilientRepairsFutureSchemaAndLegacyFields(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{
  "schemaVersion": 99,
  "projectName": "demo",
  "defaultUser": "legacyuser",
  "source": {"type": "download", "folder": "$HOME/Downloads"},
  "releaseDir": "release",
  "currentDir": "current",
  "no parameter": ["update", "no-setup"],
  "setup": {"commands": []},
  "backup": {"directory": "backup", "keep": 3},
  "retention": {"releases": 5},
  "sync": {"preserve": [".env"], "obsolete": true},
  "unknownTop": true
}`
	path := filepath.Join(stateDir, ConfigFileName)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, repair, err := LoadResilient(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if repair == nil || !repair.Local.Changed {
		t.Fatal("expected automatic repair")
	}
	if cfg.Source.DefaultUser != "legacyuser" {
		t.Fatalf("source.defaultUser = %q", cfg.Source.DefaultUser)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"defaultUser": "legacyuser"`) && !strings.Contains(string(data), `"source"`) {
		t.Fatalf("legacy defaultUser remained top-level: %s", data)
	}
	if !strings.Contains(string(data), `"schemaVersion": 9`) {
		t.Fatalf("schema not migrated: %s", data)
	}
}

func TestLoadAcceptsNoParamAliasAsDirectUpdate(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	dir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{
  "schemaVersion": 9,
  "projectName": "demo",
  "mode": "update",
  "source": {"type": "download", "folder": "` + downloads + `"},
  "releaseDir": "release",
  "currentDir": "current",
  "no-param": "update"
}`
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.NoParameterActions, []string{"update"}) {
		t.Fatalf("no-param actions = %#v, want [update]", cfg.NoParameterActions)
	}
}
