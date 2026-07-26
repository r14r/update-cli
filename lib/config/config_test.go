package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"release-updater/lib/templates"
)

func TestInitRequiresExplicitProjectName(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("PROJECTNAME=ignored\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Init(root, InitOptions{})
	if err == nil || !strings.Contains(err.Error(), "update-cli --init mediastudio") {
		t.Fatalf("expected explicit project name hint, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, ConfigDirName, ConfigFileName)); !os.IsNotExist(statErr) {
		t.Fatalf("config should not be created without project name, stat error: %v", statErr)
	}
}

func TestInitWithExplicitProjectName(t *testing.T) {
	root := t.TempDir()
	cfg, err := Init(root, InitOptions{ProjectName: "mediastudio"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProjectName != "mediastudio" {
		t.Fatalf("unexpected project name: %q", cfg.ProjectName)
	}
	if cfg.ConfigFile != filepath.Join(root, ConfigDirName, ConfigFileName) {
		t.Fatalf("unexpected config file: %q", cfg.ConfigFile)
	}

	data, err := os.ReadFile(cfg.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	var fileConfig FileConfig
	if err := json.Unmarshal(data, &fileConfig); err != nil {
		t.Fatal(err)
	}
	if fileConfig.SchemaVersion != SchemaVersion || fileConfig.ProjectName != "mediastudio" {
		t.Fatalf("unexpected file config: %#v", fileConfig)
	}
}

func TestInitDoesNotOverwriteWithoutForce(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root, InitOptions{ProjectName: "one"}); err != nil {
		t.Fatal(err)
	}
	_, err := Init(root, InitOptions{ProjectName: "two"})
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected overwrite hint, got %v", err)
	}

	cfg, err := Init(root, InitOptions{ProjectName: "two", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProjectName != "two" {
		t.Fatalf("force did not replace config: %q", cfg.ProjectName)
	}
}

func TestLoadRequiresConfigJSON(t *testing.T) {
	_, err := Load(t.TempDir(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "update-cli --init") {
		t.Fatalf("expected init hint, got %v", err)
	}
}

func TestLoadReportsLegacyConfigDirectory(t *testing.T) {
	root := t.TempDir()
	legacyDir := filepath.Join(root, LegacyConfigDirName)
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, ConfigFileName), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(root, "")
	if err == nil || !strings.Contains(err.Error(), "veraltete Updater-Konfiguration") {
		t.Fatalf("expected legacy config hint, got %v", err)
	}
}

func TestLoadRejectsUnknownJSONFields(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"schemaVersion":1,"projectName":"demo","downloadDir":"~/Downloads","releaseDir":"release","currentDir":"current","typo":true}`
	if err := os.WriteFile(filepath.Join(configDir, ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(root, "")
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestLoadRejectsUnsafeProjectDirectory(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"schemaVersion":1,"projectName":"demo","downloadDir":"~/Downloads","releaseDir":"../release","currentDir":"current"}`
	if err := os.WriteFile(filepath.Join(configDir, ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(root, "")
	if err == nil || !strings.Contains(err.Error(), "außerhalb") {
		t.Fatalf("expected path safety error, got %v", err)
	}
}

func TestInitCreatesSetupArea(t *testing.T) {
	root := t.TempDir()
	cfg, err := Init(root, InitOptions{ProjectName: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(cfg.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"setup"`) || !strings.Contains(string(data), `"commands"`) {
		t.Fatalf("setup area missing from config: %s", data)
	}
}

func TestLoadReadsSetupCommands(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"schemaVersion":1,"projectName":"demo","downloadDir":"~/Downloads","releaseDir":"release","currentDir":"current","setup":{"commands":[" just app-init ","just up"]}}`
	if err := os.WriteFile(filepath.Join(configDir, ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.SetupCommands) != 2 || cfg.SetupCommands[0] != "just app-init" || cfg.SetupCommands[1] != "just up" {
		t.Fatalf("unexpected setup commands: %#v", cfg.SetupCommands)
	}
}

func TestLoadRejectsEmptySetupCommand(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"schemaVersion":1,"projectName":"demo","downloadDir":"~/Downloads","releaseDir":"release","currentDir":"current","setup":{"commands":[" "]}}`
	if err := os.WriteFile(filepath.Join(configDir, ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(root, "")
	if err == nil || !strings.Contains(err.Error(), "setup.commands[0] ist leer") {
		t.Fatalf("expected empty setup command error, got %v", err)
	}
}

func TestApplySetupTemplatePreservesProjectConfiguration(t *testing.T) {
	root := t.TempDir()
	cfg, err := Init(root, InitOptions{ProjectName: "demo"})
	if err != nil {
		t.Fatal(err)
	}

	path, name, commands, err := ApplySetupTemplate(root, "laravel")
	if err != nil {
		t.Fatal(err)
	}
	if path != cfg.ConfigFile || name != "Laravel" || len(commands) != 5 {
		t.Fatalf("unexpected apply result: path=%q name=%q commands=%#v", path, name, commands)
	}

	loaded, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ProjectName != "demo" || len(loaded.SetupCommands) != 5 {
		t.Fatalf("unexpected loaded config: %#v", loaded)
	}
	if loaded.SetupCommands[0] != templates.DockerDownCommand {
		t.Fatalf("unexpected first command: %q", loaded.SetupCommands[0])
	}
}

func TestApplySetupTemplateRejectsUnknownTemplateWithoutChangingConfig(t *testing.T) {
	root := t.TempDir()
	cfg, err := Init(root, InitOptions{ProjectName: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(cfg.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, _, err := ApplySetupTemplate(root, "Rails"); err == nil {
		t.Fatal("expected unknown template error")
	}
	after, err := os.ReadFile(cfg.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("configuration changed after rejected template")
	}
}

func TestInitCreatesBackupAndRetentionDefaults(t *testing.T) {
	root := t.TempDir()
	cfg, err := Init(root, InitOptions{ProjectName: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BackupRoot != filepath.Join(root, "backup") || cfg.KeepBackups != 3 || cfg.KeepReleases != 5 {
		t.Fatalf("unexpected retention defaults: %#v", cfg)
	}
	data, err := os.ReadFile(cfg.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"backup"`) || !strings.Contains(string(data), `"retention"`) {
		t.Fatalf("backup/retention config missing: %s", data)
	}
}

func TestLoadOldConfigurationUsesRetentionDefaults(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"schemaVersion":1,"projectName":"demo","downloadDir":"~/Downloads","releaseDir":"release","currentDir":"current","setup":{"commands":[]}}`
	if err := os.WriteFile(filepath.Join(configDir, ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.KeepBackups != 3 || cfg.KeepReleases != 5 || cfg.BackupRoot != filepath.Join(root, "backup") {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
}

func TestInitCreatesNoParameterHelpDefault(t *testing.T) {
	root := t.TempDir()
	cfg, err := Init(root, InitOptions{ProjectName: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(cfg.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"no parameter": [
    "help"
  ]`) {
		t.Fatalf("no parameter default missing: %s", data)
	}
	if len(cfg.NoParameterActions) != 1 || cfg.NoParameterActions[0] != "help" {
		t.Fatalf("unexpected default actions: %#v", cfg.NoParameterActions)
	}
}

func TestLoadReadsNoParameterSetup(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"schemaVersion":1,"projectName":"demo","downloadDir":"~/Downloads","releaseDir":"release","currentDir":"current","no parameter":"setup"}`
	if err := os.WriteFile(filepath.Join(configDir, ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.NoParameterActions) != 1 || cfg.NoParameterActions[0] != "setup" {
		t.Fatalf("unexpected no-parameter actions: %#v", cfg.NoParameterActions)
	}
}

func TestLoadOldConfigurationDefaultsNoParameterToHelp(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"schemaVersion":1,"projectName":"demo","downloadDir":"~/Downloads","releaseDir":"release","currentDir":"current"}`
	if err := os.WriteFile(filepath.Join(configDir, ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.NoParameterActions) != 1 || cfg.NoParameterActions[0] != "help" {
		t.Fatalf("unexpected compatibility default: %#v", cfg.NoParameterActions)
	}
}

func TestLoadRejectsInvalidNoParameterAction(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"schemaVersion":1,"projectName":"demo","downloadDir":"~/Downloads","releaseDir":"release","currentDir":"current","no parameter":"invalid"}`
	if err := os.WriteFile(filepath.Join(configDir, ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(root, "")
	if err == nil || !strings.Contains(err.Error(), "unterstützt nur") {
		t.Fatalf("expected no-parameter validation error, got %v", err)
	}
}

func TestLoadReadsMultipleNoParameterActions(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"schemaVersion":3,"projectName":"demo","downloadDir":"~/Downloads","releaseDir":"release","currentDir":"current","no parameter":["setup","update"]}`
	if err := os.WriteFile(filepath.Join(configDir, ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"update", "setup"}
	if len(cfg.NoParameterActions) != len(want) || cfg.NoParameterActions[0] != want[0] || cfg.NoParameterActions[1] != want[1] {
		t.Fatalf("unexpected no-parameter actions: %#v", cfg.NoParameterActions)
	}
}

func TestLoadRejectsHelpCombinedWithOtherAction(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"schemaVersion":3,"projectName":"demo","downloadDir":"~/Downloads","releaseDir":"release","currentDir":"current","no parameter":["help","setup"]}`
	if err := os.WriteFile(filepath.Join(configDir, ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(root, "")
	if err == nil || !strings.Contains(err.Error(), "nicht mit weiteren Befehlen") {
		t.Fatalf("expected invalid action combination, got %v", err)
	}
}

func TestUpgradeMigratesSchemaOneAndAddsCurrentDefaults(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configDir, ConfigFileName)
	data := `{"schemaVersion":1,"projectName":"demo","downloadDir":"/tmp/custom-downloads","releaseDir":"versions","currentDir":"active","setup":{"commands":["just build"]}}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Upgrade(root)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.PreviousSchema != 1 || result.CurrentSchema != SchemaVersion {
		t.Fatalf("unexpected upgrade result: %#v", result)
	}
	if result.BackupFile == "" {
		t.Fatal("expected config backup")
	}
	if _, err := os.Stat(result.BackupFile); err != nil {
		t.Fatalf("backup missing: %v", err)
	}

	upgradedData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var upgraded FileConfig
	if err := json.Unmarshal(upgradedData, &upgraded); err != nil {
		t.Fatal(err)
	}
	if upgraded.SchemaVersion != SchemaVersion {
		t.Fatalf("schema not upgraded: %#v", upgraded)
	}
	if upgraded.ProjectName != "demo" || upgraded.Source == nil || upgraded.Source.Type != "download" || upgraded.Source.Folder != "/tmp/custom-downloads" || upgraded.ReleaseDir != "versions" || upgraded.CurrentDir != "active" {
		t.Fatalf("existing settings were not preserved: %#v", upgraded)
	}
	if upgraded.DownloadDir != "" {
		t.Fatalf("legacy downloadDir should be removed after migration: %#v", upgraded)
	}
	if len(upgraded.NoParameter) != 1 || upgraded.NoParameter[0] != "help" || upgraded.Backup == nil || upgraded.Backup.Directory != "backup" || upgraded.Backup.Keep != 3 {
		t.Fatalf("defaults missing: %#v", upgraded)
	}
	if upgraded.Retention == nil || upgraded.Retention.Releases != 5 || upgraded.Setup == nil || len(upgraded.Setup.Commands) != 1 {
		t.Fatalf("nested defaults or setup missing: %#v", upgraded)
	}
}

func TestUpgradeCurrentConfigurationIsNoOp(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root, InitOptions{ProjectName: "demo"}); err != nil {
		t.Fatal(err)
	}
	result, err := Upgrade(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed || result.BackupFile != "" || result.PreviousSchema != SchemaVersion {
		t.Fatalf("expected no-op upgrade, got %#v", result)
	}
}

func TestUpgradeRejectsFutureSchema(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"schemaVersion":99,"projectName":"demo","downloadDir":"~/Downloads","releaseDir":"release","currentDir":"current"}`
	if err := os.WriteFile(filepath.Join(configDir, ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Upgrade(root)
	if err == nil || !strings.Contains(err.Error(), "neuer") {
		t.Fatalf("expected future schema rejection, got %v", err)
	}
}

func TestInitCreatesTemplatesFile(t *testing.T) {
	root := t.TempDir()
	cfg, err := Init(root, InitOptions{ProjectName: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TemplatesFile != filepath.Join(root, ConfigDirName, TemplatesFileName) {
		t.Fatalf("unexpected templates path: %q", cfg.TemplatesFile)
	}
	data, err := os.ReadFile(cfg.TemplatesFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"update-and-setup"`) {
		t.Fatalf("base template missing: %s", data)
	}
}

func TestInitCanApplyTemplate(t *testing.T) {
	root := t.TempDir()
	cfg, err := Init(root, InitOptions{ProjectName: "demo", UseTemplate: "update-and-setup"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"update", "setup"}
	if len(cfg.NoParameterActions) != len(want) || cfg.NoParameterActions[0] != want[0] || cfg.NoParameterActions[1] != want[1] {
		t.Fatalf("unexpected actions: %#v", cfg.NoParameterActions)
	}
}

func TestApplyTemplateReadsLocalTemplatesFile(t *testing.T) {
	root := t.TempDir()
	cfg, err := Init(root, InitOptions{ProjectName: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	data := `{
  "schemaVersion": 1,
  "templates": [
    {
      "name": "Custom",
      "no parameter": ["setup"],
      "setup": {"commands": ["echo custom"]}
    }
  ]
}`
	if err := os.WriteFile(cfg.TemplatesFile, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, selected, err := ApplyTemplate(root, "custom")
	if err != nil {
		t.Fatal(err)
	}
	if selected.Name != "Custom" {
		t.Fatalf("unexpected template: %#v", selected)
	}
	loaded, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.SetupCommands) != 1 || loaded.SetupCommands[0] != "echo custom" || len(loaded.NoParameterActions) != 1 || loaded.NoParameterActions[0] != "setup" {
		t.Fatalf("template not applied: %#v", loaded)
	}
}

func TestInitCreatesDownloadSourceByDefault(t *testing.T) {
	root := t.TempDir()
	cfg, err := Init(root, InitOptions{ProjectName: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SourceType != "download" || !strings.HasSuffix(filepath.ToSlash(cfg.SourceFolder), "/Downloads") {
		t.Fatalf("unexpected source configuration: %#v", cfg)
	}
}

func TestInitSupportsURLSource(t *testing.T) {
	root := t.TempDir()
	cfg, err := Init(root, InitOptions{
		ProjectName: "demo",
		SourceType:  "url",
		URL:         "https://example.test/demo-v1.2.3.zip",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SourceType != "url" || cfg.SourceURL != "https://example.test/demo-v1.2.3.zip" {
		t.Fatalf("unexpected URL source: %#v", cfg)
	}
}

func TestWithSourceOverridesSelectsRepository(t *testing.T) {
	cfg := Config{SourceType: "download", SourceFolder: "/tmp/downloads"}
	updated, err := WithSourceOverrides(cfg, "repository", "", "", "https://example.test/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	if updated.SourceType != "repository" || updated.SourceRepository == "" {
		t.Fatalf("unexpected repository source: %#v", updated)
	}
}

func TestLoadExpandsEnvironmentVariablesInSourceFolder(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(filepath.Join(home, "Downloads"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	configDir := filepath.Join(root, ConfigDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"schemaVersion":5,"projectName":"demo","source":{"type":"download","folder":"$HOME/Downloads"},"releaseDir":"release","currentDir":"current","no parameter":["help"],"setup":{"commands":[]},"backup":{"directory":"backup","keep":3},"retention":{"releases":5}}`
	if err := os.WriteFile(filepath.Join(configDir, ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "Downloads")
	if cfg.SourceFolder != want {
		t.Fatalf("SourceFolder=%s want=%s", cfg.SourceFolder, want)
	}
}
