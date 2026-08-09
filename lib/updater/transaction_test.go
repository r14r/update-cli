package updater

import (
	"context"
	"errors"
	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/ui"
	"os"
	"path/filepath"
	"testing"
)

func TestTransactionRecoveryRestoresExactCurrent(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	cfgDir := filepath.Join(root, ".updater-cli")
	_ = os.MkdirAll(current, 0o755)
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(current, "keep.txt"), []byte("old"), 0o644)
	_ = os.WriteFile(filepath.Join(current, ".env"), []byte("secret"), 0o600)
	c := config.Config{RootDir: root, ConfigDir: cfgDir, CurrentDir: current}
	tx, err := beginTransaction(context.Background(), c, ui.New(true))
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(current, "keep.txt"), []byte("new"), 0o644)
	_ = os.WriteFile(filepath.Join(current, "extra.txt"), []byte("x"), 0o644)
	got := tx.recover(errors.New("boom"))
	if got == nil {
		t.Fatal("expected original error")
	}
	b, _ := os.ReadFile(filepath.Join(current, "keep.txt"))
	if string(b) != "old" {
		t.Fatalf("old content not restored: %q", b)
	}
	b, _ = os.ReadFile(filepath.Join(current, ".env"))
	if string(b) != "secret" {
		t.Fatalf("env not restored: %q", b)
	}
	if _, err := os.Stat(filepath.Join(current, "extra.txt")); !os.IsNotExist(err) {
		t.Fatalf("extra file not removed: %v", err)
	}
}
