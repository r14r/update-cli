package buildconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParse(t *testing.T) {
	value, err := Parse([]byte(`{"schemaVersion":1,"defaultDownloadFolder":"$HOME/Downloads","defaultDeploymentPath":"/usr/local/bin","defaultConfigPath":"/usr/local/etc/update-cli"}`))
	if err != nil {
		t.Fatal(err)
	}
	if value.DefaultDeploymentPath != "/usr/local/bin" {
		t.Fatalf("unexpected deployment path: %s", value.DefaultDeploymentPath)
	}
}

func TestParseRejectsUnknownField(t *testing.T) {
	_, err := Parse([]byte(`{"schemaVersion":1,"defaultDownloadFolder":"x","defaultDeploymentPath":"y","defaultConfigPath":"z","unknown":true}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExpandPathExpandsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path, err := ExpandPath("$HOME/Downloads")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "Downloads")
	if path != want {
		t.Fatalf("got %s want %s", path, want)
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatal(err)
	}
}
