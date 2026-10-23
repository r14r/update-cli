package updater

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/r14r/update-cli/src/lib/tools"
)

const transactionJournalSchemaVersion = 1

type transactionJournal struct {
	SchemaVersion       int       `json:"schemaVersion"`
	ID                  string    `json:"id"`
	ProjectName         string    `json:"projectName"`
	State               string    `json:"state"`
	Phase               string    `json:"phase"`
	FromVersion         string    `json:"fromVersion,omitempty"`
	ToVersion           string    `json:"toVersion,omitempty"`
	CurrentDir          string    `json:"currentDir"`
	StageCurrent        string    `json:"stageCurrent,omitempty"`
	BackupCurrent       string    `json:"backupCurrent,omitempty"`
	CurrentExisted      bool      `json:"currentExisted"`
	ServicesWereRunning bool      `json:"servicesWereRunning"`
	Activated           bool      `json:"activated"`
	StartedAt           time.Time `json:"startedAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type pendingTransaction struct {
	Dir     string
	Journal transactionJournal
}

func (t *transaction) writeJournal() error {
	if t == nil || t.snapshotRoot == "" {
		return nil
	}
	now := time.Now()
	if t.journal.StartedAt.IsZero() {
		t.journal.StartedAt = now
	}
	t.journal.UpdatedAt = now
	data, err := json.MarshalIndent(t.journal, "", "  ")
	if err != nil {
		return err
	}
	return tools.WriteFileAtomic(filepath.Join(t.snapshotRoot, "transaction.json"), append(data, '\n'), 0o600)
}

func (t *transaction) setPhase(phase string) error {
	if t == nil {
		return nil
	}
	t.journal.Phase = strings.TrimSpace(phase)
	return t.writeJournal()
}

func (t *transaction) setTargetVersion(version string) error {
	if t == nil {
		return nil
	}
	t.journal.ToVersion = strings.TrimSpace(version)
	return t.writeJournal()
}

func (t *transaction) setCurrentSwap(stage, backup string) error {
	if t == nil {
		return nil
	}
	t.journal.StageCurrent = stage
	t.journal.BackupCurrent = backup
	t.journal.Phase = "activate-current"
	return t.writeJournal()
}

func (t *transaction) markActivated() error {
	if t == nil {
		return nil
	}
	t.journal.Activated = true
	t.journal.Phase = "current-activated"
	return t.writeJournal()
}

func loadPendingTransactions(configDir string) ([]pendingTransaction, error) {
	root := filepath.Join(configDir, "transactions")
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return []pendingTransaction{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []pendingTransaction{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		data, err := os.ReadFile(filepath.Join(dir, "transaction.json"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var journal transactionJournal
		if err := json.Unmarshal(data, &journal); err != nil {
			return nil, fmt.Errorf("Transaktionsjournal ist ungültig (%s): %w", dir, err)
		}
		if journal.SchemaVersion != transactionJournalSchemaVersion {
			return nil, fmt.Errorf("unbekannte Transaktionsjournal-Version %d in %s", journal.SchemaVersion, dir)
		}
		if journal.State == "committed" || journal.State == "recovered" {
			continue
		}
		out = append(out, pendingTransaction{Dir: dir, Journal: journal})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Journal.StartedAt.Before(out[j].Journal.StartedAt)
	})
	return out, nil
}
