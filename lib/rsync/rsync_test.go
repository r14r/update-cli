package rsync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCurrentPreservesPersistentPaths(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	log := filepath.Join(t.TempDir(), "r.log")
	must := func(p, s string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must(filepath.Join(src, "app.txt"), "new")
	must(filepath.Join(src, "data", "db.txt"), "release-data")
	must(filepath.Join(src, ".env"), "release-secret")
	must(filepath.Join(src, ".gitignore"), "release-ignore")
	must(filepath.Join(dst, "app.txt"), "old")
	must(filepath.Join(dst, "data", "db.txt"), "user-data")
	must(filepath.Join(dst, ".env"), "user-secret")
	must(filepath.Join(dst, ".gitignore"), "user-ignore")
	_, err := Current(context.Background(), src, dst, log, false, []string{"data/", ".env"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dst, "app.txt"))
	if string(b) != "new" {
		t.Fatalf("app not updated: %q", b)
	}
	b, _ = os.ReadFile(filepath.Join(dst, "data", "db.txt"))
	if string(b) != "user-data" {
		t.Fatalf("data overwritten: %q", b)
	}
	b, _ = os.ReadFile(filepath.Join(dst, ".env"))
	if string(b) != "user-secret" {
		t.Fatalf("env overwritten: %q", b)
	}
	b, _ = os.ReadFile(filepath.Join(dst, ".gitignore"))
	if string(b) != "user-ignore" {
		t.Fatalf(".gitignore overwritten: %q", b)
	}
}
func TestSnapshotExcludesSecrets(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "backup")
	log := filepath.Join(t.TempDir(), "r.log")
	for _, name := range []string{".env", ".env.local", "app.txt"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Snapshot(context.Background(), src, dst, log, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".env")); !os.IsNotExist(err) {
		t.Fatalf(".env was backed up: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".env.local")); !os.IsNotExist(err) {
		t.Fatalf(".env.local was backed up: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "app.txt")); err != nil {
		t.Fatal("app.txt missing")
	}
}

func TestMandatoryPreserveAlwaysIncludesGitignore(t *testing.T) {
	got := mandatoryPreserve([]string{"data/", ".env"})
	found := false
	for _, value := range got {
		if value == ".gitignore" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("mandatory preserve paths do not include .gitignore: %#v", got)
	}
}
