package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"release-updater/lib/tools"
	versionutil "release-updater/lib/version"
)

const (
	Download   = "download"
	URL        = "url"
	Repository = "repository"
)

type Options struct {
	Type        string
	ProjectName string
	Folder      string
	URL         string
	Repository  string
	WorkDir     string
	ReleaseRoot string
	Simulation  bool
}

type Artifact struct {
	Type        string              `json:"type"`
	Reference   string              `json:"reference"`
	Version     versionutil.Version `json:"-"`
	VersionText string              `json:"version"`
	ArchivePath string              `json:"archivePath,omitempty"`
	ContentDir  string              `json:"contentDir,omitempty"`
	StagingDir  string              `json:"-"`
}

func NormalizeKind(value string) (string, error) {
	kind := strings.ToLower(strings.TrimSpace(value))
	if kind == "" {
		return Download, nil
	}
	switch kind {
	case Download, URL, Repository:
		return kind, nil
	default:
		return "", fmt.Errorf("--from muss download, url oder repository sein; erhalten: %q", value)
	}
}

func Resolve(ctx context.Context, options Options) (Artifact, error) {
	kind, err := NormalizeKind(options.Type)
	if err != nil {
		return Artifact{}, err
	}
	options.Type = kind
	switch kind {
	case Download:
		return resolveDownload(options)
	case URL:
		return resolveURL(ctx, options)
	case Repository:
		return resolveRepository(ctx, options)
	default:
		return Artifact{}, fmt.Errorf("nicht unterstützte Quelle: %s", kind)
	}
}

func Discover(ctx context.Context, options Options) (Artifact, error) {
	// Discovery must not leave repository clones or downloaded URL artifacts behind.
	workDir, err := os.MkdirTemp("", "update-cli-source-discovery-*")
	if err != nil {
		return Artifact{}, fmt.Errorf("temporärer Quellordner kann nicht erstellt werden: %w", err)
	}
	defer tools.RemoveTree(workDir)
	options.WorkDir = workDir
	options.Simulation = true
	return Resolve(ctx, options)
}

func resolveDownload(options Options) (Artifact, error) {
	folder := strings.TrimSpace(options.Folder)
	if folder == "" {
		return Artifact{}, errors.New("Download-Quelle benötigt einen Ordner (--folder oder source.folder)")
	}
	path, selected, err := versionutil.SelectNewest(folder, options.ProjectName)
	if err != nil {
		return Artifact{}, err
	}
	return Artifact{
		Type:        Download,
		Reference:   path,
		Version:     selected,
		VersionText: selected.String(),
		ArchivePath: path,
	}, nil
}

func resolveURL(ctx context.Context, options Options) (Artifact, error) {
	reference := strings.TrimSpace(options.URL)
	if reference == "" {
		return Artifact{}, errors.New("URL-Quelle benötigt --url oder source.url")
	}
	if options.WorkDir == "" {
		return Artifact{}, errors.New("interner Arbeitsordner für URL-Download fehlt")
	}
	parsed, err := url.Parse(reference)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return Artifact{}, fmt.Errorf("ungültige HTTP(S)-URL: %s", reference)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, reference, nil)
	if err != nil {
		return Artifact{}, fmt.Errorf("Download-Anfrage kann nicht erstellt werden: %w", err)
	}
	client := &http.Client{Timeout: 15 * time.Minute}
	response, err := client.Do(request)
	if err != nil {
		return Artifact{}, fmt.Errorf("Release-URL kann nicht geladen werden: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Artifact{}, fmt.Errorf("Release-URL liefert HTTP %s", response.Status)
	}

	filename := filenameFromResponse(response, parsed)
	selected, err := versionutil.ParseArchiveName(options.ProjectName, filename)
	if err != nil {
		return Artifact{}, fmt.Errorf("URL muss ein Archiv namens %s-v1.2.3.zip liefern: %w", options.ProjectName, err)
	}
	destinationDir := filepath.Join(options.WorkDir, "url")
	if err := os.MkdirAll(destinationDir, 0o755); err != nil {
		return Artifact{}, fmt.Errorf("URL-Downloadordner kann nicht erstellt werden: %w", err)
	}
	destination := filepath.Join(destinationDir, filename)
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return Artifact{}, fmt.Errorf("Release-Download kann nicht erstellt werden: %w", err)
	}
	_, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return Artifact{}, fmt.Errorf("Release-URL konnte nicht vollständig gespeichert werden: %w", copyErr)
	}
	if closeErr != nil {
		return Artifact{}, fmt.Errorf("Release-Download kann nicht abgeschlossen werden: %w", closeErr)
	}
	return Artifact{
		Type:        URL,
		Reference:   reference,
		Version:     selected,
		VersionText: selected.String(),
		ArchivePath: destination,
	}, nil
}

func filenameFromResponse(response *http.Response, parsed *url.URL) string {
	if disposition := response.Header.Get("Content-Disposition"); disposition != "" {
		if _, parameters, err := mime.ParseMediaType(disposition); err == nil {
			if name := filepath.Base(strings.TrimSpace(parameters["filename"])); name != "" && name != "." {
				return name
			}
		}
	}
	name := filepath.Base(parsed.Path)
	if name == "" || name == "." || name == "/" {
		return "release.zip"
	}
	return name
}

func resolveRepository(ctx context.Context, options Options) (Artifact, error) {
	repository := strings.TrimSpace(options.Repository)
	if repository == "" {
		return Artifact{}, errors.New("Repository-Quelle benötigt --repository oder source.repository")
	}
	if _, err := exec.LookPath("git"); err != nil {
		return Artifact{}, errors.New("erforderliches Programm fehlt: git")
	}

	parent := options.WorkDir
	if !options.Simulation {
		parent = options.ReleaseRoot
	}
	if strings.TrimSpace(parent) == "" {
		return Artifact{}, errors.New("interner Arbeitsordner für Repository fehlt")
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return Artifact{}, fmt.Errorf("Repository-Stagingordner kann nicht erstellt werden: %w", err)
	}
	destination, err := os.MkdirTemp(parent, ".repository-clone-*")
	if err != nil {
		return Artifact{}, fmt.Errorf("Repository-Stagingordner kann nicht angelegt werden: %w", err)
	}
	// git clone expects the destination not to exist.
	if err := os.Remove(destination); err != nil {
		return Artifact{}, fmt.Errorf("Repository-Stagingordner kann nicht vorbereitet werden: %w", err)
	}

	command := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--single-branch", repository, destination)
	var output strings.Builder
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		_ = tools.RemoveTree(destination)
		message := strings.TrimSpace(output.String())
		if message == "" {
			message = err.Error()
		}
		return Artifact{}, fmt.Errorf("Repository kann nicht geklont werden: %s", message)
	}

	versionPath := filepath.Join(destination, "VERSION")
	data, err := os.ReadFile(versionPath)
	if err != nil {
		_ = tools.RemoveTree(destination)
		return Artifact{}, fmt.Errorf("Repository enthält keine lesbare VERSION-Datei im Projektstamm: %w", err)
	}
	selected, err := versionutil.Parse(strings.TrimSpace(string(data)))
	if err != nil {
		_ = tools.RemoveTree(destination)
		return Artifact{}, fmt.Errorf("Repository enthält eine ungültige VERSION-Datei: %w", err)
	}
	if err := tools.RemoveTree(filepath.Join(destination, ".git")); err != nil {
		_ = tools.RemoveTree(destination)
		return Artifact{}, fmt.Errorf("Git-Metadaten können nicht aus dem Release entfernt werden: %w", err)
	}
	return Artifact{
		Type:        Repository,
		Reference:   repository,
		Version:     selected,
		VersionText: selected.String(),
		ContentDir:  destination,
		StagingDir:  destination,
	}, nil
}
