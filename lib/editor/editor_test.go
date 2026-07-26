package editor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenUsesConfiguredEditorAndPassesPath(t *testing.T) {
	root := t.TempDir()
	capture := filepath.Join(root, "capture.txt")
	script := filepath.Join(root, "editor.sh")
	content := "#!/bin/sh\nprintf '%s' \"$1\" > \"$CAPTURE\"\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", script)
	t.Setenv("CAPTURE", capture)

	path := filepath.Join(root, "config file.json")
	used, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if used != script {
		t.Fatalf("unexpected editor: %q", used)
	}
	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != path {
		t.Fatalf("editor received %q, want %q", string(data), path)
	}
}
