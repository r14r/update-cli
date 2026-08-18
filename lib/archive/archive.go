package archive

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Limits struct {
	MaxEntries                         int
	MaxUncompressedBytes, MaxFileBytes int64
	MaxCompressionRatio                float64
}
type Stats struct {
	Entries           int   `json:"entries"`
	Files             int   `json:"files"`
	Directories       int   `json:"directories"`
	UncompressedBytes int64 `json:"uncompressedBytes"`
	CompressedBytes   int64 `json:"compressedBytes"`
}

func Inspect(ctx context.Context, path string, limits Limits) (Stats, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return Stats{}, fmt.Errorf("ZIP kann nicht geöffnet werden: %w", err)
	}
	defer r.Close()
	stats, err := inspectMetadata(ctx, r.File, limits)
	if err != nil {
		return Stats{}, err
	}
	for _, e := range r.File {
		if e.FileInfo().IsDir() {
			continue
		}
		if err := verifyEntryData(ctx, e); err != nil {
			return Stats{}, err
		}
	}
	return stats, nil
}

func Validate(ctx context.Context, path string, limits Limits) error {
	_, err := Inspect(ctx, path, limits)
	return err
}

// ExtractWithStats performs a metadata preflight and then extracts each entry
// exactly once. The extraction pass itself validates the real uncompressed size
// and ZIP checksum. This avoids the previous Validate -> Inspect -> Extract
// triple decompression for normal update and verify operations while preserving
// the same safety limits.
func ExtractWithStats(ctx context.Context, path, dest string, limits Limits) (Stats, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return Stats{}, fmt.Errorf("ZIP kann nicht geöffnet werden: %w", err)
	}
	defer r.Close()
	stats, err := inspectMetadata(ctx, r.File, limits)
	if err != nil {
		return Stats{}, err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return Stats{}, err
	}
	for _, e := range r.File {
		if err := ctx.Err(); err != nil {
			return Stats{}, err
		}
		rel, err := safeRelativePath(e.Name)
		if err != nil {
			return Stats{}, err
		}
		if rel == "" || shouldIgnore(rel) {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if e.FileInfo().IsDir() {
			if err := os.MkdirAll(target, directoryMode(e.Mode())); err != nil {
				return Stats{}, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return Stats{}, err
		}
		if err := extractFile(ctx, e, target, limits.MaxFileBytes); err != nil {
			return Stats{}, err
		}
	}
	return stats, nil
}

func Extract(ctx context.Context, path, dest string, limits Limits) error {
	_, err := ExtractWithStats(ctx, path, dest, limits)
	return err
}

func inspectMetadata(ctx context.Context, files []*zip.File, limits Limits) (Stats, error) {
	if len(files) == 0 {
		return Stats{}, errors.New("ZIP-Archiv ist leer")
	}
	if limits.MaxEntries > 0 && len(files) > limits.MaxEntries {
		return Stats{}, fmt.Errorf("ZIP enthält zu viele Einträge: %d > %d", len(files), limits.MaxEntries)
	}
	stats := Stats{Entries: len(files)}
	seen := make(map[string]struct{}, len(files))
	for _, e := range files {
		if err := ctx.Err(); err != nil {
			return Stats{}, err
		}
		rel, err := safeRelativePath(e.Name)
		if err != nil {
			return Stats{}, err
		}
		if rel != "" && !shouldIgnore(rel) {
			if _, exists := seen[rel]; exists {
				return Stats{}, fmt.Errorf("doppelter ZIP-Pfad ist nicht erlaubt: %s", rel)
			}
			seen[rel] = struct{}{}
		}
		if e.Mode()&os.ModeSymlink != 0 {
			return Stats{}, fmt.Errorf("symbolischer Link im ZIP ist nicht erlaubt: %s", e.Name)
		}
		if e.FileInfo().IsDir() {
			stats.Directories++
			continue
		}
		stats.Files++
		u := int64(e.UncompressedSize64)
		c := int64(e.CompressedSize64)
		stats.UncompressedBytes += u
		stats.CompressedBytes += c
		if limits.MaxFileBytes > 0 && u > limits.MaxFileBytes {
			return Stats{}, fmt.Errorf("ZIP-Datei %s ist zu groß: %d Bytes", e.Name, u)
		}
		if limits.MaxUncompressedBytes > 0 && stats.UncompressedBytes > limits.MaxUncompressedBytes {
			return Stats{}, fmt.Errorf("ZIP überschreitet maximale entpackte Größe: %d Bytes", stats.UncompressedBytes)
		}
		if limits.MaxCompressionRatio > 0 && c > 0 && float64(u)/float64(c) > limits.MaxCompressionRatio {
			return Stats{}, fmt.Errorf("verdächtiges Kompressionsverhältnis in %s: %.1f", e.Name, float64(u)/float64(c))
		}
	}
	return stats, nil
}

func verifyEntryData(ctx context.Context, e *zip.File) error {
	f, err := e.Open()
	if err != nil {
		return err
	}
	n, copyErr := copyLimitedContext(ctx, io.Discard, f, int64(e.UncompressedSize64)+1)
	closeErr := f.Close()
	if copyErr != nil {
		return fmt.Errorf("ZIP-Prüfung fehlgeschlagen (%s): %w", e.Name, copyErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if n != int64(e.UncompressedSize64) {
		return fmt.Errorf("ZIP-Größe stimmt nicht (%s): erwartet %d, gelesen %d", e.Name, e.UncompressedSize64, n)
	}
	return nil
}

func ValidateTree(ctx context.Context, root string, limits Limits) (Stats, error) {
	s := Stats{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e := ctx.Err(); e != nil {
			return e
		}
		if path == root {
			return nil
		}
		s.Entries++
		if limits.MaxEntries > 0 && s.Entries > limits.MaxEntries {
			return fmt.Errorf("Projekt enthält zu viele Einträge: %d", s.Entries)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolischer Link ist in Release-Inhalten nicht erlaubt: %s", path)
		}
		if d.IsDir() {
			s.Directories++
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("nicht-reguläre Datei ist nicht erlaubt: %s", path)
		}
		s.Files++
		s.UncompressedBytes += info.Size()
		if limits.MaxFileBytes > 0 && info.Size() > limits.MaxFileBytes {
			return fmt.Errorf("Datei ist zu groß: %s", path)
		}
		if limits.MaxUncompressedBytes > 0 && s.UncompressedBytes > limits.MaxUncompressedBytes {
			return fmt.Errorf("Release überschreitet maximale Größe")
		}
		return nil
	})
	return s, err
}
func ResolveContentRoot(dir string) (string, error) {
	c := dir
	for i := 0; i < 12; i++ {
		entries, err := os.ReadDir(c)
		if err != nil {
			return "", err
		}
		vis := []os.DirEntry{}
		for _, e := range entries {
			if e.Name() == "__MACOSX" || e.Name() == ".DS_Store" {
				continue
			}
			vis = append(vis, e)
		}
		if len(vis) != 1 || !vis[0].IsDir() {
			break
		}
		c = filepath.Join(c, vis[0].Name())
	}
	i, err := os.Stat(c)
	if err != nil || !i.IsDir() {
		return "", errors.New("Projektinhalt im ZIP wurde nicht gefunden")
	}
	return c, nil
}
func ValidateVersionFile(root, expected string) error {
	b, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	actual := strings.TrimPrefix(strings.TrimSpace(string(b)), "v")
	if actual != expected {
		return fmt.Errorf("VERSION-Datei (%s) passt nicht zum Release (%s)", actual, expected)
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
	if filepath.VolumeName(filepath.FromSlash(name)) != "" {
		return "", fmt.Errorf("ZIP-Pfad mit Laufwerksangabe ist nicht erlaubt: %s", name)
	}
	return clean, nil
}
func shouldIgnore(p string) bool {
	for _, x := range strings.Split(filepath.ToSlash(p), "/") {
		if x == "__MACOSX" || x == ".DS_Store" {
			return true
		}
	}
	return false
}
func extractFile(ctx context.Context, e *zip.File, target string, max int64) error {
	src, err := e.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	mode := e.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	dst, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	limit := int64(e.UncompressedSize64) + 1
	if max > 0 && limit > max+1 {
		limit = max + 1
	}
	n, copyErr := copyLimitedContext(ctx, dst, src, limit)
	closeErr := dst.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if max > 0 && n > max {
		return fmt.Errorf("Datei überschreitet maximale Größe: %s", e.Name)
	}
	if n != int64(e.UncompressedSize64) {
		return fmt.Errorf("Dateigröße stimmt nicht: %s", e.Name)
	}
	return nil
}
func copyLimitedContext(ctx context.Context, dst io.Writer, src io.Reader, max int64) (int64, error) {
	buf := make([]byte, 128*1024)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		want := len(buf)
		if max > 0 && int64(want) > max-total {
			want = int(max - total)
		}
		if want <= 0 {
			return total, nil
		}
		n, err := src.Read(buf[:want])
		if n > 0 {
			w, e := dst.Write(buf[:n])
			total += int64(w)
			if e != nil {
				return total, e
			}
			if w != n {
				return total, io.ErrShortWrite
			}
		}
		if errors.Is(err, io.EOF) {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}
func directoryMode(m os.FileMode) os.FileMode {
	m = m.Perm()
	if m == 0 {
		return 0o755
	}
	return m
}
