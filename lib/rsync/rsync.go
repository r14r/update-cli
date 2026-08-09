package rsync

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
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
	return run(ctx, source, dest, log, false, []string{"--exclude=/.git/", "--exclude=/.venv/", "--exclude=/.env", "--exclude=/.env.*", "--exclude=/__MACOSX/", "--exclude=/.DS_Store"})
}
func Current(ctx context.Context, source, dest, log string, dry bool, preserve []string) (Result, error) {
	extra := []string{}
	for _, p := range preserve {
		pat := normalizePattern(p)
		extra = append(extra, "--filter=protect /"+pat, "--exclude=/"+pat)
	}
	return run(ctx, source, dest, log, dry, extra)
}
func Snapshot(ctx context.Context, source, dest, log string, dry bool) (Result, error) {
	return run(ctx, source, dest, log, dry, []string{"--exclude=/.git/", "--exclude=/.venv/", "--exclude=/.env", "--exclude=/.env.*", "--exclude=/node_modules/", "--exclude=/vendor/", "--exclude=/dist/", "--exclude=/build/", "--exclude=/__pycache__/"})
}
func TransactionSnapshot(ctx context.Context, source, dest, log string) (Result, error) {
	return run(ctx, source, dest, log, false, nil)
}
func Restore(ctx context.Context, source, dest, log string, dry bool, preserve []string) (Result, error) {
	extra := []string{"--exclude=/.backup.json"}
	for _, p := range preserve {
		pat := normalizePattern(p)
		extra = append(extra, "--filter=protect /"+pat, "--exclude=/"+pat)
	}
	return run(ctx, source, dest, log, dry, extra)
}
func RestoreExact(ctx context.Context, source, dest, log string) (Result, error) {
	return run(ctx, source, dest, log, false, nil)
}
func normalizePattern(p string) string {
	p = filepath.ToSlash(strings.TrimSpace(p))
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, "/")
	return p
}
func run(ctx context.Context, source, dest, log string, dry bool, extra []string) (Result, error) {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return Result{}, fmt.Errorf("rsync-Ziel kann nicht erstellt werden: %w", err)
	}
	if log != "" {
		if err := os.MkdirAll(filepath.Dir(log), 0o755); err != nil {
			return Result{}, err
		}
	}
	args := []string{"-a", "--delete", "--checksum", "--itemize-changes", "--out-format=%i|%n%L"}
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
