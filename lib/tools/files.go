package tools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func EnsureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("Ordner kann nicht erstellt werden (%s): %w", path, err)
	}
	return nil
}
func RemoveTree(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	clean := filepath.Clean(path)
	if clean == string(filepath.Separator) {
		return errors.New("Löschen des Dateisystemwurzelverzeichnisses wird verweigert")
	}
	if h, err := os.UserHomeDir(); err == nil && clean == filepath.Clean(h) {
		return errors.New("Löschen des Benutzerverzeichnisses wird verweigert")
	}
	return os.RemoveAll(clean)
}

type DirectorySwap struct {
	Target    string
	Backup    string
	HadTarget bool
	committed bool
}

func SwapDirectory(stage, target string) (*DirectorySwap, error) {
	if _, err := os.Stat(stage); err != nil {
		return nil, fmt.Errorf("Staging-Ordner fehlt: %w", err)
	}
	backup, err := uniqueSwapBackupPath(target)
	if err != nil {
		return nil, err
	}
	swap := &DirectorySwap{Target: target, Backup: backup}
	if _, err := os.Stat(target); err == nil {
		swap.HadTarget = true
		if err := os.Rename(target, backup); err != nil {
			return nil, fmt.Errorf("bestehendes Release kann nicht verschoben werden: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("Release-Ziel kann nicht geprüft werden: %w", err)
	}
	if err := os.Rename(stage, target); err != nil {
		if swap.HadTarget {
			_ = os.Rename(backup, target)
		}
		return nil, fmt.Errorf("neues Release kann nicht aktiviert werden: %w", err)
	}
	return swap, nil
}

func uniqueSwapBackupPath(target string) (string, error) {
	stamp := time.Now().UnixNano()
	for i := 0; i < 1000; i++ {
		p := fmt.Sprintf("%s.old-%d-%d-%03d", target, os.Getpid(), stamp, i)
		if _, err := os.Lstat(p); errors.Is(err, os.ErrNotExist) {
			return p, nil
		} else if err != nil {
			return "", fmt.Errorf("Release-Swap-Pfad kann nicht geprüft werden: %w", err)
		}
	}
	return "", fmt.Errorf("kein eindeutiger Release-Swap-Pfad für %s verfügbar", target)
}

func (s *DirectorySwap) Commit() error {
	if s == nil || s.committed {
		return nil
	}
	s.committed = true
	if s.HadTarget {
		return RemoveTree(s.Backup)
	}
	return nil
}

func (s *DirectorySwap) Rollback() error {
	if s == nil || s.committed {
		return nil
	}
	if err := RemoveTree(s.Target); err != nil {
		return err
	}
	if s.HadTarget {
		if err := os.Rename(s.Backup, s.Target); err != nil {
			return fmt.Errorf("vorheriges Release kann nicht wiederhergestellt werden: %w", err)
		}
	}
	s.committed = true
	return nil
}

func ReplaceDirectory(stage, target string) error {
	swap, err := SwapDirectory(stage, target)
	if err != nil {
		return err
	}
	return swap.Commit()
}

func WriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".atomic-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func WriteMarker(dir, name, value string) error {
	return WriteFileAtomic(filepath.Join(dir, name), []byte(value+"\n"), 0o644)
}

func Within(root, path string) bool {
	ra, _ := filepath.Abs(root)
	pa, _ := filepath.Abs(path)
	rel, err := filepath.Rel(ra, pa)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
func CanonicalInside(root, path string, rejectFinalSymlink bool) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", fmt.Errorf("Projektwurzel kann nicht kanonisiert werden: %w", err)
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if rejectFinalSymlink {
		if info, err := os.Lstat(pathAbs); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("sicherheitskritischer Pfad darf kein Symlink sein: %s", pathAbs)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	ancestor := pathAbs
	tail := []string{}
	for {
		if _, err := os.Lstat(ancestor); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", fmt.Errorf("kein existierender Elternpfad für %s", pathAbs)
		}
		tail = append([]string{filepath.Base(ancestor)}, tail...)
		ancestor = parent
	}
	realAncestor, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", err
	}
	candidate := realAncestor
	for _, part := range tail {
		candidate = filepath.Join(candidate, part)
	}
	if !Within(rootReal, candidate) {
		return "", fmt.Errorf("Pfad verlässt über Symlinks den Projektordner: %s", path)
	}
	return candidate, nil
}
