package projectsetup

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r14r/update-cli/lib/ui"
)

func TestManifestV2FilesystemAndEnvironmentOperations(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `schemaVersion: 2
variables:
  base: work
  envTarget: "{{ env.UPDATE_CLI_TEST_TARGET | fallback.txt }}"
workflows:
  setup:
    tasks: [files]
tasks:
  files:
    steps:
      - name: Make folders
        mkdir:
          paths: ["{{ base }}/a", "{{ base }}/b"]
      - name: Copy
        copy:
          source: source.txt
          target: "{{ base }}/a/copied.txt"
      - name: Move
        move:
          source: "{{ base }}/a/copied.txt"
          target: "{{ base }}/b/moved.txt"
      - name: Write env fallback
        write:
          path: "{{ envTarget }}"
          content: fallback
      - name: Touch
        touch:
          path: "{{ base }}/touch.txt"
      - name: Mode
        chmod:
          path: "{{ base }}/b/moved.txt"
          mode: "0600"
      - name: Assert file
        assert:
          fileExists: "{{ base }}/b/moved.txt"
      - name: Remove touch
        remove:
          path: "{{ base }}/touch.txt"
      - name: Conditional write
        write:
          path: condition.txt
          content: yes
        when:
          all:
            - directoryExists: "{{ base }}"
            - not:
                fileExists: missing.txt
`
	path := filepath.Join(root, "setup.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RunStandalone(context.Background(), path, ui.New(true)); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "work/b/moved.txt")); err != nil || string(got) != "source" {
		t.Fatalf("moved file: %q err=%v", got, err)
	}
	if info, err := os.Stat(filepath.Join(root, "work/b/moved.txt")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%v err=%v", info, err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "fallback.txt")); err != nil || string(got) != "fallback" {
		t.Fatalf("fallback: %q err=%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(root, "work/touch.txt")); !os.IsNotExist(err) {
		t.Fatalf("touch should be removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "condition.txt")); err != nil {
		t.Fatalf("conditional step did not run: %v", err)
	}
}

func TestManifestV2DownloadHTTPCheckAndExtract(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "server.zip")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	entry, err := zw.Create("inside.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("inside")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	zipBytes, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(zipBytes)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusNoContent)
		case "/archive.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	manifest := `schemaVersion: 2
workflows:
  setup:
    tasks: [network]
tasks:
  network:
    steps:
      - name: HTTP health
        httpCheck:
          url: ` + server.URL + `/health
          status: 204
      - name: Download
        download:
          url: ` + server.URL + `/archive.zip
          destination: tmp/archive.zip
          sha256: ` + hex.EncodeToString(sum[:]) + `
      - name: Extract
        extract:
          archive: tmp/archive.zip
          destination: extracted
      - name: Verify
        assert:
          fileExists: extracted/inside.txt
`
	path := filepath.Join(root, "setup.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RunStandalone(context.Background(), path, ui.New(true)); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "extracted/inside.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != "inside" {
		t.Fatalf("unexpected extracted data %q", got)
	}
}

func TestManifestV2RemoveRefusesProjectRoot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "setup.yaml")
	data := `schemaVersion: 2
workflows:
  setup:
    tasks: [clean]
tasks:
  clean:
    steps:
      - remove:
          path: .
          recursive: true
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := RunStandalone(context.Background(), path, ui.New(true))
	if err == nil || !strings.Contains(err.Error(), "Projektwurzelordner") {
		t.Fatalf("expected root-removal rejection, got %v", err)
	}
}
