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

func TestIncompleteRecentLockIsNotStale(t *testing.T) {
	p := filepath.Join(t.TempDir(), "lock")
	if err := os.Mkdir(p, 0o700); err != nil {
		t.Fatal(err)
	}
	stale, detail := IsStaleLock(p)
	if stale {
		t.Fatalf("recent incomplete lock must not be stale: %s", detail)
	}
}

func TestIncompleteOldLockBecomesStale(t *testing.T) {
	p := filepath.Join(t.TempDir(), "lock")
	if err := os.Mkdir(p, 0o700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * incompleteLockGrace)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	stale, detail := IsStaleLock(p)
	if !stale {
		t.Fatalf("old incomplete lock should be stale: %s", detail)
	}
}

func TestAcquireLockWritesMetadataAtomically(t *testing.T) {
	p := filepath.Join(t.TempDir(), "lock")
	lock, err := AcquireLock(p, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	b, err := os.ReadFile(filepath.Join(p, "lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m lockMetadata
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("invalid metadata: %v", err)
	}
	if m.PID != os.Getpid() || m.Command != "test" {
		t.Fatalf("unexpected metadata: %+v", m)
	}
}
