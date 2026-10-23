package updater

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/r14r/update-cli/src/lib/config"
	"github.com/r14r/update-cli/src/lib/history"
	"github.com/r14r/update-cli/src/lib/projectdocker"
	"github.com/r14r/update-cli/src/lib/tools"
	"github.com/r14r/update-cli/src/lib/ui"
)

type recoveryItem struct {
	TransactionID string `json:"transactionId"`
	Phase         string `json:"phase"`
	FromVersion   string `json:"fromVersion,omitempty"`
	ToVersion     string `json:"toVersion,omitempty"`
	Restored      bool   `json:"restored"`
	Message       string `json:"message,omitempty"`
}

type recoveryResult struct {
	Recovered []recoveryItem `json:"recovered"`
}

func runRecover(ctx context.Context, console *ui.Console, cfg config.Config, jsonOut bool) error {
	lock, err := tools.AcquireLock(filepath.Join(cfg.RootDir, ".release-update.lock"), "recover")
	if err != nil {
		return err
	}
	defer lock.Release()

	pending, err := loadPendingTransactions(cfg.ConfigDir)
	if err != nil {
		return err
	}
	result := recoveryResult{Recovered: []recoveryItem{}}
	for _, pendingTx := range pending {
		item, err := recoverPendingTransaction(ctx, console, cfg, pendingTx)
		if err != nil {
			return err
		}
		result.Recovered = append(result.Recovered, item)
	}
	if jsonOut {
		return writeJSON(result)
	}
	if len(result.Recovered) == 0 {
		console.Success("Keine unvollständige Transaktion gefunden")
		return nil
	}
	for _, item := range result.Recovered {
		console.Success(fmt.Sprintf("Transaktion %s aus Phase %s wiederhergestellt", item.TransactionID, item.Phase))
	}
	return nil
}

func recoverPendingTransaction(ctx context.Context, console *ui.Console, cfg config.Config, pending pendingTransaction) (recoveryItem, error) {
	j := pending.Journal
	if strings.TrimSpace(j.ID) == "" {
		return recoveryItem{}, fmt.Errorf("Transaktionsjournal ohne ID: %s", pending.Dir)
	}
	if j.ProjectName != "" && j.ProjectName != cfg.ProjectName {
		return recoveryItem{}, fmt.Errorf("Transaktion %s gehört zu Projekt %q statt %q", j.ID, j.ProjectName, cfg.ProjectName)
	}
	if filepath.Clean(j.CurrentDir) != filepath.Clean(cfg.CurrentDir) {
		return recoveryItem{}, fmt.Errorf("Transaktion %s verweist auf unerwartetes current-Verzeichnis %s", j.ID, j.CurrentDir)
	}
	if stage := strings.TrimSpace(j.StageCurrent); stage != "" && !tools.Within(cfg.RootDir, stage) {
		return recoveryItem{}, fmt.Errorf("Transaktion %s enthält unsicheren Staging-Pfad %s", j.ID, stage)
	}
	if backup := strings.TrimSpace(j.BackupCurrent); backup != "" && !tools.Within(cfg.RootDir, backup) {
		return recoveryItem{}, fmt.Errorf("Transaktion %s enthält unsicheren Recovery-Pfad %s", j.ID, backup)
	}

	recoverCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	if j.ServicesWereRunning {
		if detected, err := projectdocker.Detect(cfg.CurrentDir); err == nil && detected.Detected {
			if running, err := projectdocker.Running(recoverCtx, cfg.CurrentDir); err == nil && running {
				_, _ = projectdocker.Stop(recoverCtx, cfg.CurrentDir)
			}
		}
	}

	restored := false
	backup := strings.TrimSpace(j.BackupCurrent)
	if backup != "" {
		if _, err := os.Lstat(backup); err == nil {
			if err := tools.RemoveTree(cfg.CurrentDir); err != nil {
				return recoveryItem{}, fmt.Errorf("aktueller unvollständiger Zustand kann nicht entfernt werden: %w", err)
			}
			if err := os.Rename(backup, cfg.CurrentDir); err != nil {
				return recoveryItem{}, fmt.Errorf("vorheriger current-Zustand kann nicht wiederhergestellt werden: %w", err)
			}
			restored = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return recoveryItem{}, err
		}
	}

	if !restored && !j.CurrentExisted {
		stage := strings.TrimSpace(j.StageCurrent)
		activationMayHaveCompleted := j.Activated
		if !activationMayHaveCompleted && j.Phase == "activate-current" && stage != "" {
			if _, stageErr := os.Lstat(stage); errors.Is(stageErr, os.ErrNotExist) {
				if _, currentErr := os.Lstat(cfg.CurrentDir); currentErr == nil {
					activationMayHaveCompleted = true
				}
			}
		}
		if activationMayHaveCompleted {
			if err := tools.RemoveTree(cfg.CurrentDir); err != nil {
				return recoveryItem{}, fmt.Errorf("unvollständige Erstinstallation kann nicht entfernt werden: %w", err)
			}
			restored = true
		}
	}

	if stage := strings.TrimSpace(j.StageCurrent); stage != "" {
		if err := tools.RemoveTree(stage); err != nil {
			return recoveryItem{}, fmt.Errorf("current-Staging kann nicht entfernt werden: %w", err)
		}
	}

	if j.ServicesWereRunning {
		if _, err := projectdocker.Start(recoverCtx, cfg.CurrentDir); err != nil {
			return recoveryItem{}, fmt.Errorf("vorheriger Docker-Compose-Stack kann nach Recovery nicht gestartet werden: %w", err)
		}
	}

	entry := history.Entry{
		TransactionID: j.ID,
		Action:        "recover",
		Phase:         j.Phase,
		ProjectName:   cfg.ProjectName,
		FromVersion:   j.ToVersion,
		ToVersion:     j.FromVersion,
		Status:        "success",
		Message:       "unvollständige Update-Transaktion wiederhergestellt",
	}
	appendSuccessHistory(console, cfg, entry)

	j.State = "recovered"
	j.Phase = "recovered"
	j.UpdatedAt = time.Now()
	dataTx := &transaction{snapshotRoot: pending.Dir, journal: j}
	_ = dataTx.writeJournal()
	if err := tools.RemoveTree(pending.Dir); err != nil {
		console.Warn("Recovery erfolgreich, Transaktionsjournal konnte aber nicht entfernt werden: " + err.Error())
	}

	return recoveryItem{
		TransactionID: j.ID,
		Phase:         pending.Journal.Phase,
		FromVersion:   j.ToVersion,
		ToVersion:     j.FromVersion,
		Restored:      restored,
		Message:       "recovered",
	}, nil
}
