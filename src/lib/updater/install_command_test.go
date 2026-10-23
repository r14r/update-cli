package updater

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallCommandRunsWithoutRuntimeConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper is Unix-specific")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "justfile"), []byte("install:\n\t@true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	result := filepath.Join(root, "installed.txt")
	just := filepath.Join(bin, "just")
	script := "#!/bin/sh\nprintf '%s\\n' \"$PWD:$1\" > \"$UPDATE_CLI_INSTALL_COMMAND_RESULT\"\n"
	if err := os.WriteFile(just, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("UPDATE_CLI_INSTALL_COMMAND_RESULT", result)

	if err := Run(context.Background(), "test", []string{"install", "--root", root, "--no-color"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(result)
	if err != nil {
		t.Fatal(err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	want := canonicalRoot + ":install\n"
	if string(data) != want {
		t.Fatalf("result=%q want canonical path %q", data, want)
	}
}
