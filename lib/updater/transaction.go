package updater

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/projectdocker"
	rsyncutil "github.com/r14r/update-cli/lib/rsync"
	"github.com/r14r/update-cli/lib/tools"
	"github.com/r14r/update-cli/lib/ui"
)

type transaction struct {
	cfg                 config.Config
	console             *ui.Console
	snapshotRoot        string
	snapshotCurrent     string
	currentExisted      bool
	servicesWereRunning bool
	dockerManaged       bool
	dockerSkipReason    string
	servicesStopped     bool
	committed           bool
}

func beginTransaction(ctx context.Context, cfg config.Config, console *ui.Console) (*transaction, error) {
	t := &transaction{cfg: cfg, console: console}
	if cfg.Docker.Lifecycle == "disabled" {
		t.dockerSkipReason = "Docker-Lifecycle deaktiviert"
	}
	if info, err := os.Stat(cfg.CurrentDir); err == nil {
		if !info.IsDir() {
			return nil, fmt.Errorf("Current-Pfad ist kein Ordner: %s", cfg.CurrentDir)
		}
		t.currentExisted = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	transactionsRoot := filepath.Join(cfg.ConfigDir, "transactions")
	if err := tools.EnsureDir(transactionsRoot); err != nil {
		return nil, fmt.Errorf("Transaktionsordner kann nicht vorbereitet werden: %w", err)
	}
	workspace, err := os.MkdirTemp(transactionsRoot, "txn-")
	if err != nil {
		return nil, fmt.Errorf("Transaktions-Snapshot kann nicht vorbereitet werden: %w", err)
	}
	t.snapshotRoot = workspace
	t.snapshotCurrent = filepath.Join(t.snapshotRoot, "current")

	if t.currentExisted && cfg.Docker.Lifecycle != "disabled" {
		d, detectErr := projectdocker.Detect(cfg.CurrentDir)
		if detectErr != nil {
			return nil, t.abortBegin(detectErr)
		}
		if !d.Detected {
			t.dockerSkipReason = "kein Compose-Stack erkannt"
		}
		if d.Detected {
			running, statusErr := projectdocker.Running(ctx, cfg.CurrentDir)
			if statusErr != nil {
				if cfg.Docker.Lifecycle == "required" {
					return nil, t.abortBegin(statusErr)
				}
				t.dockerSkipReason = "Docker-Lifecycle für dieses Update übersprungen"
				console.Warn("Docker Compose Status nicht verfügbar; Lifecycle wird für dieses Update übersprungen: " + firstLine(statusErr.Error()))
			} else {
				t.dockerManaged = true
				t.servicesWereRunning = running
			}
		}
		if t.servicesWereRunning {
			if _, err := projectdocker.Stop(ctx, cfg.CurrentDir); err != nil {
				return nil, t.abortBegin(err)
			}
			t.servicesStopped = true
		}
	}

	if t.currentExisted {
		log := filepath.Join(t.snapshotRoot, "snapshot.log")
		if _, err := rsyncutil.TransactionSnapshot(ctx, cfg.CurrentDir, t.snapshotCurrent, log); err != nil {
			return nil, t.abortBegin(fmt.Errorf("Transaktions-Snapshot fehlgeschlagen: %w", err))
		}
	}
	return t, nil
}

func (t *transaction) abortBegin(cause error) error {
	if t == nil {
		return cause
	}
	if t.servicesWereRunning && t.servicesStopped {
		recoverCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		_, restartErr := projectdocker.Start(recoverCtx, t.cfg.CurrentDir)
		cancel()
		if restartErr != nil {
			cause = fmt.Errorf("%w; Docker-Neustart nach abgebrochenem Transaktionsstart fehlgeschlagen: %v", cause, restartErr)
		}
	}
	if t.snapshotRoot != "" {
		if cleanupErr := tools.RemoveTree(t.snapshotRoot); cleanupErr != nil {
			cause = fmt.Errorf("%w; temporärer Transaktionsordner konnte nicht entfernt werden: %v", cause, cleanupErr)
		}
	}
	return cause
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func (t *transaction) commit() error {
	if t == nil {
		return nil
	}
	t.committed = true
	if t.snapshotRoot != "" {
		return tools.RemoveTree(t.snapshotRoot)
	}
	return nil
}
func (t *transaction) startPreviousServiceState(ctx context.Context) error {
	if !t.servicesWereRunning {
		return nil
	}
	_, err := projectdocker.Start(ctx, t.cfg.CurrentDir)
	if err == nil {
	}
	return err
}
func (t *transaction) recover(cause error) error {
	if t == nil || t.committed {
		return cause
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	var recoveryErr error
	// Stop any stack started by setup/new current before restoring files, but only
	// when Docker lifecycle management was confirmed usable for this transaction.
	if t.dockerManaged {
		if running, err := projectdocker.Running(ctx, t.cfg.CurrentDir); err == nil && running {
			_, _ = projectdocker.Stop(ctx, t.cfg.CurrentDir)
		}
	}
	if t.currentExisted {
		if t.snapshotCurrent != "" {
			t.console.Warn("Update fehlgeschlagen; vorherigen current-Zustand wiederherstellen")
			log := filepath.Join(t.snapshotRoot, "restore.log")
			if _, err := rsyncutil.RestoreExact(ctx, t.snapshotCurrent, t.cfg.CurrentDir, log); err != nil {
				recoveryErr = fmt.Errorf("current konnte nicht wiederhergestellt werden: %w", err)
			} else {
				t.console.Success("Vorheriger current-Zustand wiederhergestellt")
			}
		}
	} else {
		if err := tools.RemoveTree(t.cfg.CurrentDir); err != nil {
			recoveryErr = fmt.Errorf("unvollständige Erstinstallation konnte nicht entfernt werden: %w", err)
		}
	}
	if t.servicesWereRunning {
		if _, err := projectdocker.Start(ctx, t.cfg.CurrentDir); err != nil {
			if recoveryErr != nil {
				recoveryErr = fmt.Errorf("%v; Docker-Neustart fehlgeschlagen: %w", recoveryErr, err)
			} else {
				recoveryErr = fmt.Errorf("Docker-Neustart nach Recovery fehlgeschlagen: %w", err)
			}
		} else {
			t.console.Success("Vorheriger Docker-Compose-Stack wieder gestartet")
		}
	}
	_ = tools.RemoveTree(t.snapshotRoot)
	if recoveryErr != nil {
		return fmt.Errorf("%w; Recovery-Fehler: %v", cause, recoveryErr)
	}
	return cause
}
