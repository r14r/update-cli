package rollback

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"release-updater/lib/config"
)

func TestResolvePreviousAndApplyPreservesEnvironment(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{
		ProjectName: "demo",
		DownloadDir: filepath.Join(root, "downloads"),
		CurrentDir:  filepath.Join(root, "current"),
		ReleaseRoot: filepath.Join(root, "release"),
		ConfigDir:   filepath.Join(root, ".updater-cli"),
		BackupRoot:  filepath.Join(root, "backup"),
	}
	if err := os.MkdirAll(cfg.DownloadDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.CurrentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.CurrentDir, ".release-version"), []byte("2.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.CurrentDir, ".env"), []byte("LOCAL=yes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.CurrentDir, "app.txt"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"1.0.0", "2.0.0"} {
		path := filepath.Join(cfg.ReleaseRoot, version)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, value := range map[string]string{
			".release-version": version + "\n",
			".release-project": "demo\n",
			"VERSION":          version + "\n",
			"app.txt":          "v" + string(version[0]) + "\n",
		} {
			if err := os.WriteFile(filepath.Join(path, name), []byte(value), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	release, err := Resolve(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if release.Version != "1.0.0" {
		t.Fatalf("resolved version %s", release.Version)
	}
	result, err := Apply(context.Background(), cfg, release, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.FromVersion != "2.0.0" || result.ToVersion != "1.0.0" {
		t.Fatalf("unexpected result: %#v", result)
	}
	data, err := os.ReadFile(filepath.Join(cfg.CurrentDir, ".env"))
	if err != nil || string(data) != "LOCAL=yes\n" {
		t.Fatalf("environment not preserved: %q, %v", data, err)
	}
	versionData, err := os.ReadFile(filepath.Join(cfg.CurrentDir, ".release-version"))
	if err != nil || string(versionData) != "1.0.0\n" {
		t.Fatalf("rollback marker not applied: %q, %v", versionData, err)
	}
}
