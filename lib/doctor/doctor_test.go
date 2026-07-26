package doctor

import (
	"archive/zip"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"release-updater/lib/config"
)

func TestDoctorReportsMissingConfiguration(t *testing.T) {
	report := Run(t.TempDir(), "")
	if report.ErrorCount() == 0 {
		t.Fatalf("expected configuration error: %#v", report.Checks)
	}
}

func TestDoctorAcceptsConfiguredProject(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not available in test environment")
	}

	root := t.TempDir()
	downloads := filepath.Join(root, "downloads")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Init(root, config.InitOptions{ProjectName: "demo"}); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, config.ConfigDirName, config.ConfigFileName)
	data := []byte(`{"schemaVersion":1,"projectName":"demo","downloadDir":"` + downloads + `","releaseDir":"release","currentDir":"current"}`)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	archivePath := filepath.Join(downloads, "demo-v1.0.0.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("demo/README.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("demo")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	report := Run(root, "")
	if report.ErrorCount() != 0 {
		t.Fatalf("unexpected doctor errors: %#v", report.Checks)
	}
}
