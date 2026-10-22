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
	rsyncutil "github.com/r14r/update-cli/lib/rsync"
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
	Mode                 string
	Source               config.SourceConfig
	WorkDir, ReleaseRoot string
	RepositoryCacheDir   string
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
func effectiveMode(o Options, kind string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(o.Mode))
	if mode == "" {
		if kind == Repository {
			mode = config.ModePull
		} else {
			mode = config.ModeUpdate
		}
	}
	if mode != config.ModeUpdate && mode != config.ModePull {
		return "", fmt.Errorf("unbekannter Update-Modus %q", mode)
	}
	if mode == config.ModePull && kind != Repository {
		return "", errors.New("mode pull benötigt eine Repository-Quelle")
	}
	if mode == config.ModeUpdate && kind == Repository {
		return "", errors.New("mode update erwartet eine ZIP-Quelle; für Git-Repositories mode pull verwenden")
	}
	return mode, nil
}

func Discover(ctx context.Context, o Options) (Metadata, error) {
	kind, err := NormalizeKind(o.Source.Type)
	if err != nil {
		return Metadata{}, err
	}
	mode, err := effectiveMode(o, kind)
	if err != nil {
		return Metadata{}, err
	}
	if mode == config.ModePull {
		return discoverPullRepository(ctx, o)
	}
	switch kind {
	case Download:
		return discoverDownload(o)
	case URL:
		return discoverURL(ctx, o)
	default:
		return Metadata{}, errors.New("unbekannte Quelle")
	}
}
func Fetch(ctx context.Context, o Options) (Artifact, error) {
	kind, err := NormalizeKind(o.Source.Type)
	if err != nil {
		return Artifact{}, err
	}
	mode, err := effectiveMode(o, kind)
	if err != nil {
		return Artifact{}, err
	}
	if mode == config.ModePull {
		return fetchPullRepository(ctx, o)
	}
	switch kind {
	case Download:
		return fetchDownload(o)
	case URL:
		return fetchURL(ctx, o)
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
func discoverPullRepository(ctx context.Context, o Options) (Metadata, error) {
	cache, err := ensureRepositoryCache(ctx, o)
	if err != nil {
		return Metadata{}, err
	}
	if _, err := gitRun(ctx, cache, "fetch", "--prune", "--tags", "origin"); err != nil {
		return Metadata{}, err
	}
	commit, err := repositoryTargetCommit(ctx, cache, o.Source.Ref)
	if err != nil {
		return Metadata{}, err
	}
	if expected := strings.TrimSpace(o.Source.Commit); expected != "" && !strings.HasPrefix(commit, expected) {
		return Metadata{}, fmt.Errorf("Repository-Commit stimmt nicht: erwartet %s, erhalten %s", expected, commit)
	}
	v, err := repositoryVersionAt(ctx, cache, commit)
	if err != nil {
		return Metadata{}, err
	}
	if configured := strings.TrimSpace(o.Source.Version); configured != "" {
		cv, parseErr := versionutil.Parse(configured)
		if parseErr != nil {
			return Metadata{}, parseErr
		}
		if cv.Compare(v) != 0 {
			return Metadata{}, fmt.Errorf("source.version %s stimmt nicht mit VERSION %s überein", cv.String(), v.String())
		}
	}
	return Metadata{Type: Repository, Reference: strings.TrimSpace(o.Source.Repository), Version: v, VersionText: v.String(), Commit: commit}, nil
}

func fetchPullRepository(ctx context.Context, o Options) (Artifact, error) {
	cache, err := ensureRepositoryCache(ctx, o)
	if err != nil {
		return Artifact{}, err
	}
	if status, err := gitRun(ctx, cache, "status", "--porcelain", "--untracked-files=all"); err != nil {
		return Artifact{}, err
	} else if strings.TrimSpace(status) != "" {
		return Artifact{}, fmt.Errorf("interner Repository-Checkout enthält lokale Änderungen: %s", strings.TrimSpace(status))
	}
	if _, err := gitRun(ctx, cache, "fetch", "--prune", "--tags", "origin"); err != nil {
		return Artifact{}, err
	}
	branch, isBranch, err := prepareRepositoryCheckout(ctx, cache, o.Source.Ref)
	if err != nil {
		return Artifact{}, err
	}
	if isBranch {
		if _, err := gitRun(ctx, cache, "pull", "--ff-only", "origin", branch); err != nil {
			return Artifact{}, fmt.Errorf("Repository kann nicht per Fast-Forward aktualisiert werden: %w", err)
		}
	}
	commit, err := gitRun(ctx, cache, "rev-parse", "HEAD")
	if err != nil {
		return Artifact{}, err
	}
	commit = strings.TrimSpace(commit)
	if expected := strings.TrimSpace(o.Source.Commit); expected != "" && !strings.HasPrefix(commit, expected) {
		return Artifact{}, fmt.Errorf("Repository-Commit stimmt nicht: erwartet %s, erhalten %s", expected, commit)
	}
	v, err := repositoryVersionAt(ctx, cache, commit)
	if err != nil {
		return Artifact{}, err
	}
	if configured := strings.TrimSpace(o.Source.Version); configured != "" {
		cv, parseErr := versionutil.Parse(configured)
		if parseErr != nil {
			return Artifact{}, parseErr
		}
		if cv.Compare(v) != 0 {
			return Artifact{}, fmt.Errorf("source.version %s stimmt nicht mit VERSION %s überein", cv.String(), v.String())
		}
	}
	if strings.TrimSpace(o.WorkDir) == "" {
		return Artifact{}, errors.New("interner Arbeitsordner für Repository-Snapshot fehlt")
	}
	content := filepath.Join(o.WorkDir, "repository-content")
	if err := tools.RemoveTree(content); err != nil {
		return Artifact{}, err
	}
	if _, err := rsyncutil.Release(ctx, cache, content, filepath.Join(o.WorkDir, "repository-snapshot.log")); err != nil {
		return Artifact{}, fmt.Errorf("Repository-Snapshot kann nicht erzeugt werden: %w", err)
	}
	return Artifact{Metadata: Metadata{Type: Repository, Reference: strings.TrimSpace(o.Source.Repository), Version: v, VersionText: v.String(), Commit: commit}, ContentDir: content}, nil
}

func ensureRepositoryCache(ctx context.Context, o Options) (string, error) {
	repo := strings.TrimSpace(o.Source.Repository)
	if repo == "" {
		return "", errors.New("Repository-Quelle benötigt repository")
	}
	if _, err := exec.LookPath("git"); err != nil {
		return "", errors.New("erforderliches Programm fehlt: git")
	}
	cache := strings.TrimSpace(o.RepositoryCacheDir)
	if cache == "" {
		return "", errors.New("Repository-Cacheordner fehlt")
	}
	if _, err := os.Stat(filepath.Join(cache, ".git")); errors.Is(err, os.ErrNotExist) {
		if err := tools.RemoveTree(cache); err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
			return "", err
		}
		args := []string{"clone"}
		if ref := strings.TrimSpace(o.Source.Ref); ref != "" {
			args = append(args, "--branch", ref)
		}
		args = append(args, repo, cache)
		cmd := exec.CommandContext(ctx, "git", args...)
		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			_ = tools.RemoveTree(cache)
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = strings.TrimSpace(stdout.String())
			}
			return "", fmt.Errorf("Repository kann nicht geklont werden: %s", msg)
		}
	} else if err != nil {
		return "", err
	}
	origin, err := gitRun(ctx, cache, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	if !sameRepository(strings.TrimSpace(origin), repo) {
		return "", fmt.Errorf("Repository-Cache verweist auf %s statt auf %s; Cache %s entfernen oder Konfiguration korrigieren", strings.TrimSpace(origin), repo, cache)
	}
	return cache, nil
}

func repositoryTargetCommit(ctx context.Context, cache, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref != "" {
		candidates := []string{"refs/remotes/origin/" + ref, "refs/tags/" + strings.TrimPrefix(ref, "refs/tags/"), ref}
		for _, candidate := range candidates {
			if out, err := gitRun(ctx, cache, "rev-parse", "--verify", candidate+"^{commit}"); err == nil {
				return strings.TrimSpace(out), nil
			}
		}
		return "", fmt.Errorf("Repository-Ref nicht gefunden: %s", ref)
	}
	if out, err := gitRun(ctx, cache, "rev-parse", "--verify", "refs/remotes/origin/HEAD^{commit}"); err == nil {
		return strings.TrimSpace(out), nil
	}
	if _, err := gitRun(ctx, cache, "remote", "set-head", "origin", "-a"); err == nil {
		if out, err := gitRun(ctx, cache, "rev-parse", "--verify", "refs/remotes/origin/HEAD^{commit}"); err == nil {
			return strings.TrimSpace(out), nil
		}
	}
	return "", errors.New("Default-Branch des Repository kann nicht bestimmt werden; source.ref konfigurieren")
}

func prepareRepositoryCheckout(ctx context.Context, cache, ref string) (string, bool, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		branch, err := gitRun(ctx, cache, "symbolic-ref", "--quiet", "--short", "HEAD")
		if err == nil && strings.TrimSpace(branch) != "" {
			return strings.TrimSpace(branch), true, nil
		}
		remoteHead, err := gitRun(ctx, cache, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
		if err != nil {
			return "", false, errors.New("Default-Branch des Repository kann nicht bestimmt werden; source.ref konfigurieren")
		}
		branch = strings.TrimPrefix(strings.TrimSpace(remoteHead), "origin/")
		if _, err := gitRun(ctx, cache, "checkout", "-B", branch, "origin/"+branch); err != nil {
			return "", false, err
		}
		return branch, true, nil
	}
	branchRef := "refs/remotes/origin/" + ref
	if _, err := gitRun(ctx, cache, "rev-parse", "--verify", branchRef+"^{commit}"); err == nil {
		current, _ := gitRun(ctx, cache, "symbolic-ref", "--quiet", "--short", "HEAD")
		if strings.TrimSpace(current) != ref {
			if _, err := gitRun(ctx, cache, "show-ref", "--verify", "--quiet", "refs/heads/"+ref); err == nil {
				if _, err := gitRun(ctx, cache, "checkout", ref); err != nil {
					return "", false, err
				}
			} else if _, err := gitRun(ctx, cache, "checkout", "-b", ref, "--track", "origin/"+ref); err != nil {
				return "", false, err
			}
		}
		return ref, true, nil
	}
	tag := strings.TrimPrefix(ref, "refs/tags/")
	if _, err := gitRun(ctx, cache, "rev-parse", "--verify", "refs/tags/"+tag+"^{commit}"); err == nil {
		if _, err := gitRun(ctx, cache, "checkout", "--detach", "refs/tags/"+tag); err != nil {
			return "", false, err
		}
		return tag, false, nil
	}
	return "", false, fmt.Errorf("Repository-Ref nicht gefunden: %s", ref)
}

func repositoryVersionAt(ctx context.Context, cache, commit string) (versionutil.Version, error) {
	data, err := gitRun(ctx, cache, "show", strings.TrimSpace(commit)+":VERSION")
	if err != nil {
		return versionutil.Version{}, fmt.Errorf("Repository enthält am Ziel-Commit keine lesbare VERSION-Datei: %w", err)
	}
	v, err := versionutil.Parse(strings.TrimSpace(data))
	if err != nil {
		return versionutil.Version{}, err
	}
	return v, nil
}

func gitRun(ctx context.Context, dir string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s fehlgeschlagen: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func sameRepository(a, b string) bool {
	normalize := func(v string) string {
		v = strings.TrimSpace(strings.TrimSuffix(v, "/"))
		v = strings.TrimSuffix(v, ".git")
		return v
	}
	return normalize(a) == normalize(b)
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
func max0(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}
