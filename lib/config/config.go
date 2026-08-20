package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/r14r/update-cli/lib/buildconfig"
	"github.com/r14r/update-cli/lib/tools"
)

const (
	ConfigDirName       = ".updater-cli"
	LegacyConfigDirName = ".update-cli"
	ConfigFileName      = "config.json"
	TemplatesFileName   = "templates.json"
	SchemaVersion       = 7
	ModeUpdate          = "update"
	ModePull            = "pull"
)

var projectNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type SourceConfig struct {
	Type       string `json:"type"`
	Folder     string `json:"folder,omitempty"`
	URL        string `json:"url,omitempty"`
	Repository string `json:"repository,omitempty"`
	Ref        string `json:"ref,omitempty"`
	Commit     string `json:"commit,omitempty"`
	Version    string `json:"version,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
}
type SetupConfig struct {
	Commands []string `json:"commands"`
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
	SchemaVersion int                `json:"schemaVersion"`
	ProjectName   string             `json:"projectName"`
	Mode          string             `json:"mode,omitempty"`
	DownloadDir   string             `json:"downloadDir,omitempty"`
	Source        *SourceConfig      `json:"source,omitempty"`
	ReleaseDir    string             `json:"releaseDir"`
	CurrentDir    string             `json:"currentDir"`
	NoParameter   NoParameterConfig  `json:"no parameter,omitempty"`
	Setup         *SetupConfig       `json:"setup,omitempty"`
	Backup        *BackupConfig      `json:"backup,omitempty"`
	Retention     *RetentionConfig   `json:"retention,omitempty"`
	Sync          *SyncConfig        `json:"sync,omitempty"`
	Security      *SecurityConfig    `json:"security,omitempty"`
	Docker        *DockerConfig      `json:"docker,omitempty"`
	Healthcheck   *HealthcheckConfig `json:"healthcheck,omitempty"`
}

type Config struct {
	RootDir             string            `json:"rootDir"`
	ConfigDir           string            `json:"configDir"`
	ConfigFile          string            `json:"configFile"`
	ProjectName         string            `json:"projectName"`
	Mode                string            `json:"mode"`
	Source              SourceConfig      `json:"source"`
	SourceType          string            `json:"sourceType"`
	SourceFolder        string            `json:"sourceFolder,omitempty"`
	SourceURL           string            `json:"sourceUrl,omitempty"`
	SourceRepository    string            `json:"sourceRepository,omitempty"`
	DownloadDir         string            `json:"downloadDir,omitempty"`
	ReleaseRoot         string            `json:"releaseDir"`
	CurrentDir          string            `json:"currentDir"`
	BackupRoot          string            `json:"backupDir"`
	KeepBackups         int               `json:"keepBackups"`
	KeepReleases        int               `json:"keepReleases"`
	NoParameterActions  []string          `json:"noParameterActions"`
	LegacySetupCommands []string          `json:"legacySetupCommands,omitempty"`
	Preserve            []string          `json:"preserve"`
	Security            SecurityConfig    `json:"security"`
	Docker              DockerConfig      `json:"docker"`
	Healthcheck         HealthcheckConfig `json:"healthcheck"`
	HistoryFile         string            `json:"historyFile"`
	TemplatesFile       string            `json:"templatesFile"`
	GlobalConfigDir     string            `json:"globalConfigDir"`
	GlobalTemplatesFile string            `json:"globalTemplatesFile"`
	RepositoryCacheDir  string            `json:"repositoryCacheDir"`
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
	return FileConfig{SchemaVersion: SchemaVersion, ProjectName: project, Mode: ModeUpdate, Source: &SourceConfig{Type: "download", Folder: buildconfig.Current().DefaultDownloadFolder}, ReleaseDir: "release", CurrentDir: "current", NoParameter: NoParameterConfig{"check"}, Setup: &SetupConfig{Commands: []string{}}, Backup: &BackupConfig{Directory: "backup", Keep: 3}, Retention: &RetentionConfig{Releases: 5}, Sync: &SyncConfig{Preserve: defaultPreserve()}, Security: ptrSecurity(defaultSecurity()), Docker: &DockerConfig{Lifecycle: "auto"}, Healthcheck: &HealthcheckConfig{}}
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
// .updater-cli/config.json. This lets commands be executed from current/ or
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
func Load(root, downloadOverride string) (Config, error) {
	root, err := absoluteDir(root)
	if err != nil {
		return Config{}, err
	}
	file := filepath.Join(root, ConfigDirName, ConfigFileName)
	fc, err := readConfigFile(root, file)
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
	gdir, err := buildconfig.ExpandPath(buildconfig.Current().DefaultConfigPath)
	if err != nil {
		return Config{}, err
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
	if fc.Setup != nil {
		legacy = append(legacy, fc.Setup.Commands...)
	}
	return Config{RootDir: root, ConfigDir: filepath.Join(root, ConfigDirName), ConfigFile: file, ProjectName: fc.ProjectName, Mode: fc.Mode, Source: src, SourceType: src.Type, SourceFolder: src.Folder, SourceURL: src.URL, SourceRepository: src.Repository, DownloadDir: src.Folder, ReleaseRoot: rel, CurrentDir: cur, BackupRoot: back, KeepBackups: keepB, KeepReleases: keepR, NoParameterActions: append([]string(nil), fc.NoParameter...), LegacySetupCommands: legacy, Preserve: pres, Security: sec, Docker: docker, Healthcheck: hc, HistoryFile: filepath.Join(root, ConfigDirName, "history.jsonl"), TemplatesFile: filepath.Join(root, ConfigDirName, TemplatesFileName), GlobalConfigDir: gdir, GlobalTemplatesFile: filepath.Join(gdir, TemplatesFileName), RepositoryCacheDir: filepath.Join(root, ConfigDirName, "repository")}, nil
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
	file := filepath.Join(root, ConfigDirName, ConfigFileName)
	orig, err := readConfigFile(root, file)
	if err != nil {
		return CheckResult{ConfigFile: file, CurrentSchema: SchemaVersion}, err
	}
	up, changed, err := migrate(orig)
	if err != nil {
		return CheckResult{ConfigFile: file, SchemaVersion: orig.SchemaVersion, CurrentSchema: SchemaVersion}, err
	}
	if err := validateFile(up); err != nil {
		return CheckResult{ConfigFile: file, SchemaVersion: orig.SchemaVersion, CurrentSchema: SchemaVersion, MigrationNeeded: changed, ProjectName: up.ProjectName, Mode: up.Mode, SourceType: up.Source.Type}, err
	}
	// Resolve all project paths and runtime-facing values as Load would, but do not write anything.
	if _, err := Load(root, ""); err != nil {
		return CheckResult{ConfigFile: file, SchemaVersion: orig.SchemaVersion, CurrentSchema: SchemaVersion, MigrationNeeded: changed, ProjectName: up.ProjectName, Mode: up.Mode, SourceType: up.Source.Type}, err
	}
	return CheckResult{ConfigFile: file, SchemaVersion: orig.SchemaVersion, CurrentSchema: SchemaVersion, MigrationNeeded: changed, ProjectName: up.ProjectName, Mode: up.Mode, SourceType: up.Source.Type, Valid: true}, nil
}

func Upgrade(root string) (UpgradeResult, error) {
	root, err := absoluteDir(root)
	if err != nil {
		return UpgradeResult{}, err
	}
	file := filepath.Join(root, ConfigDirName, ConfigFileName)
	orig, err := readConfigFile(root, file)
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
	res := UpgradeResult{ConfigFile: file, PreviousSchema: orig.SchemaVersion, CurrentSchema: SchemaVersion, Changed: changed, ProjectName: up.ProjectName}
	if !changed {
		return res, nil
	}
	bak, err := backupConfigFile(file, orig.SchemaVersion)
	if err != nil {
		return res, err
	}
	res.BackupFile = bak
	if err := writeConfigFile(file, up); err != nil {
		return res, err
	}
	if _, err := Load(root, ""); err != nil {
		_ = writeConfigFile(file, orig)
		return res, fmt.Errorf("aktualisierte Konfiguration ungültig; vorherige wiederhergestellt: %w", err)
	}
	return res, nil
}
func Format(root string) (string, string, error) {
	root, err := absoluteDir(root)
	if err != nil {
		return "", "", err
	}
	file := filepath.Join(root, ConfigDirName, ConfigFileName)
	fc, err := readConfigFile(root, file)
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
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		legacy := filepath.Join(root, LegacyConfigDirName, ConfigFileName)
		if _, e := os.Stat(legacy); e == nil {
			return FileConfig{}, fmt.Errorf("veraltete Konfiguration gefunden: %s; update-cli --init verwenden", legacy)
		}
		return FileConfig{}, fmt.Errorf("Updater-Konfiguration fehlt: %s", path)
	}
	if err != nil {
		return FileConfig{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var fc FileConfig
	if err := dec.Decode(&fc); err != nil {
		return fc, fmt.Errorf("config.json enthält ungültiges JSON: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return fc, err
		}
		return fc, errors.New("config.json enthält mehrere JSON-Werte")
	}
	return fc, nil
}
func migrate(v FileConfig) (FileConfig, bool, error) {
	orig, _ := marshalPretty(v)
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
		return fmt.Errorf("nicht unterstützte schemaVersion %d; update-cli --upgrade", v.SchemaVersion)
	}
	if err := validateProjectName(v.ProjectName); err != nil {
		return err
	}
	if v.Source == nil {
		return errors.New("source fehlt")
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
		case "help", "check", "update", "setup":
			out = append(out, a)
		default:
			return nil, fmt.Errorf("no parameter unterstützt nur help, check, update, setup")
		}
	}
	if seen["help"] && len(out) > 1 {
		return nil, errors.New("help darf nicht kombiniert werden")
	}
	if seen["check"] && seen["update"] {
		return nil, errors.New("check und update dürfen nicht kombiniert werden")
	}
	// Historical semantics: ["check", "setup"] means check for an update and,
	// after the user confirms installation, run project setup as part of that
	// update. setup is therefore a modifier of check here, not a second CLI mode.
	ordered := []string{}
	for _, a := range []string{"help", "check", "update", "setup"} {
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
			v.Source = &SourceConfig{Type: kind, Folder: folder}
		}
	case "url":
		if u == "" {
			return errors.New("URL-Quelle benötigt --url")
		}
		v.Source = &SourceConfig{Type: kind, URL: u}
	case "repository":
		if repo == "" {
			return errors.New("Repository-Quelle benötigt --repository")
		}
		v.Source = &SourceConfig{Type: kind, Repository: repo}
	default:
		return fmt.Errorf("unbekannte Quelle: %s", kind)
	}
	v.Mode = mode
	return nil
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
	i, e := os.Stat(filepath.Join(p, ConfigDirName, ConfigFileName))
	return e == nil && !i.IsDir()
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
