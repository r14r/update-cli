package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r14r/update-cli/lib/config"
)

func TestFetchURLRejectsOversize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="demo-v1.0.0.zip"`)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(strings.Repeat("x", 1024)))
	}))
	defer srv.Close()
	_, err := Fetch(context.Background(), Options{ProjectName: "demo", Source: config.SourceConfig{Type: "url", URL: srv.URL, Version: "1.0.0"}, WorkDir: t.TempDir(), AllowHTTP: true, MaxArchiveBytes: 100})
	if err == nil || !(strings.Contains(err.Error(), "maximale Downloadgröße") || strings.Contains(err.Error(), "zu groß")) {
		t.Fatalf("expected size error, got %v", err)
	}
}

func TestDiscoverURLFallsBackFromHeadToRangeGet(t *testing.T) {
	methods := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="demo-v1.2.3.zip"`)
		w.Header().Set("Content-Range", "bytes 0-0/12345")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()
	m, err := Discover(context.Background(), Options{ProjectName: "demo", Source: config.SourceConfig{Type: "url", URL: srv.URL}, AllowHTTP: true, MaxArchiveBytes: 20000})
	if err != nil {
		t.Fatal(err)
	}
	if m.VersionText != "1.2.3" || m.Size != 12345 {
		t.Fatalf("unexpected metadata: %#v", m)
	}
	if len(methods) != 2 || methods[0] != "HEAD" || methods[1] != "GET" {
		t.Fatalf("unexpected methods: %#v", methods)
	}
}

func TestPullRepositoryUsesPersistentCheckoutAndPullsNewCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	origin := filepath.Join(t.TempDir(), "origin")
	if err := os.MkdirAll(origin, 0o755); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, origin, "init", "-b", "main")
	gitTestRun(t, origin, "config", "user.name", "Update CLI Test")
	gitTestRun(t, origin, "config", "user.email", "update-cli@example.invalid")
	if err := os.WriteFile(filepath.Join(origin, "VERSION"), []byte("1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(origin, "app.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, origin, "add", ".")
	gitTestRun(t, origin, "commit", "-m", "v1")

	cache := filepath.Join(t.TempDir(), "repository")
	opts := Options{
		ProjectName:        "demo",
		Mode:               config.ModePull,
		Source:             config.SourceConfig{Type: Repository, Repository: origin, Ref: "main"},
		RepositoryCacheDir: cache,
		WorkDir:            t.TempDir(),
	}
	first, err := Fetch(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if first.VersionText != "1.0.0" || first.Commit == "" {
		t.Fatalf("unexpected first artifact: %#v", first)
	}
	firstCommit := first.Commit
	if data, err := os.ReadFile(filepath.Join(first.ContentDir, "app.txt")); err != nil || string(data) != "one\n" {
		t.Fatalf("first repository snapshot invalid: %q, %v", data, err)
	}

	if err := os.WriteFile(filepath.Join(origin, "VERSION"), []byte("1.0.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(origin, "app.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, origin, "add", ".")
	gitTestRun(t, origin, "commit", "-m", "v2")

	meta, err := Discover(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if meta.VersionText != "1.0.1" || meta.Commit == firstCommit {
		t.Fatalf("discover did not see new remote state: %#v", meta)
	}
	cacheHeadBefore := strings.TrimSpace(gitTestOutput(t, cache, "rev-parse", "HEAD"))
	if cacheHeadBefore != firstCommit {
		t.Fatalf("discover unexpectedly changed checkout HEAD: %s != %s", cacheHeadBefore, firstCommit)
	}

	opts.WorkDir = t.TempDir()
	second, err := Fetch(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if second.VersionText != "1.0.1" || second.Commit != meta.Commit {
		t.Fatalf("unexpected pulled artifact: %#v", second)
	}
	cacheHeadAfter := strings.TrimSpace(gitTestOutput(t, cache, "rev-parse", "HEAD"))
	if cacheHeadAfter != second.Commit {
		t.Fatalf("persistent checkout was not pulled: %s != %s", cacheHeadAfter, second.Commit)
	}
	if data, err := os.ReadFile(filepath.Join(second.ContentDir, "app.txt")); err != nil || string(data) != "two\n" {
		t.Fatalf("second repository snapshot invalid: %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(second.ContentDir, ".git")); !os.IsNotExist(err) {
		t.Fatalf("repository snapshot must not contain .git, stat err=%v", err)
	}
}

func gitTestRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func gitTestOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
