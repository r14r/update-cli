package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestListIgnoresOnlyIncompleteTrailingRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	good := Entry{Timestamp: time.Now(), Action: "update", ProjectName: "demo", Status: "success"}
	if err := Append(path, good); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"timestamp":"broken"`); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	items, err := List(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ProjectName != "demo" {
		t.Fatalf("unexpected history: %+v", items)
	}
}

func TestListRejectsMalformedCommittedLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	if err := os.WriteFile(path, []byte("{bad json}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := List(path, 0)
	if err == nil || !strings.Contains(err.Error(), "Zeile 1") {
		t.Fatalf("expected committed corruption error, got %v", err)
	}
}
