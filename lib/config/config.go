package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/r14r/update-cli/lib/buildconfig"
	"github.com/r14r/update-cli/lib/tools"
)

const (
	ConfigDirName         = ".update-cli"
	ConfigFileName        = "config.json"
	ProjectFileName       = "update-cli.yaml"
	TemplatesFileName     = "templates.json"
	SchemaVersion         = 9
	ModeUpdate            = "update"
	ModePull              = "pull"
	DefaultRepositoryUser = "r1r"
)

var projectNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type SourceConfig struct {
	Type        string `json:"type"`
	DefaultUser string `json:"defaultUser,omitempty"`
	Folder      string `json:"folder,omitempty"`
	URL         string `json:"url,omitempty"`
	Repository  string `json:"repository,omitempty"`
	Ref         string `json:"ref,omitempty"`
	Commit      string `json:"commit,omitempty"`
	Version     string `json:"version,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
}
type SetupConfig struct {
	Commands         []string `json:"commands"`
	KeepRsyncOnError *bool    `json:"keepRsyncOnError,omitempty"`
}
type BackupConfig struct {
	Directory string `json:"directory"`
	Keep      int    `json:"keep"`
}
type RetentionConfig struct {
	Releases int `json:"releases"`
}
type SyncConfig struct {
	Preserve []string `json:"preserve"`
}
type SecurityConfig struct {
	AllowHTTP            bool    `json:"allowHttp"`
	MaxArchiveBytes      int64   `json:"maxArchiveBytes"`
	MaxUncompressedBytes int64   `json:"maxUncompressedBytes"`
	MaxFileBytes         int64   `json:"maxFileBytes"`
	MaxEntries           int     `json:"maxEntries"`
	MaxCompressionRatio  float64 `json:"maxCompressionRatio"`
}
type DockerConfig struct {
	Lifecycle string `json:"lifecycle"`
}

type HealthcheckConfig struct {
	Type           string `json:"type,omitempty"`
	URL            string `json:"url,omitempty"`
	Command        string `json:"command,omitempty"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
}
type NoParameterConfig []string

func (v *NoParameterConfig) UnmarshalJSON(data []byte) error {
	var s string
	if json.Unmarshal(data, &s) == nil {
		*v = []string{s}
		return nil
	}
	var a []string
	if err := json.Unmarshal(data, &a); err != nil {
		return errors.New(`"no parameter" muss ein String oder eine Liste von Strings sein`)
	}
	*v = a
	return nil
}
func (v NoParameterConfig) MarshalJSON() ([]byte, error) { return json.Marshal([]string(v)) }

type FileConfig struct {
	SchemaVersion     int                `json:"schemaVersion"`
	ProjectName       string             `json:"projectName"`
	Mode              string             `json:"mode,omitempty"`
	DownloadDir       string             `json:"downloadDir,omitempty"`
	DefaultUser       string             `json:"defaultUser,omitempty"` // legacy top-level alias; migrate() moves this into source.defaultUser
	Source            *SourceConfig      `json:"source,omitempty"`
	ReleaseDir        string             `json:"releaseDir"`
	CurrentDir        string             `json:"currentDir"`
	NoParameter       NoParameterConfig  `json:"no parameter,omitempty"`
	NoParamLegacy     NoParameterConfig  `json:"no-param,omitempty"`
	NoParameterLegacy NoParameterConfig  `json:"no-parameter,omitempty"`
	NoParameterCamel  NoParameterConfig  `json:"noParameter,omitempty"`
	Setup             *SetupConfig       `json:"setup,omitempty"`
	Backup            *BackupConfig      `json:"backup,omitempty"`
	Retention         *RetentionConfig   `json:"retention,omitempty"`
	Sync              *SyncConfig        `json:"sync,omitempty"`
	Security          *SecurityConfig    `json:"security,omitempty"`
	Docker            *DockerConfig      `json:"docker,omitempty"`
	Healthcheck       *HealthcheckConfig `json:"healthcheck,omitempty"`
}

type Config struct {
	RootDir               string            `json:"rootDir"`
	ConfigDir             string            `json:"configDir"`
	ConfigFile            string            `json:"configFile"`
	ProjectName           string            `json:"projectName"`
	Mode                  string            `json:"mode"`
	Source                SourceConfig      `json:"source"`
	SourceType            string            `json:"sourceType"`
	SourceFolder          string            `json:"sourceFolder,omitempty"`
	SourceURL             string            `json:"sourceUrl,omitempty"`
	SourceRepository      string            `json:"sourceRepository,omitempty"`
	DownloadDir           string            `json:"downloadDir,omitempty"`
	ReleaseRoot           string            `json:"releaseDir"`
	CurrentDir            string            `json:"currentDir"`
	BackupRoot            string            `json:"backupDir"`
	KeepBackups           int               `json:"keepBackups"`
	KeepReleases          int               `json:"keepReleases"`
	KeepRsyncOnSetupError bool              `json:"keepRsyncOnSetupError"`
	NoParameterActions    []string          `json:"noParameterActions"`
	LegacySetupCommands   []string          `json:"legacySetupCommands,omitempty"`
	Preserve              []string          `json:"preserve"`
	Security              SecurityConfig    `json:"security"`
	Docker                DockerConfig      `json:"docker"`
	Healthcheck           HealthcheckConfig `json:"healthcheck"`
	HistoryFile           string            `json:"historyFile"`
	TemplatesFile         string            `json:"templatesFile"`
	GlobalConfigDir       string            `json:"globalConfigDir"`
	GlobalConfigFile      string            `json:"globalConfigFile"`
	GlobalTemplatesFile   string            `json:"globalTemplatesFile"`
	RepositoryCacheDir    string            `json:"repositoryCacheDir"`
	ProjectManifestFile   string            `json:"projectManifestFile,omitempty"`
	ManifestOverrides     []string          `json:"manifestOverrides,omitempty"`
}

// ProjectOverrides contains project-versioned runtime settings read from
// update-cli.yaml. Pointer fields distinguish an explicit zero/false value from
// an omitted value. Host security policy and user defaults intentionally do not
// belong here.
type ProjectOverrides struct {
	ProjectName        string
	Mode               *string
	Source             *SourceConfig
	ReleaseDir         *string
	CurrentDir         *string
	BackupDirectory    *string
	BackupKeep         *int
	RetentionReleases  *int
	Preserve           *[]string
	KeepRsyncOnError   *bool
	DockerLifecycle    *string
	HealthcheckType    *string
	HealthcheckURL     *string
	HealthcheckCommand *string
	HealthcheckTimeout *int
}

type InitOptions struct {
	ProjectName, UseTemplate, Mode, SourceType, Folder, URL, Repository string
	Force                                                               bool
}
type UpgradeResult struct {
	ConfigFile     string `json:"configFile"`
	BackupFile     string `json:"backupFile,omitempty"`
	PreviousSchema int    `json:"previousSchemaVersion"`
	CurrentSchema  int    `json:"currentSchemaVersion"`
	Changed        bool   `json:"changed"`
	ProjectName    string `json:"projectName"`
}
type CheckResult struct {
	ConfigFile      string `json:"configFile"`
	SchemaVersion   int    `json:"schemaVersion"`
	CurrentSchema   int    `json:"currentSchemaVersion"`
	MigrationNeeded bool   `json:"migrationNeeded"`
	ProjectName     string `json:"projectName"`
	Mode            string `json:"mode"`
	SourceType      string `json:"sourceType"`
	Valid           bool   `json:"valid"`
}

func defaultPreserve() []string {
	return []string{".git/", ".gitignore", ".venv/", ".env", ".env.*", "data/", "storage/", "uploads/", "media/", "logs/", "var/"}
}
func ensurePreserve(values []string, required ...string) []string {
	out := append([]string(nil), values...)
	seen := make(map[string]bool, len(out))
	for _, value := range out {
		seen[filepath.ToSlash(strings.TrimSpace(value))] = true
	}
	for _, value := range required {
		key := filepath.ToSlash(strings.TrimSpace(value))
		if key == "" || seen[key] {
			continue
		}
		out = append(out, value)
		seen[key] = true
	}
	return out
}

func defaultSecurity() SecurityConfig {
	return SecurityConfig{MaxArchiveBytes: 2 << 30, MaxUncompressedBytes: 8 << 30, MaxFileBytes: 2 << 30, MaxEntries: 100000, MaxCompressionRatio: 200}
}
func defaultFile(project string) FileConfig {
	// sync is intentionally omitted from a new project-local config. The
	// installation-wide config provides the default preserve/exclude policy;
	// migrate() still supplies built-in defaults when no global config exists.
	return FileConfig{SchemaVersion: SchemaVersion, ProjectName: project, Mode: ModeUpdate, Source: &SourceConfig{Type: "download", Folder: buildconfig.Current().DefaultDownloadFolder, DefaultUser: DefaultRepositoryUser}, ReleaseDir: "release", CurrentDir: "current", NoParameter: NoParameterConfig{"update", "no-setup"}, Setup: &SetupConfig{Commands: []string{}}, Backup: &BackupConfig{Directory: "backup", Keep: 3}, Retention: &RetentionConfig{Releases: 5}, Security: ptrSecurity(defaultSecurity()), Docker: &DockerConfig{Lifecycle: "auto"}, Healthcheck: &HealthcheckConfig{}}
}
func ptrSecurity(s SecurityConfig) *SecurityConfig { return &s }

func ResolveRoot(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return absoluteDir(explicit)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// current/ is a replaceable deployment directory, never an independent
	// update-cli project root. Prefer the parent runtime state even when an old
	// broken release accidentally copied .update-cli/ into current/.
	cleanCWD := filepath.Clean(cwd)
	if filepath.Base(cleanCWD) == "current" {
		parent := filepath.Dir(cleanCWD)
		if hasProjectConfiguration(parent) {
			return absoluteDir(parent)
		}
	}
	if root, ok := findProjectRoot(cwd); ok {
		return root, nil
	}
	exe, err := os.Executable()
	if err == nil {
		exe, _ = filepath.EvalSymlinks(exe)
		if root, ok := findProjectRoot(filepath.Dir(exe)); ok {
			return root, nil
		}
	}
	return filepath.Abs(cwd)
}

// findProjectRoot walks from start towards the filesystem root looking for
// .update-cli/config.json. This lets commands be executed from current/ or
// another project subdirectory without requiring an explicit --root.
func findProjectRoot(start string) (string, bool) {
	p, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		if hasProjectConfiguration(p) {
			return p, true
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", false
		}
		p = parent
	}
}
func configFilePath(root string) (string, bool) {
	return filepath.Join(root, ConfigDirName, ConfigFileName), false
}

func Init(root string, o InitOptions) (Config, error) {
	root, err := absoluteDir(root)
	if err != nil {
		return Config{}, err
	}
	name := strings.TrimSpace(o.ProjectName)
	if err := validateProjectName(name); err != nil {
		return Config{}, err
	}
	dir := filepath.Join(root, ConfigDirName)
	file := filepath.Join(dir, ConfigFileName)
	if _, err := os.Stat(file); err == nil && !o.Force {
		return Config{}, fmt.Errorf("Konfiguration existiert bereits: %s; zum Überschreiben --force verwenden", file)
	}
	fc := defaultFile(name)
	if err := applySource(&fc, o.Mode, o.SourceType, o.Folder, o.URL, o.Repository); err != nil {
		return Config{}, err
	}
	if err := writeConfigFile(file, fc); err != nil {
		return Config{}, err
	}
	return Load(root, "")
}

// LoadResilient loads the effective runtime configuration and automatically
// repairs known legacy/transition structures when the strict loader rejects
// them. Repairs always use the existing backup+atomic-write machinery before
// the strict loader is retried.
func LoadResilient(root, downloadOverride string) (Config, *RepairResult, error) {
	cfg, err := Load(root, downloadOverride)
	if err == nil {
		return cfg, nil, nil
	}
	absRoot, absErr := absoluteDir(root)
	if absErr != nil {
		return Config{}, nil, err
	}
	hasRuntimeConfig := false
	if info, statErr := os.Stat(filepath.Join(absRoot, ConfigDirName, ConfigFileName)); statErr == nil && !info.IsDir() {
		hasRuntimeConfig = true
	}
	if !hasRuntimeConfig {
		return Config{}, nil, err
	}
	repair, repairErr := RepairProjectConfig(root)
	if repairErr != nil {
		return Config{}, &repair, fmt.Errorf("config.json konnte nach Ladefehler nicht automatisch repariert werden (%v): %w", err, repairErr)
	}
	cfg, retryErr := Load(root, downloadOverride)
	if retryErr != nil {
		return Config{}, &repair, fmt.Errorf("config.json ist nach automatischer Reparatur weiterhin ungültig: %w", retryErr)
	}
	return cfg, &repair, nil
}

func Load(root, downloadOverride string) (Config, error) {
	root, err := absoluteDir(root)
	if err != nil {
		return Config{}, err
	}
	file, _ := configFilePath(root)
	gdir, err := GlobalConfigDir()
	if err != nil {
		return Config{}, err
	}
	globalFile := filepath.Join(gdir, ConfigFileName)
	fc, err := readMergedConfigFile(file, globalFile)
	if err != nil {
		return Config{}, err
	}
	fc, _, err = migrate(fc)
	if err != nil {
		return Config{}, err
	}
	if err := validateFile(fc); err != nil {
		return Config{}, fmt.Errorf("ungültige Konfiguration %s: %w", file, err)
	}
	src := *fc.Source
	if strings.TrimSpace(downloadOverride) != "" {
		src.Type = "download"
		src.Folder = downloadOverride
	}
	if src.Folder != "" {
		src.Folder, err = expandAndAbs(src.Folder)
		if err != nil {
			return Config{}, err
		}
	}
	rel, err := resolveProjectDirectory(root, fc.ReleaseDir, "releaseDir")
	if err != nil {
		return Config{}, err
	}
	cur, err := resolveProjectDirectory(root, fc.CurrentDir, "currentDir")
	if err != nil {
		return Config{}, err
	}
	backDir := "backup"
	keepB := 3
	if fc.Backup != nil {
		backDir = fc.Backup.Directory
		keepB = fc.Backup.Keep
	}
	back, err := resolveProjectDirectory(root, backDir, "backup.directory")
	if err != nil {
		return Config{}, err
	}
	if samePath(rel, cur) || samePath(back, cur) || samePath(back, rel) {
		return Config{}, errors.New("releaseDir, currentDir und backup.directory müssen verschieden sein")
	}
	for _, p := range []string{rel, cur, back} {
		if _, err := tools.CanonicalInside(root, p, true); err != nil {
			return Config{}, err
		}
	}
	keepR := 5
	if fc.Retention != nil {
		keepR = fc.Retention.Releases
	}
	pres := append([]string(nil), fc.Sync.Preserve...)
	sec := *fc.Security
	docker := DockerConfig{Lifecycle: "auto"}
	if fc.Docker != nil {
		docker = *fc.Docker
	}
	hc := HealthcheckConfig{}
	if fc.Healthcheck != nil {
		hc = *fc.Healthcheck
	}
	legacy := []string{}
	keepRsyncOnSetupError := false
	if fc.Setup != nil {
		legacy = append(legacy, fc.Setup.Commands...)
		if fc.Setup.KeepRsyncOnError != nil {
			keepRsyncOnSetupError = *fc.Setup.KeepRsyncOnError
		}
	}
	stateDir := filepath.Dir(file)
	return Config{RootDir: root, ConfigDir: stateDir, ConfigFile: file, ProjectName: fc.ProjectName, Mode: fc.Mode, Source: src, SourceType: src.Type, SourceFolder: src.Folder, SourceURL: src.URL, SourceRepository: src.Repository, DownloadDir: src.Folder, ReleaseRoot: rel, CurrentDir: cur, BackupRoot: back, KeepBackups: keepB, KeepReleases: keepR, KeepRsyncOnSetupError: keepRsyncOnSetupError, NoParameterActions: append([]string(nil), fc.NoParameter...), LegacySetupCommands: legacy, Preserve: pres, Security: sec, Docker: docker, Healthcheck: hc, HistoryFile: filepath.Join(stateDir, "history.jsonl"), TemplatesFile: filepath.Join(stateDir, TemplatesFileName), GlobalConfigDir: gdir, GlobalConfigFile: globalFile, GlobalTemplatesFile: filepath.Join(gdir, TemplatesFileName), RepositoryCacheDir: filepath.Join(stateDir, "repository")}, nil
}

// ApplyProjectOverrides applies project-versioned update-cli.yaml settings to
// an already merged global+local runtime configuration. The manifest has higher
// priority than config.json, while command-line source overrides are applied
// afterwards by WithSourceOverrides and therefore remain the highest priority.
func ApplyProjectOverrides(c Config, o ProjectOverrides) (Config, error) {
	if strings.TrimSpace(o.ProjectName) != "" {
		if err := validateProjectName(strings.TrimSpace(o.ProjectName)); err != nil {
			return c, fmt.Errorf("project.slug in update-cli.yaml ist ungültig: %w", err)
		}
		c.ProjectName = strings.TrimSpace(o.ProjectName)
	}
	if o.Mode != nil {
		c.Mode = strings.ToLower(strings.TrimSpace(*o.Mode))
	}
	if o.Source != nil {
		src := c.Source
		if value := strings.TrimSpace(o.Source.Type); value != "" {
			src.Type = strings.ToLower(value)
		}
		if value := strings.TrimSpace(o.Source.Folder); value != "" {
			expanded, err := expandAndAbs(value)
			if err != nil {
				return c, fmt.Errorf("update.source.folder: %w", err)
			}
			src.Folder = expanded
		}
		if value := strings.TrimSpace(o.Source.URL); value != "" {
			src.URL = value
		}
		if value := strings.TrimSpace(o.Source.Repository); value != "" {
			src.Repository = value
		}
		if value := strings.TrimSpace(o.Source.Ref); value != "" {
			src.Ref = value
		}
		if value := strings.TrimSpace(o.Source.Commit); value != "" {
			src.Commit = value
		}
		if value := strings.TrimSpace(o.Source.Version); value != "" {
			src.Version = value
		}
		if value := strings.TrimSpace(o.Source.SHA256); value != "" {
			src.SHA256 = value
		}
		c.Source = src
	}
	if o.ReleaseDir != nil {
		resolved, err := resolveProjectDirectory(c.RootDir, *o.ReleaseDir, "update.releaseDir")
		if err != nil {
			return c, err
		}
		c.ReleaseRoot = resolved
	}
	if o.CurrentDir != nil {
		resolved, err := resolveProjectDirectory(c.RootDir, *o.CurrentDir, "update.currentDir")
		if err != nil {
			return c, err
		}
		c.CurrentDir = resolved
	}
	if o.BackupDirectory != nil {
		resolved, err := resolveProjectDirectory(c.RootDir, *o.BackupDirectory, "update.backup.directory")
		if err != nil {
			return c, err
		}
		c.BackupRoot = resolved
	}
	if o.BackupKeep != nil {
		if *o.BackupKeep < 0 {
			return c, errors.New("update.backup.keep darf nicht negativ sein")
		}
		c.KeepBackups = *o.BackupKeep
	}
	if o.RetentionReleases != nil {
		if *o.RetentionReleases < 0 {
			return c, errors.New("update.retention.releases darf nicht negativ sein")
		}
		c.KeepReleases = *o.RetentionReleases
	}
	if o.Preserve != nil {
		preserve := append([]string(nil), (*o.Preserve)...)
		for i, value := range preserve {
			value = strings.TrimSpace(value)
			if value == "" || filepath.IsAbs(value) || value == ".." || strings.HasPrefix(filepath.Clean(value), ".."+string(os.PathSeparator)) {
				return c, fmt.Errorf("update.sync.preserve[%d] ist unsicher: %q", i, value)
			}
		}
		c.Preserve = ensurePreserve(preserve, ".gitignore")
	}
	if o.KeepRsyncOnError != nil {
		c.KeepRsyncOnSetupError = *o.KeepRsyncOnError
	}
	if o.DockerLifecycle != nil {
		value := strings.ToLower(strings.TrimSpace(*o.DockerLifecycle))
		switch value {
		case "auto", "disabled", "required":
			c.Docker.Lifecycle = value
		default:
			return c, fmt.Errorf("update.docker.lifecycle ungültig: %q; erlaubt: auto, disabled, required", value)
		}
	}
	if o.HealthcheckType != nil {
		c.Healthcheck.Type = strings.ToLower(strings.TrimSpace(*o.HealthcheckType))
	}
	if o.HealthcheckURL != nil {
		c.Healthcheck.URL = strings.TrimSpace(*o.HealthcheckURL)
	}
	if o.HealthcheckCommand != nil {
		c.Healthcheck.Command = strings.TrimSpace(*o.HealthcheckCommand)
	}
	if o.HealthcheckTimeout != nil {
		if *o.HealthcheckTimeout < 0 {
			return c, errors.New("update.healthcheck.timeoutSeconds darf nicht negativ sein")
		}
		c.Healthcheck.TimeoutSeconds = *o.HealthcheckTimeout
	}
	if samePath(c.ReleaseRoot, c.CurrentDir) || samePath(c.BackupRoot, c.CurrentDir) || samePath(c.BackupRoot, c.ReleaseRoot) {
		return c, errors.New("releaseDir, currentDir und backup.directory müssen verschieden sein")
	}
	for _, path := range []string{c.ReleaseRoot, c.CurrentDir, c.BackupRoot} {
		if _, err := tools.CanonicalInside(c.RootDir, path, true); err != nil {
			return c, err
		}
	}
	if c.Mode != ModeUpdate && c.Mode != ModePull {
		return c, fmt.Errorf("update.mode ungültig: %s; erlaubt: update, pull", c.Mode)
	}
	if c.Mode == ModePull && c.Source.Type != "repository" {
		return c, errors.New("update.mode pull benötigt source.type repository")
	}
	if c.Mode == ModeUpdate && c.Source.Type == "repository" {
		return c, errors.New("update.mode update erwartet eine ZIP-Quelle; für Git-Repositories mode pull verwenden")
	}
	switch c.Source.Type {
	case "download":
		if strings.TrimSpace(c.Source.Folder) == "" {
			return c, errors.New("update.source.folder fehlt")
		}
	case "url":
		if strings.TrimSpace(c.Source.URL) == "" {
			return c, errors.New("update.source.url fehlt")
		}
	case "repository":
		if strings.TrimSpace(c.Source.Repository) == "" {
			return c, errors.New("update.source.repository fehlt")
		}
	default:
		return c, fmt.Errorf("update.source.type ungültig: %s", c.Source.Type)
	}
	c.SourceType = c.Source.Type
	c.SourceFolder = c.Source.Folder
	c.SourceURL = c.Source.URL
	c.SourceRepository = c.Source.Repository
	c.DownloadDir = c.Source.Folder
	switch c.Healthcheck.Type {
	case "", "none":
	case "http":
		if c.Healthcheck.URL == "" {
			return c, errors.New("update.healthcheck.url fehlt")
		}
	case "command":
		if c.Healthcheck.Command == "" {
			return c, errors.New("update.healthcheck.command fehlt")
		}
	default:
		return c, fmt.Errorf("update.healthcheck.type ungültig: %s", c.Healthcheck.Type)
	}
	return c, nil
}

func WithSourceOverrides(c Config, mode, kind, folder, u, repo string) (Config, error) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	kind = strings.TrimSpace(strings.ToLower(kind))
	provided := 0
	if folder != "" {
		provided++
		if kind == "" {
			kind = "download"
		}
	}
	if u != "" {
		provided++
		if kind == "" {
			kind = "url"
		}
	}
	if repo != "" {
		provided++
		if kind == "" {
			kind = "repository"
		}
	}
	if provided > 1 {
		return c, errors.New("--folder, --url und --repository schließen sich gegenseitig aus")
	}
	if mode == "" {
		if kind == "repository" || repo != "" {
			mode = ModePull
		} else if kind == "download" || kind == "url" || folder != "" || u != "" {
			mode = ModeUpdate
		} else {
			mode = c.Mode
		}
	}
	if mode != ModeUpdate && mode != ModePull {
		return c, fmt.Errorf("--mode muss update oder pull sein")
	}
	if kind == "" {
		kind = c.Source.Type
	}
	if mode == ModePull && kind != "repository" {
		return c, errors.New("mode pull benötigt --from repository/--repository")
	}
	if mode == ModeUpdate && kind == "repository" {
		return c, errors.New("mode update erwartet eine ZIP-Quelle; für Git-Repositories --mode pull verwenden")
	}
	switch kind {
	case "download":
		if folder != "" {
			p, err := expandAndAbs(folder)
			if err != nil {
				return c, err
			}
			c.Source.Folder = p
		}
		if c.Source.Folder == "" {
			return c, errors.New("Download-Quelle benötigt einen Ordner")
		}
	case "url":
		if u != "" {
			c.Source.URL = u
		}
		if c.Source.URL == "" {
			return c, errors.New("URL-Quelle benötigt --url")
		}
	case "repository":
		if repo != "" {
			c.Source.Repository = repo
		}
		if c.Source.Repository == "" {
			return c, errors.New("Repository-Quelle benötigt --repository")
		}
	default:
		return c, fmt.Errorf("--from muss download, url oder repository sein")
	}
	c.Mode = mode
	c.Source.Type = kind
	c.SourceType = kind
	c.SourceFolder = c.Source.Folder
	c.SourceURL = c.Source.URL
	c.SourceRepository = c.Source.Repository
	c.DownloadDir = c.Source.Folder
	return c, nil
}
func Check(root string) (CheckResult, error) {
	root, err := absoluteDir(root)
	if err != nil {
		return CheckResult{}, err
	}
	file, _ := configFilePath(root)
	orig, err := readConfigFile(root, file)
	if err != nil {
		return CheckResult{ConfigFile: file, CurrentSchema: SchemaVersion}, err
	}
	up, _, err := migrate(orig)
	if err != nil {
		return CheckResult{ConfigFile: file, SchemaVersion: orig.SchemaVersion, CurrentSchema: SchemaVersion}, err
	}
	migrationNeeded := orig.SchemaVersion < SchemaVersion
	if err := validateFile(up); err != nil {
		return CheckResult{ConfigFile: file, SchemaVersion: orig.SchemaVersion, CurrentSchema: SchemaVersion, MigrationNeeded: migrationNeeded, ProjectName: up.ProjectName, Mode: up.Mode, SourceType: up.Source.Type}, err
	}
	// Resolve all project paths and the merged global/local runtime-facing values
	// as Load would, but do not write anything. Missing optional local defaults
	// are intentional inheritance, not a schema migration.
	if _, err := Load(root, ""); err != nil {
		return CheckResult{ConfigFile: file, SchemaVersion: orig.SchemaVersion, CurrentSchema: SchemaVersion, MigrationNeeded: migrationNeeded, ProjectName: up.ProjectName, Mode: up.Mode, SourceType: up.Source.Type}, err
	}
	return CheckResult{ConfigFile: file, SchemaVersion: orig.SchemaVersion, CurrentSchema: SchemaVersion, MigrationNeeded: migrationNeeded, ProjectName: up.ProjectName, Mode: up.Mode, SourceType: up.Source.Type, Valid: true}, nil
}

func Upgrade(root string) (UpgradeResult, error) {
	root, err := absoluteDir(root)
	if err != nil {
		return UpgradeResult{}, err
	}
	sourceFile, _ := configFilePath(root)
	orig, err := readConfigFile(root, sourceFile)
	if err != nil {
		return UpgradeResult{}, err
	}
	up, changed, err := migrate(orig)
	if err != nil {
		return UpgradeResult{}, err
	}
	if err := validateFile(up); err != nil {
		return UpgradeResult{}, err
	}
	targetFile := filepath.Join(root, ConfigDirName, ConfigFileName)
	res := UpgradeResult{ConfigFile: targetFile, PreviousSchema: orig.SchemaVersion, CurrentSchema: SchemaVersion, Changed: changed, ProjectName: up.ProjectName}
	if !res.Changed {
		return res, nil
	}
	bak, err := backupConfigFile(sourceFile, orig.SchemaVersion)
	if err != nil {
		return res, err
	}
	res.BackupFile = bak
	if err := writeConfigFile(targetFile, up); err != nil {
		return res, err
	}
	if _, err := Load(root, ""); err != nil {
		return res, fmt.Errorf("aktualisierte config.json ist ungültig: %w", err)
	}
	return res, nil
}
func Format(root string) (string, string, error) {
	root, err := absoluteDir(root)
	if err != nil {
		return "", "", err
	}
	file, _ := configFilePath(root)
	gdir, err := GlobalConfigDir()
	if err != nil {
		return "", "", err
	}
	fc, err := readMergedConfigFile(file, filepath.Join(gdir, ConfigFileName))
	if err != nil {
		return "", "", err
	}
	fc, _, err = migrate(fc)
	if err != nil {
		return "", "", err
	}
	if err := validateFile(fc); err != nil {
		return "", "", err
	}
	b, err := marshalPretty(fc)
	return file, string(b), err
}

func readConfigFile(root, path string) (FileConfig, error) {
	object, err := readJSONObject(path, true)
	if err != nil {
		return FileConfig{}, err
	}
	normalizeLegacySetupPolicy(object)
	data, err := json.Marshal(object)
	if err != nil {
		return FileConfig{}, err
	}
	fc, err := decodeFileConfig(data)
	if err != nil {
		return fc, err
	}
	return fc, nil
}
func migrate(v FileConfig) (FileConfig, bool, error) {
	orig, _ := marshalPretty(v)
	legacyDefaultUser := strings.TrimSpace(v.DefaultUser)
	if v.SchemaVersion < 1 {
		return v, false, fmt.Errorf("schemaVersion fehlt oder ist ungültig: %d", v.SchemaVersion)
	}
	if v.SchemaVersion > SchemaVersion {
		return v, false, fmt.Errorf("schemaVersion %d ist neuer als unterstützt %d", v.SchemaVersion, SchemaVersion)
	}
	v.SchemaVersion = SchemaVersion
	if v.Source == nil {
		folder := strings.TrimSpace(v.DownloadDir)
		if folder == "" {
			folder = buildconfig.Current().DefaultDownloadFolder
		}
		v.Source = &SourceConfig{Type: "download", Folder: folder}
	}
	if v.Source.Type == "" {
		v.Source.Type = "download"
	}
	v.Source.Type = strings.ToLower(strings.TrimSpace(v.Source.Type))
	if strings.TrimSpace(v.Source.DefaultUser) == "" {
		if legacyDefaultUser != "" {
			v.Source.DefaultUser = legacyDefaultUser
		} else {
			v.Source.DefaultUser = DefaultRepositoryUser
		}
	}
	// Canonical form is source.defaultUser. Keep reading the historical
	// top-level alias, but never write it back after migration.
	v.DefaultUser = ""
	if strings.TrimSpace(v.Mode) == "" {
		if v.Source.Type == "repository" {
			v.Mode = ModePull
		} else {
			v.Mode = ModeUpdate
		}
	}
	v.Mode = strings.ToLower(strings.TrimSpace(v.Mode))
	v.DownloadDir = ""
	if v.ReleaseDir == "" {
		v.ReleaseDir = "release"
	}
	if v.CurrentDir == "" {
		v.CurrentDir = "current"
	}
	// Canonical key is "no parameter". Historical/user-facing aliases are
	// accepted so a parameterless invocation cannot silently fall back to
	// check/prompt semantics just because the JSON key used a different spelling.
	if len(v.NoParameter) == 0 {
		for _, legacy := range []NoParameterConfig{v.NoParamLegacy, v.NoParameterLegacy, v.NoParameterCamel} {
			if len(legacy) > 0 {
				v.NoParameter = append(NoParameterConfig(nil), legacy...)
				break
			}
		}
	}
	v.NoParamLegacy = nil
	v.NoParameterLegacy = nil
	v.NoParameterCamel = nil
	acts, err := normalizedNoParameter(v.NoParameter)
	if err != nil {
		return v, false, err
	}
	v.NoParameter = acts
	if v.Setup == nil {
		v.Setup = &SetupConfig{Commands: []string{}}
	}
	if v.Backup == nil {
		v.Backup = &BackupConfig{Directory: "backup", Keep: 3}
	}
	if v.Backup.Directory == "" {
		v.Backup.Directory = "backup"
	}
	if v.Retention == nil {
		v.Retention = &RetentionConfig{Releases: 5}
	}
	if v.Sync == nil {
		v.Sync = &SyncConfig{Preserve: defaultPreserve()}
	}
	if len(v.Sync.Preserve) == 0 {
		v.Sync.Preserve = defaultPreserve()
	} else {
		v.Sync.Preserve = ensurePreserve(v.Sync.Preserve, ".gitignore")
	}
	if v.Security == nil {
		v.Security = ptrSecurity(defaultSecurity())
	} else {
		d := defaultSecurity()
		if v.Security.MaxArchiveBytes == 0 {
			v.Security.MaxArchiveBytes = d.MaxArchiveBytes
		}
		if v.Security.MaxUncompressedBytes == 0 {
			v.Security.MaxUncompressedBytes = d.MaxUncompressedBytes
		}
		if v.Security.MaxFileBytes == 0 {
			v.Security.MaxFileBytes = d.MaxFileBytes
		}
		if v.Security.MaxEntries == 0 {
			v.Security.MaxEntries = d.MaxEntries
		}
		if v.Security.MaxCompressionRatio == 0 {
			v.Security.MaxCompressionRatio = d.MaxCompressionRatio
		}
	}
	if v.Docker == nil {
		v.Docker = &DockerConfig{Lifecycle: "auto"}
	}
	if strings.TrimSpace(v.Docker.Lifecycle) == "" {
		v.Docker.Lifecycle = "auto"
	}
	v.Docker.Lifecycle = strings.ToLower(strings.TrimSpace(v.Docker.Lifecycle))
	if v.Healthcheck == nil {
		v.Healthcheck = &HealthcheckConfig{}
	}
	now, _ := marshalPretty(v)
	return v, !bytes.Equal(orig, now), nil
}
func validateFile(v FileConfig) error {
	if v.SchemaVersion != SchemaVersion {
		return fmt.Errorf("nicht unterstützte schemaVersion %d; update-cli upgrade", v.SchemaVersion)
	}
	if err := validateProjectName(v.ProjectName); err != nil {
		return err
	}
	if v.Source == nil {
		return errors.New("source fehlt")
	}
	if err := validateRepositoryUser(v.Source.DefaultUser); err != nil {
		return err
	}
	switch v.Mode {
	case ModeUpdate, ModePull:
	default:
		return fmt.Errorf("mode ungültig: %s; erlaubt: update, pull", v.Mode)
	}
	if v.Mode == ModePull && v.Source.Type != "repository" {
		return errors.New("mode pull benötigt source.type repository")
	}
	if v.Mode == ModeUpdate && v.Source.Type == "repository" {
		return errors.New("mode update erwartet eine ZIP-Quelle (download oder url); für Git-Repositories mode pull verwenden")
	}
	switch v.Source.Type {
	case "download":
		if strings.TrimSpace(v.Source.Folder) == "" {
			return errors.New("source.folder fehlt")
		}
	case "url":
		if strings.TrimSpace(v.Source.URL) == "" {
			return errors.New("source.url fehlt")
		}
	case "repository":
		if strings.TrimSpace(v.Source.Repository) == "" {
			return errors.New("source.repository fehlt")
		}
	default:
		return fmt.Errorf("source.type ungültig: %s", v.Source.Type)
	}
	if v.Backup != nil && v.Backup.Keep < 0 {
		return errors.New("backup.keep darf nicht negativ sein")
	}
	if v.Retention != nil && v.Retention.Releases < 0 {
		return errors.New("retention.releases darf nicht negativ sein")
	}
	if v.Sync == nil {
		return errors.New("sync fehlt")
	}
	for i, p := range v.Sync.Preserve {
		p = strings.TrimSpace(p)
		if p == "" || filepath.IsAbs(p) || p == ".." || strings.HasPrefix(filepath.Clean(p), ".."+string(os.PathSeparator)) {
			return fmt.Errorf("sync.preserve[%d] ist unsicher: %q", i, p)
		}
	}
	s := v.Security
	if s == nil {
		return errors.New("security fehlt")
	}
	if s.MaxArchiveBytes <= 0 || s.MaxUncompressedBytes <= 0 || s.MaxFileBytes <= 0 || s.MaxEntries <= 0 || s.MaxCompressionRatio <= 0 {
		return errors.New("security limits müssen > 0 sein")
	}
	if v.Source.Type == "url" && strings.HasPrefix(strings.ToLower(v.Source.URL), "http://") && !s.AllowHTTP {
		return errors.New("unsichere HTTP-Quelle ist deaktiviert; security.allowHttp=true wäre erforderlich")
	}
	if v.Docker != nil {
		switch strings.ToLower(strings.TrimSpace(v.Docker.Lifecycle)) {
		case "auto", "disabled", "required":
		default:
			return fmt.Errorf("ungültiger Docker-Lifecycle %q; erlaubt: auto, disabled, required", v.Docker.Lifecycle)
		}
	}
	if v.Healthcheck != nil {
		switch v.Healthcheck.Type {
		case "", "none", "http", "command":
		default:
			return fmt.Errorf("healthcheck.type ungültig: %s", v.Healthcheck.Type)
		}
		if v.Healthcheck.Type == "http" && v.Healthcheck.URL == "" {
			return errors.New("healthcheck.url fehlt")
		}
		if v.Healthcheck.Type == "command" && v.Healthcheck.Command == "" {
			return errors.New("healthcheck.command fehlt")
		}
	}
	return nil
}
func normalizedNoParameter(v NoParameterConfig) (NoParameterConfig, error) {
	if len(v) == 0 {
		return NoParameterConfig{"help"}, nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, a := range v {
		a = strings.ToLower(strings.TrimSpace(a))
		if seen[a] {
			return nil, fmt.Errorf("no parameter enthält %q mehrfach", a)
		}
		seen[a] = true
		switch a {
		case "help", "check", "update", "setup", "no-setup":
			out = append(out, a)
		default:
			return nil, fmt.Errorf("no parameter unterstützt nur help, check, update, setup, no-setup")
		}
	}
	if seen["help"] && len(out) > 1 {
		return nil, errors.New("help darf nicht kombiniert werden")
	}
	if seen["check"] && seen["update"] {
		return nil, errors.New("check und update dürfen nicht kombiniert werden")
	}
	if seen["no-setup"] && !seen["update"] {
		return nil, errors.New("no-setup darf in no parameter nur mit update kombiniert werden")
	}
	if seen["setup"] && seen["no-setup"] {
		return nil, errors.New("setup und no-setup schließen sich in no parameter aus")
	}
	// Historical semantics: ["check", "setup"] means check for an update and,
	// after the user confirms installation, run project setup as part of that
	// update. setup is therefore a modifier of check here, not a second CLI mode.
	ordered := []string{}
	for _, a := range []string{"help", "check", "update", "setup", "no-setup"} {
		if seen[a] {
			ordered = append(ordered, a)
		}
	}
	return ordered, nil
}
func resolveProjectDirectory(root, configured, field string) (string, error) {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return "", fmt.Errorf("%s fehlt", field)
	}
	if filepath.IsAbs(configured) {
		return "", fmt.Errorf("%s muss relativ sein", field)
	}
	clean := filepath.Clean(configured)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("%s verweist außerhalb des Projekts", field)
	}
	if clean == ConfigDirName || strings.HasPrefix(clean, ConfigDirName+string(os.PathSeparator)) {
		return "", fmt.Errorf("%s darf nicht in %s liegen", field, ConfigDirName)
	}
	return filepath.Join(root, clean), nil
}
func applySource(v *FileConfig, mode, kind, folder, u, repo string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	kind = strings.ToLower(strings.TrimSpace(kind))
	if mode == "" {
		switch {
		case kind == "repository" || repo != "":
			mode = ModePull
		default:
			mode = ModeUpdate
		}
	}
	if mode != ModeUpdate && mode != ModePull {
		return fmt.Errorf("unbekannter mode: %s; erlaubt: update, pull", mode)
	}
	if kind == "" {
		if folder != "" {
			kind = "download"
		} else if u != "" {
			kind = "url"
		} else if repo != "" {
			kind = "repository"
		} else if mode == ModePull {
			kind = "repository"
		} else {
			v.Mode = mode
			return nil
		}
	}
	if mode == ModePull && kind != "repository" {
		return errors.New("mode pull benötigt eine Repository-Quelle")
	}
	if mode == ModeUpdate && kind == "repository" {
		return errors.New("mode update erwartet eine ZIP-Quelle; für Git-Repositories mode pull verwenden")
	}
	switch kind {
	case "download":
		if folder != "" {
			v.Source = &SourceConfig{Type: kind, Folder: folder, DefaultUser: sourceDefaultUser(v)}
		}
	case "url":
		if u == "" {
			return errors.New("URL-Quelle benötigt --url")
		}
		v.Source = &SourceConfig{Type: kind, URL: u, DefaultUser: sourceDefaultUser(v)}
	case "repository":
		if repo == "" {
			return errors.New("Repository-Quelle benötigt --repository")
		}
		v.Source = &SourceConfig{Type: kind, Repository: repo, DefaultUser: sourceDefaultUser(v)}
	default:
		return fmt.Errorf("unbekannte Quelle: %s", kind)
	}
	v.Mode = mode
	return nil
}
func sourceDefaultUser(v *FileConfig) string {
	if v != nil && v.Source != nil && strings.TrimSpace(v.Source.DefaultUser) != "" {
		return strings.TrimSpace(v.Source.DefaultUser)
	}
	return DefaultRepositoryUser
}

var repositoryUserPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)
var repositoryNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func validateRepositoryUser(user string) error {
	user = strings.TrimSpace(user)
	if user == "" {
		return errors.New("source.defaultUser fehlt")
	}
	if !repositoryUserPattern.MatchString(user) {
		return fmt.Errorf("source.defaultUser %q ist kein gültiger GitHub-Benutzername", user)
	}
	return nil
}

// NormalizeRepositorySpec accepts a complete GitHub URL, user/repository or
// only repository. Short forms are expanded to an HTTPS GitHub URL. The
// one-part form uses defaultUser from .update-cli/config.json.
func NormalizeRepositorySpec(spec, defaultUser string) (string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", errors.New("--from-repository benötigt REPOSITORY")
	}
	if filepath.IsAbs(spec) {
		return filepath.Clean(spec), nil
	}

	defaultUser = strings.TrimSpace(defaultUser)
	if defaultUser == "" {
		defaultUser = DefaultRepositoryUser
	}
	if err := validateRepositoryUser(defaultUser); err != nil {
		return "", err
	}

	lower := strings.ToLower(spec)
	if strings.HasPrefix(lower, "https://github.com/") || strings.HasPrefix(lower, "http://github.com/") {
		if strings.HasPrefix(lower, "http://") {
			return "", errors.New("--from-repository akzeptiert GitHub-URLs nur über https")
		}
		path := strings.TrimPrefix(spec, "https://github.com/")
		path = strings.Trim(path, "/")
		parts := strings.Split(path, "/")
		if len(parts) != 2 {
			return "", fmt.Errorf("ungültiges GitHub-Repository %q; erwartet https://github.com/USER/REPO", spec)
		}
		return githubRepositoryURL(parts[0], parts[1])
	}
	if strings.Contains(spec, "://") {
		return "", fmt.Errorf("--from-repository unterstützt als URL nur https://github.com/USER/REPO: %q", spec)
	}

	parts := strings.Split(strings.Trim(spec, "/"), "/")
	switch len(parts) {
	case 1:
		return githubRepositoryURL(defaultUser, parts[0])
	case 2:
		return githubRepositoryURL(parts[0], parts[1])
	default:
		return "", fmt.Errorf("ungültiges Repository %q; erwartet REPO, USER/REPO oder https://github.com/USER/REPO", spec)
	}
}

func githubRepositoryURL(user, repository string) (string, error) {
	user = strings.TrimSpace(user)
	repository = strings.TrimSuffix(strings.TrimSpace(repository), ".git")
	if err := validateRepositoryUser(user); err != nil {
		return "", err
	}
	if repository == "" || !repositoryNamePattern.MatchString(repository) {
		return "", fmt.Errorf("ungültiger Repository-Name %q", repository)
	}
	return "https://github.com/" + user + "/" + repository + ".git", nil
}

func validateProjectName(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errors.New("projectName fehlt oder ist leer")
	}
	if !projectNamePattern.MatchString(s) {
		return fmt.Errorf("ungültiger projectName %q", s)
	}
	return nil
}
func absoluteDir(p string) (string, error) {
	p, err := expandAndAbs(p)
	if err != nil {
		return "", err
	}
	i, err := os.Stat(p)
	if err != nil {
		return "", fmt.Errorf("Projektordner ist nicht verfügbar: %w", err)
	}
	if !i.IsDir() {
		return "", fmt.Errorf("Projektpfad ist kein Ordner: %s", p)
	}
	return p, nil
}
func expandAndAbs(p string) (string, error) {
	p = os.ExpandEnv(strings.TrimSpace(p))
	if p == "" {
		return "", errors.New("leerer Pfad")
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(h, strings.TrimPrefix(p, "~/"))
	}
	return filepath.Abs(p)
}
func hasProjectConfiguration(p string) bool {
	i, err := os.Stat(filepath.Join(p, ConfigDirName, ConfigFileName))
	return err == nil && !i.IsDir()
}
func samePath(a, b string) bool {
	aa, _ := filepath.Abs(a)
	bb, _ := filepath.Abs(b)
	return filepath.Clean(aa) == filepath.Clean(bb)
}
func marshalPretty(v FileConfig) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func writeConfigFile(path string, v FileConfig) error {
	b, err := marshalPretty(v)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".config-*.json")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	_ = f.Chmod(0o644)
	if _, err := f.Write(b); err != nil {
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
func backupConfigFile(path string, schema int) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	base := fmt.Sprintf("%s.backup-v%d-%s", path, schema, time.Now().Format("20060102-150405"))
	for i := 0; ; i++ {
		p := base
		if i > 0 {
			p = fmt.Sprintf("%s-%d", base, i+1)
		}
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if _, err := f.Write(b); err != nil {
			f.Close()
			os.Remove(p)
			return "", err
		}
		if err := f.Sync(); err != nil {
			f.Close()
			os.Remove(p)
			return "", err
		}
		if err := f.Close(); err != nil {
			return "", err
		}
		return p, nil
	}
}

// ApplyTemplate updates reusable project defaults in the active JSON config.
// update-cli.yaml/setup.yaml remain setup automation manifests and are not
// rewritten by template application.
func ApplyTemplate(root string, noParameter, preserve []string) error {
	root, err := absoluteDir(root)
	if err != nil {
		return err
	}
	file, _ := configFilePath(root)
	fc, err := readConfigFile(root, file)
	if err != nil {
		return err
	}
	fc, _, err = migrate(fc)
	if err != nil {
		return err
	}
	if len(noParameter) > 0 {
		fc.NoParameter = NoParameterConfig(append([]string(nil), noParameter...))
	}
	if len(preserve) > 0 {
		fc.Sync = &SyncConfig{Preserve: append([]string(nil), preserve...)}
	}
	fc, _, err = migrate(fc)
	if err != nil {
		return err
	}
	if err := validateFile(fc); err != nil {
		return err
	}
	return writeConfigFile(file, fc)
}
