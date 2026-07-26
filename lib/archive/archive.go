package archive

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Stats struct {
	Entries           int   `json:"entries"`
	Files             int   `json:"files"`
	Directories       int   `json:"directories"`
	UncompressedBytes int64 `json:"uncompressedBytes"`
}

func Inspect(path string) (Stats, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return Stats{}, fmt.Errorf("ZIP kann nicht geöffnet werden: %w", err)
	}
	defer reader.Close()

	if len(reader.File) == 0 {
		return Stats{}, errors.New("ZIP-Archiv ist leer")
	}

	stats := Stats{Entries: len(reader.File)}
	for _, entry := range reader.File {
		if _, err := safeRelativePath(entry.Name); err != nil {
			return Stats{}, err
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return Stats{}, fmt.Errorf("symbolischer Link im ZIP ist nicht erlaubt: %s", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			stats.Directories++
			continue
		}
		stats.Files++
		stats.UncompressedBytes += int64(entry.UncompressedSize64)

		file, err := entry.Open()
		if err != nil {
			return Stats{}, fmt.Errorf("ZIP-Eintrag kann nicht geöffnet werden (%s): %w", entry.Name, err)
		}
		_, copyErr := io.Copy(io.Discard, file)
		closeErr := file.Close()
		if copyErr != nil {
			return Stats{}, fmt.Errorf("ZIP-Prüfsumme ist fehlerhaft (%s): %w", entry.Name, copyErr)
		}
		if closeErr != nil {
			return Stats{}, fmt.Errorf("ZIP-Eintrag kann nicht geschlossen werden (%s): %w", entry.Name, closeErr)
		}
	}
	return stats, nil
}

func Validate(path string) error {
	_, err := Inspect(path)
	return err
}

func Extract(path, destination string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("ZIP kann nicht geöffnet werden: %w", err)
	}
	defer reader.Close()

	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("Entpackordner kann nicht erstellt werden: %w", err)
	}

	for _, entry := range reader.File {
		relative, err := safeRelativePath(entry.Name)
		if err != nil {
			return err
		}
		if relative == "" || shouldIgnore(relative) {
			continue
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolischer Link im ZIP ist nicht erlaubt: %s", entry.Name)
		}

		target := filepath.Join(destination, filepath.FromSlash(relative))
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, directoryMode(entry.Mode())); err != nil {
				return fmt.Errorf("Ordner kann nicht erstellt werden (%s): %w", target, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("Elternordner kann nicht erstellt werden (%s): %w", target, err)
		}
		if err := extractFile(entry, target); err != nil {
			return err
		}
	}
	return nil
}

func ResolveContentRoot(extractDir string) (string, error) {
	candidate := extractDir
	for range 12 {
		entries, err := os.ReadDir(candidate)
		if err != nil {
			return "", fmt.Errorf("entpackter Inhalt kann nicht gelesen werden: %w", err)
		}

		visible := make([]os.DirEntry, 0, len(entries))
		for _, entry := range entries {
			if entry.Name() == "__MACOSX" || entry.Name() == ".DS_Store" {
				continue
			}
			visible = append(visible, entry)
		}
		if len(visible) != 1 || !visible[0].IsDir() {
			break
		}
		candidate = filepath.Join(candidate, visible[0].Name())
	}

	info, err := os.Stat(candidate)
	if err != nil || !info.IsDir() {
		return "", errors.New("Projektinhalt im ZIP wurde nicht gefunden")
	}
	return candidate, nil
}

func ValidateVersionFile(contentDir, expected string) error {
	path := filepath.Join(contentDir, "VERSION")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("VERSION-Datei kann nicht gelesen werden: %w", err)
	}

	actual := strings.TrimPrefix(strings.TrimSpace(string(data)), "v")
	if actual != expected {
		return fmt.Errorf("VERSION-Datei (%s) passt nicht zum Archiv (%s)", actual, expected)
	}
	return nil
}

func safeRelativePath(name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "./")
	if name == "" {
		return "", nil
	}
	if strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("absoluter ZIP-Pfad ist nicht erlaubt: %s", name)
	}

	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("unsicherer ZIP-Pfad ist nicht erlaubt: %s", name)
	}
	if volume := filepath.VolumeName(filepath.FromSlash(name)); volume != "" {
		return "", fmt.Errorf("ZIP-Pfad mit Laufwerksangabe ist nicht erlaubt: %s", name)
	}
	return clean, nil
}

func shouldIgnore(relative string) bool {
	parts := strings.Split(filepath.ToSlash(relative), "/")
	for _, part := range parts {
		if part == "__MACOSX" || part == ".DS_Store" {
			return true
		}
	}
	return false
}

func extractFile(entry *zip.File, target string) error {
	source, err := entry.Open()
	if err != nil {
		return fmt.Errorf("ZIP-Eintrag kann nicht geöffnet werden (%s): %w", entry.Name, err)
	}
	defer source.Close()

	mode := entry.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	destination, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("Datei kann nicht erstellt werden (%s): %w", target, err)
	}

	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	if copyErr != nil {
		return fmt.Errorf("Datei kann nicht entpackt werden (%s): %w", target, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("Datei kann nicht abgeschlossen werden (%s): %w", target, closeErr)
	}
	return nil
}

func directoryMode(mode os.FileMode) os.FileMode {
	mode = mode.Perm()
	if mode == 0 {
		return 0o755
	}
	return mode
}
