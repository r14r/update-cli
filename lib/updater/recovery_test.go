package updater

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/history"
	"github.com/r14r/update-cli/lib/ui"
)

func TestRecoverRestoresInterruptedAtomicCurrentSwap(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, config.ConfigDirName)
	current := filepath.Join(root, "current")
	backup := filepath.Join(root, "current.previous-tx-test")
	txDir := filepath.Join(cfgDir, "transactions", "txn-test")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(backup, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(txDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "value.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "value.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	journal := transactionJournal{
		SchemaVersion:       transactionJournalSchemaVersion,
		ID:                  "txn-test",
		ProjectName:         "demo",
		State:               "running",
		Phase:               "services-start",
		FromVersion:         "1.0.0",
		ToVersion:           "2.0.0",
		CurrentDir:          current,
		BackupCurrent:       backup,
		CurrentExisted:      true,
		Activated:           true,
		StartedAt:           time.Now().Add(-time.Minute),
		UpdatedAt:           time.Now(),
	}
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(txDir, "transaction.json"), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		RootDir:     root,
		ConfigDir:   cfgDir,
		CurrentDir:  current,
		ProjectName: "demo",
		HistoryFile: filepath.Join(cfgDir, "history.jsonl"),
		Docker:      config.DockerConfig{Lifecycle: "disabled"},
	}
	if err := runRecover(context.Background(), ui.New(true), cfg, false); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(current, "value.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old" {
		t.Fatalf("current=%q want old", got)
	}
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatalf("recovery backup still exists: %v", err)
	}
	if _, err := os.Stat(txDir); !os.IsNotExist(err) {
		t.Fatalf("transaction journal still exists: %v", err)
	}

	entries, err := history.List(cfg.HistoryFile, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].TransactionID != "txn-test" || entries[0].Action != "recover" {
		t.Fatalf("unexpected recovery history: %#v", entries)
	}
}

func TestRecoverBeforeActivationLeavesCurrentUntouchedAndRemovesStage(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, config.ConfigDirName)
	current := filepath.Join(root, "current")
	stage := filepath.Join(root, "current.next-tx-pre")
	txDir := filepath.Join(cfgDir, "transactions", "txn-pre")
	for _, dir := range []string{current, stage, txDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(current, "value.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "value.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	journal := transactionJournal{
		SchemaVersion:  transactionJournalSchemaVersion,
		ID:             "txn-pre",
		ProjectName:    "demo",
		State:          "running",
		Phase:          "setup",
		CurrentDir:     current,
		StageCurrent:   stage,
		CurrentExisted: true,
		StartedAt:      time.Now().Add(-time.Minute),
		UpdatedAt:      time.Now(),
	}
	data, _ := json.MarshalIndent(journal, "", "  ")
	if err := os.WriteFile(filepath.Join(txDir, "transaction.json"), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{RootDir: root, ConfigDir: cfgDir, CurrentDir: current, ProjectName: "demo", HistoryFile: filepath.Join(cfgDir, "history.jsonl"), Docker: config.DockerConfig{Lifecycle: "disabled"}}
	if err := runRecover(context.Background(), ui.New(true), cfg, false); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(current, "value.txt"))
	if string(got) != "old" {
		t.Fatalf("pre-activation recovery changed current: %q", got)
	}
	if _, err := os.Stat(stage); !os.IsNotExist(err) {
		t.Fatalf("staging directory was not removed: %v", err)
	}
}
