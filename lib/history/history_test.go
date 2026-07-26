package history

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAndListNewestFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	old := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	newer := old.Add(time.Hour)
	if err := Append(path, Entry{Timestamp: old, Action: "update", ProjectName: "demo", ToVersion: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	if err := Append(path, Entry{Timestamp: newer, Action: "rollback", ProjectName: "demo", ToVersion: "0.9.0"}); err != nil {
		t.Fatal(err)
	}
	entries, err := List(path, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Action != "rollback" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}
