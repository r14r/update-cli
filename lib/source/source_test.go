package source

import (
	"archive/zip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDownloadSelectsNewestVersion(t *testing.T) {
	folder := t.TempDir()
	for _, name := range []string{"demo-v1.2.0.zip", "demo-v1.10.0.zip"} {
		if err := os.WriteFile(filepath.Join(folder, name), []byte("zip"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	artifact, err := Resolve(context.Background(), Options{Type: Download, ProjectName: "demo", Folder: folder})
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Version.String() != "1.10.0" || filepath.Base(artifact.ArchivePath) != "demo-v1.10.0.zip" {
		t.Fatalf("unexpected artifact: %#v", artifact)
	}
}

func TestResolveURLDownloadsNamedArchive(t *testing.T) {
	payload := zipPayload(t, "demo", "2.3.0")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Disposition", `attachment; filename="demo-v2.3.0.zip"`)
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	artifact, err := Resolve(context.Background(), Options{
		Type: URL, ProjectName: "demo", URL: server.URL + "/latest", WorkDir: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Version.String() != "2.3.0" || artifact.Reference != server.URL+"/latest" {
		t.Fatalf("unexpected artifact: %#v", artifact)
	}
	if _, err := os.Stat(artifact.ArchivePath); err != nil {
		t.Fatalf("downloaded archive missing: %v", err)
	}
}

func TestResolveRepositoryClonesAndRemovesGitMetadata(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repository := t.TempDir()
	runGit(t, repository, "init", "-q")
	runGit(t, repository, "config", "user.email", "test@example.com")
	runGit(t, repository, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repository, "VERSION"), []byte("4.5.6\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "app.txt"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", ".")
	runGit(t, repository, "commit", "-qm", "initial")

	releaseRoot := t.TempDir()
	artifact, err := Resolve(context.Background(), Options{
		Type: Repository, ProjectName: "demo", Repository: repository, ReleaseRoot: releaseRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(artifact.StagingDir)
	if artifact.Version.String() != "4.5.6" {
		t.Fatalf("unexpected version: %#v", artifact)
	}
	if _, err := os.Stat(filepath.Join(artifact.ContentDir, ".git")); !os.IsNotExist(err) {
		t.Fatalf(".git must be removed, got %v", err)
	}
	if !strings.HasPrefix(filepath.Clean(artifact.StagingDir), filepath.Clean(releaseRoot)+string(os.PathSeparator)) {
		t.Fatalf("repository clone must be staged below release root: %s", artifact.StagingDir)
	}
}

func zipPayload(t *testing.T, project, version string) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create(project + "/VERSION")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(version + "\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func runGit(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v: %s", args, err, output)
	}
}
