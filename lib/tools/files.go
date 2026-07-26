package tools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func EnsureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("Ordner kann nicht erstellt werden (%s): %w", path, err)
	}
	return nil
}

func RemoveTree(path string) error {
	if path == "" {
		return nil
	}
	clean := filepath.Clean(path)
	if clean == string(filepath.Separator) {
		return errors.New("Löschen des Dateisystemwurzelverzeichnisses wird verweigert")
	}
	if home, err := os.UserHomeDir(); err == nil && clean == filepath.Clean(home) {
		return errors.New("Löschen des Benutzerverzeichnisses wird verweigert")
	}
	return os.RemoveAll(clean)
}

func ReplaceDirectory(stage, target string) error {
	if _, err := os.Stat(stage); err != nil {
		return fmt.Errorf("Staging-Ordner fehlt: %w", err)
	}

	backup := target + fmt.Sprintf(".old-%d", os.Getpid())
	_ = RemoveTree(backup)
	targetExists := false
	if _, err := os.Stat(target); err == nil {
		targetExists = true
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("bestehendes Release kann nicht verschoben werden: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("Release-Ziel kann nicht geprüft werden: %w", err)
	}

	if err := os.Rename(stage, target); err != nil {
		if targetExists {
			_ = os.Rename(backup, target)
		}
		return fmt.Errorf("neues Release kann nicht aktiviert werden: %w", err)
	}

	if targetExists {
		if err := RemoveTree(backup); err != nil {
			return fmt.Errorf("altes Release kann nicht entfernt werden: %w", err)
		}
	}
	return nil
}

func WriteMarker(directory, name, value string) error {
	if err := EnsureDir(directory); err != nil {
		return err
	}
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte(value+"\n"), 0o644); err != nil {
		return fmt.Errorf("Marker kann nicht geschrieben werden (%s): %w", path, err)
	}
	return nil
}
