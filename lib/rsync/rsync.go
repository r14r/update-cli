package rsync

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ChangeKind string

const (
	ChangeCreated ChangeKind = "created"
	ChangeUpdated ChangeKind = "updated"
	ChangeDeleted ChangeKind = "deleted"
	ChangeOther   ChangeKind = "other"
)

type Change struct {
	Kind ChangeKind `json:"kind"`
	Path string     `json:"path"`
	Raw  string     `json:"raw,omitempty"`
}
type Result struct {
	Changes int      `json:"changes"`
	LogFile string   `json:"logFile,omitempty"`
	Items   []Change `json:"items,omitempty"`
}

func Require() error {
	if _, err := exec.LookPath("rsync"); err != nil {
		return fmt.Errorf("erforderliches Programm fehlt: rsync")
	}
	return nil
}
func Release(ctx context.Context, source, dest, log string) (Result, error) {
	reservedChanges, err := removeNestedRuntimeState(dest, false)
	if err != nil {
		return Result{}, err
	}
	r, err := run(ctx, source, dest, log, false, false, []string{"--exclude=/.git/", "--exclude=/.update-cli/", "--exclude=/.venv/", "--exclude=/__MACOSX/", "--exclude=/.DS_Store"})
	if err != nil {
		return Result{}, err
	}
	return prependChanges(r, reservedChanges), nil
}
func Current(ctx context.Context, source, dest, log string, dry bool, preserve []string) (Result, error) {
	reservedChanges, err := removeNestedRuntimeState(dest, dry)
	if err != nil {
		return Result{}, err
	}
	protected := mandatoryPreserve(preserve)
	seeded, err := seedMissingPreserved(ctx, source, dest, protected, dry)
	if err != nil {
		return Result{}, err
	}
	extra := append([]string{"--exclude=/.update-cli/"}, preserveFilters(protected)...)
	r, err := run(ctx, source, dest, log, dry, true, extra)
	if err != nil {
		return Result{}, err
	}
	return prependChanges(r, reservedChanges, seeded), nil
}
func Snapshot(ctx context.Context, source, dest, log string, dry bool) (Result, error) {
	return run(ctx, source, dest, log, dry, false, []string{"--exclude=/.git/", "--exclude=/.update-cli/", "--exclude=/.venv/", "--exclude=/.env", "--exclude=/.env.*", "--exclude=/node_modules/", "--exclude=/vendor/", "--exclude=/dist/", "--exclude=/build/", "--exclude=/__pycache__/"})
}
func TransactionSnapshot(ctx context.Context, source, dest, log string) (Result, error) {
	return run(ctx, source, dest, log, false, false, []string{"--exclude=/.update-cli/"})
}
func Restore(ctx context.Context, source, dest, log string, dry bool, preserve []string) (Result, error) {
	reservedChanges, err := removeNestedRuntimeState(dest, dry)
	if err != nil {
		return Result{}, err
	}
	protected := mandatoryPreserve(preserve)
	seeded, err := seedMissingPreserved(ctx, source, dest, protected, dry)
	if err != nil {
		return Result{}, err
	}
	extra := append([]string{"--exclude=/.backup.json", "--exclude=/.update-cli/"}, preserveFilters(protected)...)
	r, err := run(ctx, source, dest, log, dry, true, extra)
	if err != nil {
		return Result{}, err
	}
	return prependChanges(r, reservedChanges, seeded), nil
}
func RestoreExact(ctx context.Context, source, dest, log string) (Result, error) {
	reservedChanges, err := removeNestedRuntimeState(dest, false)
	if err != nil {
		return Result{}, err
	}
	r, err := run(ctx, source, dest, log, false, true, []string{"--exclude=/.update-cli/"})
	if err != nil {
		return Result{}, err
	}
	return prependChanges(r, reservedChanges), nil
}

func prependChanges(r Result, groups ...[]Change) Result {
	total := len(r.Items)
	for _, group := range groups {
		total += len(group)
	}
	if total == len(r.Items) {
		return r
	}
	items := make([]Change, 0, total)
	for _, group := range groups {
		items = append(items, group...)
	}
	items = append(items, r.Items...)
	r.Items = items
	r.Changes = len(items)
	return r
}

func removeNestedRuntimeState(dest string, dry bool) ([]Change, error) {
	changes := []Change{}
	for _, name := range []string{".update-cli"} {
		path := filepath.Join(dest, name)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("verschachtelter Runtime-State kann nicht geprüft werden: %s: %w", path, err)
		}
		_ = info
		changes = append(changes, Change{Kind: ChangeDeleted, Path: name + "/", Raw: "reserved-runtime-state|" + name})
		if dry {
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return nil, fmt.Errorf("verschachtelter Runtime-State kann nicht entfernt werden: %s: %w", path, err)
		}
	}
	return changes, nil
}

func preserveFilters(protected []string) []string {
	extra := make([]string, 0, len(protected)*2)
	for _, p := range protected {
		pat := normalizePattern(p)
		if pat == "" {
			continue
		}
		extra = append(extra, "--filter=protect /"+pat, "--exclude=/"+pat)
	}
	return extra
}

// seedMissingPreserved gives preserve rules the intended "keep local if present"
// semantics without making them "never install" rules. If a protected path does
// not exist in the destination yet, it is copied from the release once. Future
// updates then leave the local copy untouched because the normal rsync pass
// excludes protected paths.
func seedMissingPreserved(ctx context.Context, source, dest string, protected []string, dry bool) ([]Change, error) {
	changes := []Change{}
	seen := map[string]bool{}
	for _, raw := range protected {
		pat := normalizePattern(raw)
		if pat == "" {
			continue
		}
		matchPattern := strings.TrimSuffix(pat, "/")
		relMatches, err := fs.Glob(os.DirFS(source), matchPattern)
		if err != nil {
			return nil, fmt.Errorf("ungültiges preserve-Muster %q: %w", raw, err)
		}
		if len(relMatches) == 0 && !strings.ContainsAny(pat, "*?[") {
			relMatches = []string{matchPattern}
		}
		for _, relMatch := range relMatches {
			src := filepath.Join(source, filepath.FromSlash(relMatch))
			info, err := os.Lstat(src)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("preserve-Quelle kann nicht geprüft werden: %s: %w", src, err)
			}
			rel, err := filepath.Rel(source, src)
			if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
				continue
			}
			relSlash := filepath.ToSlash(rel)
			if seen[relSlash] {
				continue
			}
			seen[relSlash] = true
			dst := filepath.Join(dest, rel)
			if _, err := os.Lstat(dst); err == nil {
				continue
			} else if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("preserve-Ziel kann nicht geprüft werden: %s: %w", dst, err)
			}
			changes = append(changes, Change{Kind: ChangeCreated, Path: relSlash, Raw: "seed-preserved|" + relSlash})
			if dry {
				continue
			}
			if info.IsDir() {
				if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
					return nil, fmt.Errorf("preserve-Verzeichnis kann nicht angelegt werden: %s: %w", dst, err)
				}
				cmd := exec.CommandContext(ctx, "rsync", "-a", trailing(src), trailing(dst))
				if out, err := cmd.CombinedOutput(); err != nil {
					return nil, fmt.Errorf("preserve-Verzeichnis kann nicht übernommen werden: %s: %s", relSlash, strings.TrimSpace(string(out)))
				}
				continue
			}
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return nil, fmt.Errorf("preserve-Zielordner kann nicht angelegt werden: %s: %w", filepath.Dir(dst), err)
			}
			cmd := exec.CommandContext(ctx, "rsync", "-a", src, dst)
			if out, err := cmd.CombinedOutput(); err != nil {
				return nil, fmt.Errorf("preserve-Datei kann nicht übernommen werden: %s: %s", relSlash, strings.TrimSpace(string(out)))
			}
		}
	}
	return changes, nil
}

func mandatoryPreserve(preserve []string) []string {
	out := make([]string, 0, len(preserve)+1)
	seen := map[string]bool{}
	for _, p := range append([]string{".gitignore"}, preserve...) {
		pat := normalizePattern(p)
		if pat == "" || seen[pat] {
			continue
		}
		seen[pat] = true
		out = append(out, pat)
	}
	return out
}

func normalizePattern(p string) string {
	p = filepath.ToSlash(strings.TrimSpace(p))
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, "/")
	return p
}
func run(ctx context.Context, source, dest, log string, dry, checksum bool, extra []string) (Result, error) {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return Result{}, fmt.Errorf("rsync-Ziel kann nicht erstellt werden: %w", err)
	}
	if log != "" {
		if err := os.MkdirAll(filepath.Dir(log), 0o755); err != nil {
			return Result{}, err
		}
	}
	args := []string{"-a", "--delete", "--itemize-changes", "--out-format=%i|%n%L"}
	if checksum {
		args = append(args, "--checksum")
	}
	if dry {
		args = append(args, "--dry-run")
	}
	args = append(args, extra...)
	args = append(args, trailing(source), trailing(dest))
	var out, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "rsync", args...)
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return Result{}, fmt.Errorf("rsync fehlgeschlagen: %s", msg)
	}
	if log != "" {
		if err := os.WriteFile(log, out.Bytes(), 0o600); err != nil {
			return Result{}, fmt.Errorf("rsync-Log kann nicht geschrieben werden: %w", err)
		}
	}
	items := ParseChanges(out.String())
	return Result{Changes: len(items), LogFile: log, Items: items}, nil
}
func ParseChanges(v string) []Change {
	res := []Change{}
	sc := bufio.NewScanner(strings.NewReader(v))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "*deleting") {
			p := strings.TrimSpace(strings.TrimPrefix(line, "*deleting"))
			p = strings.TrimPrefix(p, "|")
			res = append(res, Change{Kind: ChangeDeleted, Path: p, Raw: line})
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			res = append(res, Change{Kind: ChangeOther, Path: line, Raw: line})
			continue
		}
		code, p := parts[0], parts[1]
		kind := ChangeUpdated
		if strings.Contains(code, "+++++++++") {
			kind = ChangeCreated
		} else if strings.HasPrefix(code, ".") {
			kind = ChangeOther
		}
		res = append(res, Change{Kind: kind, Path: p, Raw: line})
	}
	return res
}
func trailing(p string) string {
	if strings.HasSuffix(p, string(os.PathSeparator)) {
		return p
	}
	return p + string(os.PathSeparator)
}
func Describe() (string, error) {
	p, err := exec.LookPath("rsync")
	if err != nil {
		return "", err
	}
	b, err := exec.Command(p, "--version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.SplitN(string(b), "\n", 2)[0]), nil
}
