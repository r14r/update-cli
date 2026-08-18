package updater

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/history"
	"github.com/r14r/update-cli/lib/ui"
)

func TestAppendSuccessHistoryWarnsInsteadOfFailingCommittedOperation(t *testing.T) {
	root := t.TempDir()
	badHistoryPath := filepath.Join(root, "history-as-directory")
	if err := os.MkdirAll(badHistoryPath, 0o755); err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	appendSuccessHistory(ui.New(true), config.Config{HistoryFile: badHistoryPath}, history.Entry{Action: "update", ProjectName: "demo", Status: "success"})
	_ = w.Close()
	os.Stderr = old
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "Vorgang erfolgreich") || !strings.Contains(string(b), "Historie") {
		t.Fatalf("expected non-fatal history warning, got %q", b)
	}
}
