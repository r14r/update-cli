package projectsetup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/ui"
)

func TestFailureTailKeepsLastNonEmptyLines(t *testing.T) {
	got := failureTail("one\n\ntwo\nthree\nfour\n", 2)
	want := []string{"three", "four"}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}

func TestTailWriterIsBounded(t *testing.T) {
	w := &tailWriter{max: 5}
	if _, err := w.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("defg")); err != nil {
		t.Fatal(err)
	}
	if got, want := w.String(), "cdefg"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLegacySetupChildDoesNotOwnWaitOrFullscreen(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "setup.sh")
	data := `#!/usr/bin/env bash
set -eu
[ "${SETUP_WAIT:-}" = "0" ] || { echo "SETUP_WAIT not disabled" >&2; exit 41; }
[ "${SETUP_TUI_MODE:-}" = "plain" ] || { echo "SETUP_TUI_MODE not plain" >&2; exit 42; }
[ "${SETUP_DETAILS:-}" = "0" ] || { echo "SETUP_DETAILS not compact" >&2; exit 43; }
printf ok > legacy-result.txt
`
	if err := os.WriteFile(script, []byte(data), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{ProjectName: "legacy-demo", CurrentDir: root}
	res, err := Run(context.Background(), cfg, ui.New(true))
	if err != nil {
		t.Fatal(err)
	}
	if !res.LegacyScriptExecuted {
		t.Fatal("legacy setup.sh was not executed")
	}
	got, err := os.ReadFile(filepath.Join(root, "legacy-result.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "ok" {
		t.Fatalf("unexpected legacy result %q", got)
	}
}
