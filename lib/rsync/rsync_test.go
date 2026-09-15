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

func TestReleaseIncludesEnvFiles(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "release")
	log := filepath.Join(t.TempDir(), "release.log")
	for name, content := range map[string]string{
		".env":         "SECRET=value\n",
		".env.example": "SECRET=example\n",
		"app.txt":      "app\n",
	} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Release(context.Background(), src, dst, log); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".env", ".env.example", "app.txt"} {
		if _, err := os.Stat(filepath.Join(dst, name)); err != nil {
			t.Fatalf("release is missing %s: %v", name, err)
		}
	}
}

func TestCurrentSeedsMissingPreservedPathsThenPreservesThem(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	log := filepath.Join(t.TempDir(), "current.log")
	must := func(p, content string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must(filepath.Join(src, ".env"), "release-secret")
	must(filepath.Join(src, ".env.example"), "release-example")
	must(filepath.Join(src, "data", "seed.txt"), "release-data")

	r, err := Current(context.Background(), src, dst, log, false, []string{".env", ".env.*", "data/"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Changes == 0 {
		t.Fatal("expected seeded preserved paths to be reported as changes")
	}
	for name, want := range map[string]string{
		".env":          "release-secret",
		".env.example":  "release-example",
		"data/seed.txt": "release-data",
	} {
		b, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("seeded path %s missing: %v", name, err)
		}
		if string(b) != want {
			t.Fatalf("seeded path %s = %q, want %q", name, b, want)
		}
	}

	must(filepath.Join(dst, ".env"), "local-secret")
	must(filepath.Join(dst, ".env.example"), "local-example")
	must(filepath.Join(dst, "data", "seed.txt"), "local-data")
	must(filepath.Join(src, ".env"), "new-release-secret")
	must(filepath.Join(src, ".env.example"), "new-release-example")
	must(filepath.Join(src, "data", "seed.txt"), "new-release-data")

	if _, err := Current(context.Background(), src, dst, log, false, []string{".env", ".env.*", "data/"}); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		".env":          "local-secret",
		".env.example":  "local-example",
		"data/seed.txt": "local-data",
	} {
		b, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != want {
			t.Fatalf("preserved path %s was overwritten: %q", name, b)
		}
	}
}

func TestCurrentDryRunReportsMissingPreservedPathsWithoutCreatingThem(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	log := filepath.Join(t.TempDir(), "dry.log")
	if err := os.WriteFile(filepath.Join(src, ".env"), []byte("release"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Current(context.Background(), src, dst, log, true, []string{".env"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Changes == 0 {
		t.Fatal("dry-run did not report missing preserved file")
	}
	if _, err := os.Stat(filepath.Join(dst, ".env")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created .env: %v", err)
	}
}

func TestReleaseExcludesRuntimeStateDirectories(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "release")
	log := filepath.Join(t.TempDir(), "release.log")
	for _, name := range []string{".update-cli/config.json", "app.txt"} {
		path := filepath.Join(src, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Release(context.Background(), src, dst, log); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "app.txt")); err != nil {
		t.Fatalf("normal release file missing: %v", err)
	}
	for _, name := range []string{".update-cli"} {
		if _, err := os.Stat(filepath.Join(dst, name)); !os.IsNotExist(err) {
			t.Fatalf("reserved runtime directory %s leaked into release: %v", name, err)
		}
	}
}

func TestCurrentRemovesNestedRuntimeStateAndNeverCopiesIt(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	log := filepath.Join(t.TempDir(), "current.log")
	for _, base := range []string{src, dst} {
		path := filepath.Join(base, ".update-cli", "config.json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(src, "app.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Current(context.Background(), src, dst, log, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".update-cli")); !os.IsNotExist(err) {
		t.Fatalf("nested .update-cli still exists after current sync: %v", err)
	}
	foundDelete := false
	for _, item := range r.Items {
		if item.Kind == ChangeDeleted && item.Path == ".update-cli/" {
			foundDelete = true
		}
	}
	if !foundDelete {
		t.Fatalf("nested runtime cleanup was not reported: %#v", r.Items)
	}
}

func TestTransactionSnapshotAndRestoreExactExcludeRuntimeState(t *testing.T) {
	source := t.TempDir()
	snapshot := filepath.Join(t.TempDir(), "snapshot")
	restored := t.TempDir()
	logDir := t.TempDir()
	for _, rel := range []string{"app.txt", ".update-cli/config.json"} {
		path := filepath.Join(source, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(rel), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := TransactionSnapshot(context.Background(), source, snapshot, filepath.Join(logDir, "snapshot.log")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".update-cli"} {
		if _, err := os.Stat(filepath.Join(snapshot, name)); !os.IsNotExist(err) {
			t.Fatalf("runtime state %s leaked into transaction snapshot: %v", name, err)
		}
	}
	if err := os.MkdirAll(filepath.Join(restored, ".update-cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(restored, ".update-cli", "config.json"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RestoreExact(context.Background(), snapshot, restored, filepath.Join(logDir, "restore.log")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(restored, ".update-cli")); !os.IsNotExist(err) {
		t.Fatalf("RestoreExact kept nested runtime state: %v", err)
	}
	if _, err := os.Stat(filepath.Join(restored, "app.txt")); err != nil {
		t.Fatalf("RestoreExact lost normal application file: %v", err)
	}
}
