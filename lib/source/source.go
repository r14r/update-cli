package source

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/tools"
	versionutil "github.com/r14r/update-cli/lib/version"
)

const (
	Download   = "download"
	URL        = "url"
	Repository = "repository"
)

type Options struct {
	ProjectName          string
	Source               config.SourceConfig
	WorkDir, ReleaseRoot string
	Simulation           bool
	AllowHTTP            bool
	MaxArchiveBytes      int64
}
type Metadata struct {
	Type        string              `json:"type"`
	Reference   string              `json:"reference"`
	Version     versionutil.Version `json:"-"`
	VersionText string              `json:"version"`
	Size        int64               `json:"sizeBytes,omitempty"`
	Commit      string              `json:"commit,omitempty"`
}
type Artifact struct {
	Metadata
	ArchivePath string `json:"archivePath,omitempty"`
	ContentDir  string `json:"contentDir,omitempty"`
	StagingDir  string `json:"-"`
	SHA256      string `json:"sha256,omitempty"`
}

func NormalizeKind(v string) (string, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return Download, nil
	}
	switch v {
	case Download, URL, Repository:
		return v, nil
	default:
		return "", fmt.Errorf("Quelle muss download, url oder repository sein; erhalten %q", v)
	}
}
func Discover(ctx context.Context, o Options) (Metadata, error) {
	kind, err := NormalizeKind(o.Source.Type)
	if err != nil {
		return Metadata{}, err
	}
	switch kind {
	case Download:
		return discoverDownload(o)
	case URL:
		return discoverURL(ctx, o)
	case Repository:
		return discoverRepository(ctx, o)
	default:
		return Metadata{}, errors.New("unbekannte Quelle")
	}
}
func Fetch(ctx context.Context, o Options) (Artifact, error) {
	kind, err := NormalizeKind(o.Source.Type)
	if err != nil {
		return Artifact{}, err
	}
	switch kind {
	case Download:
		return fetchDownload(o)
	case URL:
		return fetchURL(ctx, o)
	case Repository:
		return fetchRepository(ctx, o)
	default:
		return Artifact{}, errors.New("unbekannte Quelle")
	}
}
func discoverDownload(o Options) (Metadata, error) {
	p, v, err := versionutil.SelectNewest(o.Source.Folder, o.ProjectName)
	if err != nil {
		return Metadata{}, err
	}
	i, err := os.Stat(p)
	if err != nil {
		return Metadata{}, err
	}
	return Metadata{Type: Download, Reference: p, Version: v, VersionText: v.String(), Size: i.Size()}, nil
}
func fetchDownload(o Options) (Artifact, error) {
	m, err := discoverDownload(o)
	if err != nil {
		return Artifact{}, err
	}
	sha, err := verifySHA256(m.Reference, o.Source.SHA256)
	if err != nil {
		return Artifact{}, err
	}
	return Artifact{Metadata: m, ArchivePath: m.Reference, SHA256: sha}, nil
}
func httpClient(allowHTTP bool) *http.Client {
	return &http.Client{Timeout: 15 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 10 {
			return errors.New("zu viele HTTP-Weiterleitungen")
		}
		if req.URL.Scheme == "http" && !allowHTTP {
			return errors.New("Weiterleitung auf unsicheres HTTP ist blockiert")
		}
		return nil
	}}
}
func validateURL(ref string, allowHTTP bool) (*url.URL, error) {
	u, err := url.Parse(ref)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "https" && !(allowHTTP && u.Scheme == "http") {
		return nil, fmt.Errorf("URL muss HTTPS verwenden%s", func() string {
			if allowHTTP {
				return " oder explizit erlaubtes HTTP"
			}
			return ""
		}())
	}
	return u, nil
}
func discoverURL(ctx context.Context, o Options) (Metadata, error) {
	ref := strings.TrimSpace(o.Source.URL)
	u, err := validateURL(ref, o.AllowHTTP)
	if err != nil {
		return Metadata{}, err
	}
	client := httpClient(o.AllowHTTP)
	request := func(method string, useRange bool) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, method, ref, nil)
		if err != nil {
			return nil, err
		}
		if useRange {
			req.Header.Set("Range", "bytes=0-0")
		}
		return client.Do(req)
	}
	resp, err := request(http.MethodHead, false)
	if err != nil {
		return Metadata{}, fmt.Errorf("Release-URL kann nicht geprüft werden: %w", err)
	}
	if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented {
		_ = resp.Body.Close()
		resp, err = request(http.MethodGet, true)
		if err != nil {
			return Metadata{}, fmt.Errorf("Release-URL kann nicht geprüft werden: %w", err)
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return Metadata{}, fmt.Errorf("Release-URL liefert HTTP %s", resp.Status)
	}
	name := filenameFromResponse(resp, u)
	v, err := versionFromSourceOrName(o.Source.Version, o.ProjectName, name)
	if err != nil {
		return Metadata{}, err
	}
	size := resp.ContentLength
	if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
		if slash := strings.LastIndex(contentRange, "/"); slash >= 0 {
			if total, parseErr := strconv.ParseInt(contentRange[slash+1:], 10, 64); parseErr == nil {
				size = total
			}
		}
	}
	if size > 0 && o.MaxArchiveBytes > 0 && size > o.MaxArchiveBytes {
		return Metadata{}, fmt.Errorf("Release ist zu groß: %d > %d Bytes", size, o.MaxArchiveBytes)
	}
	return Metadata{Type: URL, Reference: ref, Version: v, VersionText: v.String(), Size: max0(size)}, nil
}

func fetchURL(ctx context.Context, o Options) (Artifact, error) {
	ref := strings.TrimSpace(o.Source.URL)
	u, err := validateURL(ref, o.AllowHTTP)
	if err != nil {
		return Artifact{}, err
	}
	if o.WorkDir == "" {
		return Artifact{}, errors.New("interner Arbeitsordner für URL-Download fehlt")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ref, nil)
	if err != nil {
		return Artifact{}, err
	}
	resp, err := httpClient(o.AllowHTTP).Do(req)
	if err != nil {
		return Artifact{}, fmt.Errorf("Release-URL kann nicht geladen werden: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Artifact{}, fmt.Errorf("Release-URL liefert HTTP %s", resp.Status)
	}
	if resp.ContentLength > 0 && o.MaxArchiveBytes > 0 && resp.ContentLength > o.MaxArchiveBytes {
		return Artifact{}, fmt.Errorf("Release ist zu groß: %d > %d Bytes", resp.ContentLength, o.MaxArchiveBytes)
	}
	name := filenameFromResponse(resp, u)
	v, err := versionFromSourceOrName(o.Source.Version, o.ProjectName, name)
	if err != nil {
		return Artifact{}, err
	}
	dir := filepath.Join(o.WorkDir, "url")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Artifact{}, err
	}
	dest := filepath.Join(dir, filepath.Base(name))
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return Artifact{}, err
	}
	h := sha256.New()
	writer := io.MultiWriter(f, h)
	limit := o.MaxArchiveBytes
	if limit <= 0 {
		limit = 2 << 30
	}
	n, copyErr := io.Copy(writer, io.LimitReader(resp.Body, limit+1))
	closeErr := f.Close()
	if copyErr != nil {
		return Artifact{}, copyErr
	}
	if closeErr != nil {
		return Artifact{}, closeErr
	}
	if n > limit {
		_ = os.Remove(dest)
		return Artifact{}, fmt.Errorf("Release überschreitet maximale Downloadgröße von %d Bytes", limit)
	}
	sum := fmt.Sprintf("%x", h.Sum(nil))
	if expected := normalizeHash(o.Source.SHA256); expected != "" && sum != expected {
		_ = os.Remove(dest)
		return Artifact{}, fmt.Errorf("SHA-256 stimmt nicht: erwartet %s, erhalten %s", expected, sum)
	}
	return Artifact{Metadata: Metadata{Type: URL, Reference: ref, Version: v, VersionText: v.String(), Size: n}, ArchivePath: dest, SHA256: sum}, nil
}
func discoverRepository(ctx context.Context, o Options) (Metadata, error) {
	repo := strings.TrimSpace(o.Source.Repository)
	if repo == "" {
		return Metadata{}, errors.New("Repository-Quelle benötigt repository")
	}
	if _, err := exec.LookPath("git"); err != nil {
		return Metadata{}, errors.New("erforderliches Programm fehlt: git")
	}
	if strings.TrimSpace(o.Source.Version) != "" {
		v, err := versionutil.Parse(o.Source.Version)
		if err != nil {
			return Metadata{}, err
		}
		commit, err := lsRemote(ctx, repo, firstNonEmpty(o.Source.Commit, o.Source.Ref, "HEAD"))
		if err != nil {
			return Metadata{}, err
		}
		if o.Source.Commit != "" && !strings.HasPrefix(commit, o.Source.Commit) {
			return Metadata{}, fmt.Errorf("Repository-Commit stimmt nicht: erwartet %s, erhalten %s", o.Source.Commit, commit)
		}
		return Metadata{Type: Repository, Reference: repo, Version: v, VersionText: v.String(), Commit: commit}, nil
	}
	if v, err := versionutil.Parse(strings.TrimPrefix(o.Source.Ref, "refs/tags/")); err == nil && o.Source.Ref != "" {
		commit, err := lsRemote(ctx, repo, o.Source.Ref)
		if err != nil {
			return Metadata{}, err
		}
		return Metadata{Type: Repository, Reference: repo, Version: v, VersionText: v.String(), Commit: commit}, nil
	}
	if strings.TrimSpace(o.Source.Ref) == "" && strings.TrimSpace(o.Source.Commit) == "" {
		if v, commit, ok, err := latestSemverTag(ctx, repo, o.ProjectName); err != nil {
			return Metadata{}, err
		} else if ok {
			return Metadata{Type: Repository, Reference: repo, Version: v, VersionText: v.String(), Commit: commit}, nil
		}
	}
	tmp, err := os.MkdirTemp("", "update-cli-repo-meta-*")
	if err != nil {
		return Metadata{}, err
	}
	defer tools.RemoveTree(tmp)
	a, err := fetchRepository(ctx, Options{ProjectName: o.ProjectName, Source: o.Source, WorkDir: tmp, ReleaseRoot: tmp, Simulation: true, AllowHTTP: o.AllowHTTP, MaxArchiveBytes: o.MaxArchiveBytes})
	if err != nil {
		return Metadata{}, err
	}
	return a.Metadata, nil
}
func fetchRepository(ctx context.Context, o Options) (Artifact, error) {
	repo := strings.TrimSpace(o.Source.Repository)
	if repo == "" {
		return Artifact{}, errors.New("Repository-Quelle benötigt repository")
	}
	if _, err := exec.LookPath("git"); err != nil {
		return Artifact{}, errors.New("erforderliches Programm fehlt: git")
	}
	parent := o.WorkDir
	if !o.Simulation && o.ReleaseRoot != "" {
		parent = o.ReleaseRoot
	}
	if parent == "" {
		return Artifact{}, errors.New("Repository-Stagingordner fehlt")
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return Artifact{}, err
	}
	dest, err := os.MkdirTemp(parent, ".repository-clone-*")
	if err != nil {
		return Artifact{}, err
	}
	_ = os.Remove(dest)
	args := []string{"clone", "--depth", "1", "--single-branch"}
	if strings.TrimSpace(o.Source.Ref) != "" {
		args = append(args, "--branch", o.Source.Ref)
	}
	args = append(args, repo, dest)
	cmd := exec.CommandContext(ctx, "git", args...)
	var b strings.Builder
	cmd.Stdout = &b
	cmd.Stderr = &b
	if err := cmd.Run(); err != nil {
		_ = tools.RemoveTree(dest)
		return Artifact{}, fmt.Errorf("Repository kann nicht geklont werden: %s", strings.TrimSpace(b.String()))
	}
	commitBytes, err := exec.CommandContext(ctx, "git", "-C", dest, "rev-parse", "HEAD").Output()
	if err != nil {
		_ = tools.RemoveTree(dest)
		return Artifact{}, err
	}
	commit := strings.TrimSpace(string(commitBytes))
	if expected := strings.TrimSpace(o.Source.Commit); expected != "" && !strings.HasPrefix(commit, expected) {
		_ = tools.RemoveTree(dest)
		return Artifact{}, fmt.Errorf("Repository-Commit stimmt nicht: erwartet %s, erhalten %s", expected, commit)
	}
	data, err := os.ReadFile(filepath.Join(dest, "VERSION"))
	if err != nil {
		_ = tools.RemoveTree(dest)
		return Artifact{}, fmt.Errorf("Repository enthält keine lesbare VERSION-Datei: %w", err)
	}
	v, err := versionutil.Parse(strings.TrimSpace(string(data)))
	if err != nil {
		_ = tools.RemoveTree(dest)
		return Artifact{}, err
	}
	if configured := strings.TrimSpace(o.Source.Version); configured != "" {
		cv, err := versionutil.Parse(configured)
		if err != nil {
			_ = tools.RemoveTree(dest)
			return Artifact{}, err
		}
		if cv.Compare(v) != 0 {
			_ = tools.RemoveTree(dest)
			return Artifact{}, fmt.Errorf("source.version %s stimmt nicht mit VERSION %s überein", cv.String(), v.String())
		}
	}
	if err := tools.RemoveTree(filepath.Join(dest, ".git")); err != nil {
		_ = tools.RemoveTree(dest)
		return Artifact{}, err
	}
	return Artifact{Metadata: Metadata{Type: Repository, Reference: repo, Version: v, VersionText: v.String(), Commit: commit}, ContentDir: dest, StagingDir: dest}, nil
}
func latestSemverTag(ctx context.Context, repo, project string) (versionutil.Version, string, bool, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--tags", "--refs", repo)
	out, err := cmd.Output()
	if err != nil {
		return versionutil.Version{}, "", false, fmt.Errorf("Repository-Tags können nicht gelesen werden: %w", err)
	}
	var best versionutil.Version
	bestCommit := ""
	found := false
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		tag := strings.TrimPrefix(fields[1], "refs/tags/")
		v, err := versionutil.Parse(tag)
		if err != nil {
			continue
		}
		if !found || versionutil.CompareForProject(project, v, best) > 0 {
			best, bestCommit, found = v, fields[0], true
		}
	}
	return best, bestCommit, found, nil
}

func lsRemote(ctx context.Context, repo, ref string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-remote", repo, ref)
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Repository-Metadaten können nicht gelesen werden: %w", err)
	}
	line := strings.TrimSpace(string(b))
	if line == "" {
		return "", fmt.Errorf("Repository-Ref nicht gefunden: %s", ref)
	}
	return strings.Fields(line)[0], nil
}
func filenameFromResponse(resp *http.Response, u *url.URL) string {
	if d := resp.Header.Get("Content-Disposition"); d != "" {
		if _, p, err := mime.ParseMediaType(d); err == nil {
			if n := filepath.Base(strings.TrimSpace(p["filename"])); n != "" && n != "." {
				return n
			}
		}
	}
	n := filepath.Base(u.Path)
	if n == "" || n == "." || n == "/" {
		return "release.zip"
	}
	return n
}
func versionFromSourceOrName(configured, project, name string) (versionutil.Version, error) {
	if strings.TrimSpace(configured) != "" {
		return versionutil.Parse(configured)
	}
	v, err := versionutil.ParseArchiveName(project, name)
	if err != nil {
		return v, fmt.Errorf("URL muss Version über Dateiname oder source.version liefern: %w", err)
	}
	return v, nil
}
func verifySHA256(path, expected string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	sum := fmt.Sprintf("%x", h.Sum(nil))
	if e := normalizeHash(expected); e != "" && sum != e {
		return "", fmt.Errorf("SHA-256 stimmt nicht: erwartet %s, erhalten %s", e, sum)
	}
	return sum, nil
}
func normalizeHash(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "sha256:")
	return s
}
func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
func max0(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}
