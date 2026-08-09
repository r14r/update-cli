package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type lockMetadata struct {
	PID       int       `json:"pid"`
	Hostname  string    `json:"hostname"`
	StartedAt time.Time `json:"startedAt"`
	Command   string    `json:"command,omitempty"`
}
type Lock struct{ path string }

func AcquireLock(path, command string) (*Lock, error) {
	host, _ := os.Hostname()
	for attempt := 0; attempt < 2; attempt++ {
		if err := os.Mkdir(path, 0o700); err == nil {
			m := lockMetadata{PID: os.Getpid(), Hostname: host, StartedAt: time.Now(), Command: command}
			b, _ := json.MarshalIndent(m, "", "  ")
			_ = os.WriteFile(filepath.Join(path, "lock.json"), append(b, '\n'), 0o600)
			return &Lock{path: path}, nil
		} else if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("Update-Sperre kann nicht erstellt werden: %w", err)
		}
		stale, detail := IsStaleLock(path)
		if stale {
			if err := os.RemoveAll(path); err != nil {
				return nil, fmt.Errorf("veraltete Update-Sperre kann nicht entfernt werden: %w", err)
			}
			continue
		}
		return nil, fmt.Errorf("ein anderes Update läuft bereits: %s (%s)", path, detail)
	}
	return nil, fmt.Errorf("Update-Sperre konnte nicht übernommen werden: %s", path)
}
func IsStaleLock(path string) (bool, string) {
	b, err := os.ReadFile(filepath.Join(path, "lock.json"))
	if err != nil {
		return false, "Lock-Metadaten fehlen"
	}
	var m lockMetadata
	if json.Unmarshal(b, &m) != nil || m.PID <= 0 {
		return false, "Lock-Metadaten ungültig"
	}
	host, _ := os.Hostname()
	if m.Hostname != "" && m.Hostname != host {
		return false, fmt.Sprintf("Host %s, PID %d", m.Hostname, m.PID)
	}
	err = syscall.Kill(m.PID, 0)
	if errors.Is(err, syscall.ESRCH) {
		return true, fmt.Sprintf("stale PID %d", m.PID)
	}
	return false, fmt.Sprintf("PID %d seit %s", m.PID, m.StartedAt.Format(time.RFC3339))
}
func UnlockStale(path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	stale, detail := IsStaleLock(path)
	if !stale {
		return fmt.Errorf("Sperre ist nicht als veraltet erkennbar: %s", detail)
	}
	return os.RemoveAll(path)
}
func (l *Lock) Release() {
	if l != nil && l.path != "" {
		_ = os.RemoveAll(l.path)
	}
}
