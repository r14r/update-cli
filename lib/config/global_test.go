package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGlobalConfigDirFromExecutable(t *testing.T) {
	got, err := globalConfigDirFromExecutable("/usr/local/bin/update-cli")
	if err != nil {
		t.Fatal(err)
	}
	want := "/usr/local/etc/update-cli"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLoadMergesGlobalThenLocalConfig(t *testing.T) {
	root := t.TempDir()
	globalDir := t.TempDir()
	t.Setenv(globalConfigDirEnv, globalDir)
	if err := os.MkdirAll(filepath.Join(root, ConfigDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	global := `{
  "source": {"defaultUser": "globaluser"},
  "sync": {"preserve": ["global/", ".env"]},
  "retention": {"releases": 9}
}`
	if err := os.WriteFile(filepath.Join(globalDir, ConfigFileName), []byte(global), 0o644); err != nil {
		t.Fatal(err)
	}
	local := `{
  "schemaVersion": 8,
  "projectName": "demo",
  "mode": "update",
  "source": {"type": "download", "folder": "` + t.TempDir() + `"},
  "releaseDir": "release",
  "currentDir": "current",
  "backup": {"directory": "backup", "keep": 3},
  "sync": {"preserve": ["local/", ".env"]}
}`
	localPath := filepath.Join(root, ConfigDirName, ConfigFileName)
	if err := os.WriteFile(localPath, []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Source.DefaultUser != "globaluser" {
		t.Fatalf("default user = %q", cfg.Source.DefaultUser)
	}
	if cfg.KeepReleases != 9 {
		t.Fatalf("keep releases = %d", cfg.KeepReleases)
	}
	wantPreserve := []string{"global/", ".env", "local/", ".gitignore"}
	if !reflect.DeepEqual(cfg.Preserve, wantPreserve) {
		t.Fatalf("preserve = %#v want %#v", cfg.Preserve, wantPreserve)
	}
	if cfg.GlobalConfigFile != filepath.Join(globalDir, ConfigFileName) {
		t.Fatalf("global config = %q", cfg.GlobalConfigFile)
	}
	if cfg.ConfigFile != localPath {
		t.Fatalf("local config = %q", cfg.ConfigFile)
	}
}

func TestLoadUsesGlobalPreserveWhenLocalOmitsSync(t *testing.T) {
	root := t.TempDir()
	globalDir := t.TempDir()
	t.Setenv(globalConfigDirEnv, globalDir)
	if err := os.MkdirAll(filepath.Join(root, ConfigDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, ConfigFileName), []byte(`{"sync":{"preserve":["global-cache/"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fc := defaultFile("demo")
	fc.Sync = nil
	if err := writeConfigFile(filepath.Join(root, ConfigDirName, ConfigFileName), fc); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"global-cache/", ".gitignore"}
	if !reflect.DeepEqual(cfg.Preserve, want) {
		t.Fatalf("preserve = %#v want %#v", cfg.Preserve, want)
	}
}

func TestLoadInheritsAndOverridesKeepRsyncOnSetupError(t *testing.T) {
	root := t.TempDir()
	globalDir := t.TempDir()
	t.Setenv(globalConfigDirEnv, globalDir)
	if err := os.MkdirAll(filepath.Join(root, ConfigDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, ConfigFileName), []byte(`{"setup":{"keepRsyncOnError":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fc := defaultFile("demo")
	fc.Source.Folder = t.TempDir()
	if err := writeConfigFile(filepath.Join(root, ConfigDirName, ConfigFileName), fc); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.KeepRsyncOnSetupError {
		t.Fatal("expected global setup.keepRsyncOnError=true to be inherited")
	}
	value := false
	fc.Setup.KeepRsyncOnError = &value
	if err := writeConfigFile(filepath.Join(root, ConfigDirName, ConfigFileName), fc); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.KeepRsyncOnSetupError {
		t.Fatal("expected explicit local false to override global true")
	}
}

func TestLoadAcceptsLegacyKeepRsyncOnErrorLocations(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"top-level", `"keepRsyncOnError":true`, true},
		{"sync-old-name", `"sync":{"keepRsyncOnError":true}`, true},
		{"sync-yaml-name", `"sync":{"keepOnSetupError":"true"}`, true},
		{"setup-yaml-name", `"setup":{"keepOnSetupError":1}`, true},
		{"invalid-legacy-value-is-dropped", `"keepRsyncOnError":"invalid"`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			global := t.TempDir()
			t.Setenv(globalConfigDirEnv, global)
			if err := os.MkdirAll(filepath.Join(root, ConfigDirName), 0o755); err != nil {
				t.Fatal(err)
			}
			body := `{"schemaVersion":9,"projectName":"demo","source":{"type":"download","folder":".` + `"},"releaseDir":"release","currentDir":"current","backup":{"directory":"backup","keep":3},"retention":{"releases":5},"security":{"maxArchiveBytes":2147483648,"maxUncompressedBytes":8589934592,"maxFileBytes":2147483648,"maxEntries":100000,"maxCompressionRatio":200},` + tt.body + `}`
			if err := os.WriteFile(filepath.Join(root, ConfigDirName, ConfigFileName), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(root, "")
			if err != nil {
				t.Fatalf("Load failed for legacy alias: %v", err)
			}
			if cfg.KeepRsyncOnSetupError != tt.want {
				t.Fatalf("KeepRsyncOnSetupError = %v, want %v", cfg.KeepRsyncOnSetupError, tt.want)
			}
		})
	}
}

func TestCanonicalSetupKeepRsyncOnErrorWinsOverLegacyAlias(t *testing.T) {
	root := t.TempDir()
	global := t.TempDir()
	t.Setenv(globalConfigDirEnv, global)
	if err := os.MkdirAll(filepath.Join(root, ConfigDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"schemaVersion":9,"projectName":"demo","source":{"type":"download","folder":"."},"releaseDir":"release","currentDir":"current","backup":{"directory":"backup","keep":3},"retention":{"releases":5},"security":{"maxArchiveBytes":2147483648,"maxUncompressedBytes":8589934592,"maxFileBytes":2147483648,"maxEntries":100000,"maxCompressionRatio":200},"setup":{"keepRsyncOnError":false},"sync":{"keepOnSetupError":true}}`
	if err := os.WriteFile(filepath.Join(root, ConfigDirName, ConfigFileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.KeepRsyncOnSetupError {
		t.Fatal("canonical setup.keepRsyncOnError=false must win over legacy alias")
	}
}
