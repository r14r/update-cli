package inventory

import (
	"github.com/r14r/update-cli/lib/config"
	"os"
	"path/filepath"
	"testing"
)

func TestFindReleaseIsLocalOnly(t *testing.T) {
	root := t.TempDir()
	relRoot := filepath.Join(root, "release")
	current := filepath.Join(root, "current")
	backup := filepath.Join(root, "backup")
	_ = os.MkdirAll(filepath.Join(relRoot, "1.0.0"), 0o755)
	_ = os.MkdirAll(filepath.Join(relRoot, "2.0.0"), 0o755)
	_ = os.MkdirAll(current, 0o755)
	for _, x := range []struct{ p, v string }{{filepath.Join(current, ".release-version"), "2.0.0"}, {filepath.Join(relRoot, "1.0.0", ".release-version"), "1.0.0"}, {filepath.Join(relRoot, "1.0.0", ".release-project"), "demo"}, {filepath.Join(relRoot, "2.0.0", ".release-version"), "2.0.0"}, {filepath.Join(relRoot, "2.0.0", ".release-project"), "demo"}} {
		_ = os.WriteFile(x.p, []byte(x.v+"\n"), 0o644)
	}
	c := config.Config{ProjectName: "demo", ReleaseRoot: relRoot, CurrentDir: current, BackupRoot: backup, Source: config.SourceConfig{Type: "url", URL: "https://invalid.invalid/demo-v9.9.9.zip"}}
	r, err := FindRelease(c, "")
	if err != nil {
		t.Fatal(err)
	}
	if r.Version != "1.0.0" {
		t.Fatalf("unexpected rollback release: %s", r.Version)
	}
}
