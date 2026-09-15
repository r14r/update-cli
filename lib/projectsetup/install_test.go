package projectsetup

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/r14r/update-cli/lib/ui"
)

func TestRunJustInstallPrefersCurrent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper is Unix-specific")
	}
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "justfile"), []byte("install:\n\t@true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "justfile"), []byte("install:\n\t@true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	bin := t.TempDir()
	just := filepath.Join(bin, "just")
	script := "#!/bin/sh\nprintf '%s\\n' \"$PWD:$1:$UPDATE_CLI_INSTALL_RUNNING\" > \"$UPDATE_CLI_INSTALL_TEST_RESULT\"\n"
	if err := os.WriteFile(just, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	result := filepath.Join(root, "result.txt")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("UPDATE_CLI_INSTALL_TEST_RESULT", result)

	console := ui.New(true)
	console.SetDirect(true)
	if err := RunJustInstall(context.Background(), root, console); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(result)
	if err != nil {
		t.Fatal(err)
	}
	canonicalCurrent, err := filepath.EvalSymlinks(current)
	if err != nil {
		t.Fatal(err)
	}
	want := canonicalCurrent + ":install:1\n"
	if string(data) != want {
		t.Fatalf("result=%q want canonical path %q", data, want)
	}
}

func TestRunJustInstallUsesProjectRootWithoutCurrent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper is Unix-specific")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Justfile"), []byte("install:\n\t@true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	just := filepath.Join(bin, "just")
	if err := os.WriteFile(just, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	console := ui.New(true)
	console.SetDirect(true)
	if err := RunJustInstall(context.Background(), root, console); err != nil {
		t.Fatal(err)
	}
}

func TestRunJustInstallRequiresJustfile(t *testing.T) {
	console := ui.New(true)
	console.SetDirect(true)
	if err := RunJustInstall(context.Background(), t.TempDir(), console); err == nil {
		t.Fatal("expected missing justfile error")
	}
}
