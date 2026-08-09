package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUnlockStaleLock(t *testing.T) {
	p := filepath.Join(t.TempDir(), "lock")
	if err := os.Mkdir(p, 0o700); err != nil {
		t.Fatal(err)
	}
	host, _ := os.Hostname()
	b, _ := json.Marshal(lockMetadata{PID: 99999999, Hostname: host, StartedAt: time.Now().Add(-time.Hour)})
	if err := os.WriteFile(filepath.Join(p, "lock.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	stale, _ := IsStaleLock(p)
	if !stale {
		t.Fatal("expected stale lock")
	}
	if err := UnlockStale(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("lock still exists: %v", err)
	}
}
