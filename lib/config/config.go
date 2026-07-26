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

	"release-updater/lib/buildconfig"
	"release-updater/lib/templates"
)

const (
	ConfigDirName       = ".updater-cli"
	LegacyConfigDirName = ".update-cli"
	ConfigFileName      = "config.json"
	TemplatesFileName   = templates.FileName
	SchemaVersion       = 5
)

var projectNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

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

// SourceConfig defines where release content is obtained. Folder is used for
// local download archives, URL for a direct ZIP link, and Repository for a
// Git repository cloned by update-cli.
type SourceConfig struct {
	Type       string `json:"type"`
	Folder     string `json:"folder,omitempty"`
	URL        string `json:"url,omitempty"`
	Repository string `json:"repository,omitempty"`
}

// NoParameterConfig accepts both the legacy string form and the current
// array form. New configurations are always written as an array.
type NoParameterConfig []string

func (value *NoParameterConfig) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*value = NoParameterConfig{single}
		return nil
	}

	var multiple []string
	if err := json.Unmarshal(data, &multiple); err != nil {
		return errors.New(`"no parameter" muss ein String oder eine Liste von Strings sein`)
	}
	*value = NoParameterConfig(multiple)
	return nil
}

func (value NoParameterConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string(value))
}

type FileConfig struct {
	SchemaVersion int    `json:"schemaVersion"`
	ProjectName   string `json:"projectName"`
	// DownloadDir is retained only for migration from schema <= 4. New files use Source.
	DownloadDir string            `json:"downloadDir,omitempty"`
	Source      *SourceConfig     `json:"source,omitempty"`
	ReleaseDir  string            `json:"releaseDir"`
	CurrentDir  string            `json:"currentDir"`
	NoParameter NoParameterConfig `json:"no parameter,omitempty"`
	Setup       *SetupConfig      `json:"setup,omitempty"`
	Backup      *BackupConfig     `json:"backup,omitempty"`
	Retention   *RetentionConfig  `json:"retention,omitempty"`
}

type Config struct {
	RootDir          string `json:"rootDir"`
	ConfigDir        string `json:"configDir"`
	ConfigFile       string `json:"configFile"`
	ProjectName      string `json:"projectName"`
	SourceType       string `json:"sourceType"`
	SourceFolder     string `json:"sourceFolder,omitempty"`
	SourceURL        string `json:"sourceUrl,omitempty"`
	SourceRepository string `json:"sourceRepository,omitempty"`
	// DownloadDir remains an alias for SourceFolder for compatibility with older callers.
	DownloadDir         string   `json:"downloadDir,omitempty"`
	ReleaseRoot         string   `json:"releaseDir"`
	CurrentDir          string   `json:"currentDir"`
	NoParameterActions  []string `json:"noParameterActions"`
	SetupCommands       []string `json:"setupCommands"`
	BackupRoot          string   `json:"backupDir"`
	KeepBackups         int      `json:"keepBackups"`
	KeepReleases        int      `json:"keepReleases"`
	HistoryFile         string   `json:"historyFile"`
	TemplatesFile       string   `json:"templatesFile"`
	GlobalConfigDir     string   `json:"globalConfigDir"`
	GlobalTemplatesFile string   `json:"globalTemplatesFile"`
}

type InitOptions struct {
	ProjectName string
	UseTemplate string
	SourceType  string
	Folder      string
	URL         string
	Repository  string
	Force       bool
}

type UpgradeResult struct {
	ConfigFile       string `json:"configFile"`
	BackupFile       string `json:"backupFile,omitempty"`
	PreviousSchema   int    `json:"previousSchemaVersion"`
	CurrentSchema    int    `json:"currentSchemaVersion"`
	Changed          bool   `json:"changed"`
	ProjectName      string `json:"projectName"`
	TemplatesFile    string `json:"templatesFile,omitempty"`
	TemplatesCreated bool   `json:"templatesCreated"`
	TemplatesUpdated bool   `json:"templatesUpdated"`
}

func ResolveRoot(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return absoluteDir(explicit)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Arbeitsverzeichnis kann nicht gelesen werden: %w", err)
	}
	if hasProjectConfiguration(cwd) {
		return filepath.Abs(cwd)
	}

	executable, err := os.Executable()
	if err == nil {
		executable, _ = filepath.EvalSymlinks(executable)
		candidate := filepath.Dir(executable)
		for range 2 {
			if hasProjectConfiguration(candidate) {
				return filepath.Abs(candidate)
			}
			candidate = filepath.Dir(candidate)
		}
	}

	return filepath.Abs(cwd)
}

func Init(rootDir string, options InitOptions) (Config, error) {
	rootDir, err := absoluteDir(rootDir)
	if err != nil {
		return Config{}, err
	}

	projectName := strings.TrimSpace(options.ProjectName)
	if projectName == "" {
		return Config{}, errors.New(
			"Projektname fehlt\n\nInitialisierung mit direktem Projektnamen:\n\n  update-cli --init mediastudio",
		)
	}
	if err := validateProjectName(projectName); err != nil {
		return Config{}, err
	}
	globalTemplatesFile, err := buildconfig.GlobalTemplatesFile()
	if err != nil {
		return Config{}, fmt.Errorf("globaler Template-Pfad ist ungültig: %w", err)
	}
	if strings.TrimSpace(options.UseTemplate) != "" {
		if _, err := templates.LookupCatalog(globalTemplatesFile, options.UseTemplate); err != nil {
			return Config{}, fmt.Errorf("Template für Initialisierung ist ungültig: %w", err)
		}
	}

	configDir := filepath.Join(rootDir, ConfigDirName)
	configFile := filepath.Join(configDir, ConfigFileName)
	if _, err := os.Stat(configFile); err == nil && !options.Force {
		return Config{}, fmt.Errorf(
			"Konfiguration existiert bereits: %s\nZum Überschreiben --force verwenden",
			configFile,
		)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("Konfiguration kann nicht geprüft werden: %w", err)
	}

	fileConfig := defaultFileConfig(projectName)
	if err := applySourceToFileConfig(&fileConfig, options.SourceType, options.Folder, options.URL, options.Repository); err != nil {
		return Config{}, err
	}
	if err := writeConfigFile(configFile, fileConfig); err != nil {
		return Config{}, err
	}

	templatesFile := filepath.Join(configDir, TemplatesFileName)
	if _, err := templates.WriteConfiguredDefaults(templatesFile, globalTemplatesFile, options.Force); err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(options.UseTemplate) != "" {
		if _, _, err := ApplyTemplate(rootDir, options.UseTemplate); err != nil {
			return Config{}, fmt.Errorf("Initialisierung wurde erstellt, aber Template konnte nicht angewendet werden: %w", err)
		}
	}

	return Load(rootDir, "")
}

func Load(rootDir, downloadOverride string) (Config, error) {
	rootDir, err := absoluteDir(rootDir)
	if err != nil {
		return Config{}, err
	}

	configDir := filepath.Join(rootDir, ConfigDirName)
	configFile := filepath.Join(configDir, ConfigFileName)
	fileConfig, err := readConfigFile(rootDir, configFile)
	if err != nil {
		return Config{}, err
	}
	fileConfig, _, err = migrateFileConfig(fileConfig)
	if err != nil {
		return Config{}, fmt.Errorf("Konfiguration %s kann nicht gelesen werden: %w", configFile, err)
	}
	if err := validateFileConfig(fileConfig); err != nil {
		return Config{}, fmt.Errorf("ungültige Konfiguration %s: %w", configFile, err)
	}

	sourceType := strings.ToLower(strings.TrimSpace(fileConfig.Source.Type))
	sourceFolder := strings.TrimSpace(fileConfig.Source.Folder)
	if strings.TrimSpace(downloadOverride) != "" {
		sourceType = "download"
		sourceFolder = strings.TrimSpace(downloadOverride)
	}
	if sourceFolder != "" {
		sourceFolder, err = expandAndAbs(sourceFolder)
		if err != nil {
			return Config{}, fmt.Errorf("Quellordner ist ungültig: %w", err)
		}
	}

	releaseRoot, err := resolveProjectDirectory(rootDir, fileConfig.ReleaseDir, "releaseDir")
	if err != nil {
		return Config{}, err
	}
	currentDir, err := resolveProjectDirectory(rootDir, fileConfig.CurrentDir, "currentDir")
	if err != nil {
		return Config{}, err
	}
	if samePath(releaseRoot, currentDir) {
		return Config{}, errors.New("releaseDir und currentDir dürfen nicht identisch sein")
	}

	backupDirectory := "backup"
	keepBackups := 3
	if fileConfig.Backup != nil {
		backupDirectory = fileConfig.Backup.Directory
		keepBackups = fileConfig.Backup.Keep
	}
	backupRoot, err := resolveProjectDirectory(rootDir, backupDirectory, "backup.directory")
	if err != nil {
		return Config{}, err
	}
	if samePath(backupRoot, currentDir) || samePath(backupRoot, releaseRoot) {
		return Config{}, errors.New("backup.directory darf nicht mit releaseDir oder currentDir identisch sein")
	}
	keepReleases := 5
	if fileConfig.Retention != nil {
		keepReleases = fileConfig.Retention.Releases
	}
	globalConfigDir, err := buildconfig.ExpandPath(buildconfig.Current().DefaultConfigPath)
	if err != nil {
		return Config{}, fmt.Errorf("globaler Konfigurationspfad ist ungültig: %w", err)
	}
	globalTemplatesFile := filepath.Join(globalConfigDir, TemplatesFileName)

	return Config{
		RootDir:             rootDir,
		ConfigDir:           configDir,
		ConfigFile:          configFile,
		ProjectName:         fileConfig.ProjectName,
		SourceType:          sourceType,
		SourceFolder:        sourceFolder,
		SourceURL:           strings.TrimSpace(fileConfig.Source.URL),
		SourceRepository:    strings.TrimSpace(fileConfig.Source.Repository),
		DownloadDir:         sourceFolder,
		ReleaseRoot:         releaseRoot,
		CurrentDir:          currentDir,
		NoParameterActions:  append([]string(nil), fileConfig.NoParameter...),
		SetupCommands:       normalizedSetupCommands(fileConfig.Setup),
		BackupRoot:          backupRoot,
		KeepBackups:         keepBackups,
		KeepReleases:        keepReleases,
		HistoryFile:         filepath.Join(configDir, "history.jsonl"),
		TemplatesFile:       filepath.Join(configDir, TemplatesFileName),
		GlobalConfigDir:     globalConfigDir,
		GlobalTemplatesFile: globalTemplatesFile,
	}, nil
}

func readConfigFile(rootDir, path string) (FileConfig, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		legacyPath := filepath.Join(rootDir, LegacyConfigDirName, ConfigFileName)
		if fileExists(legacyPath) {
			return FileConfig{}, fmt.Errorf(
				"veraltete Updater-Konfiguration gefunden: %s\n\nDie Konfiguration wird jetzt ausschließlich hier erwartet:\n\n  %s\n\nNeu initialisieren mit:\n\n  update-cli --init mediastudio",
				legacyPath,
				path,
			)
		}
		return FileConfig{}, fmt.Errorf(
			"Updater-Konfiguration fehlt: %s\n\nInitialisieren mit:\n\n  update-cli --init mediastudio",
			path,
		)
	}
	if err != nil {
		return FileConfig{}, fmt.Errorf("Konfiguration kann nicht gelesen werden: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var result FileConfig
	if err := decoder.Decode(&result); err != nil {
		return FileConfig{}, fmt.Errorf("config.json enthält ungültiges JSON: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return FileConfig{}, err
	}
	return result, nil
}

func writeConfigFile(path string, value FileConfig) error {
	data, err := marshalPretty(value)
	if err != nil {
		return fmt.Errorf("Konfiguration kann nicht serialisiert werden: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("Konfigurationsordner kann nicht erstellt werden: %w", err)
	}

	temporary, err := os.CreateTemp(filepath.Dir(path), ".config-*.json")
	if err != nil {
		return fmt.Errorf("temporäre Konfiguration kann nicht erstellt werden: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return fmt.Errorf("Dateirechte der Konfiguration können nicht gesetzt werden: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("Konfiguration kann nicht geschrieben werden: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("Konfiguration kann nicht synchronisiert werden: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("Konfiguration kann nicht geschlossen werden: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("Konfiguration kann nicht aktiviert werden: %w", err)
	}
	return nil
}

func validateFileConfig(value FileConfig) error {
	if value.SchemaVersion != SchemaVersion {
		return fmt.Errorf("nicht unterstützte schemaVersion %d; erwartet wird %d; aktualisieren mit update-cli --upgrade", value.SchemaVersion, SchemaVersion)
	}
	if err := validateProjectName(value.ProjectName); err != nil {
		return err
	}
	if value.Source == nil {
		return errors.New("source fehlt")
	}
	sourceType := strings.ToLower(strings.TrimSpace(value.Source.Type))
	switch sourceType {
	case "download":
		if strings.TrimSpace(value.Source.Folder) == "" {
			return errors.New("source.folder fehlt oder ist leer")
		}
	case "url":
		if strings.TrimSpace(value.Source.URL) == "" {
			return errors.New("source.url fehlt oder ist leer")
		}
	case "repository":
		if strings.TrimSpace(value.Source.Repository) == "" {
			return errors.New("source.repository fehlt oder ist leer")
		}
	default:
		return fmt.Errorf("source.type muss download, url oder repository sein; erhalten: %q", value.Source.Type)
	}
	if strings.TrimSpace(value.ReleaseDir) == "" {
		return errors.New("releaseDir fehlt oder ist leer")
	}
	if strings.TrimSpace(value.CurrentDir) == "" {
		return errors.New("currentDir fehlt oder ist leer")
	}
	actions, err := normalizedNoParameter(value.NoParameter)
	if err != nil {
		return err
	}
	value.NoParameter = NoParameterConfig(actions)
	if value.Backup != nil {
		if strings.TrimSpace(value.Backup.Directory) == "" {
			return errors.New("backup.directory fehlt oder ist leer")
		}
		if value.Backup.Keep < 0 {
			return errors.New("backup.keep darf nicht negativ sein")
		}
	}
	if value.Retention != nil && value.Retention.Releases < 0 {
		return errors.New("retention.releases darf nicht negativ sein")
	}
	if value.Setup != nil {
		for index, command := range value.Setup.Commands {
			command = strings.TrimSpace(command)
			if command == "" {
				return fmt.Errorf("setup.commands[%d] ist leer", index)
			}
			if strings.ContainsRune(command, '\x00') {
				return fmt.Errorf("setup.commands[%d] enthält ein ungültiges Nullzeichen", index)
			}
		}
	}
	return nil
}

func normalizedNoParameter(value NoParameterConfig) ([]string, error) {
	if len(value) == 0 {
		return []string{"help"}, nil
	}

	seen := make(map[string]bool, len(value))
	hasHelp := false
	hasUpdate := false
	hasSetup := false
	for index, raw := range value {
		action := strings.ToLower(strings.TrimSpace(raw))
		if action == "" {
			return nil, fmt.Errorf(`"no parameter"[%d] ist leer`, index)
		}
		if seen[action] {
			return nil, fmt.Errorf(`"no parameter" enthält den Befehl %q mehrfach`, action)
		}
		seen[action] = true
		switch action {
		case "help":
			hasHelp = true
		case "update":
			hasUpdate = true
		case "setup":
			hasSetup = true
		default:
			return nil, fmt.Errorf(`"no parameter" unterstützt nur "help", "update" und "setup"; erhalten: %q`, action)
		}
	}

	if hasHelp && len(seen) > 1 {
		return nil, errors.New(`"help" kann in "no parameter" nicht mit weiteren Befehlen kombiniert werden`)
	}
	if hasHelp {
		return []string{"help"}, nil
	}

	actions := make([]string, 0, 2)
	if hasUpdate {
		actions = append(actions, "update")
	}
	if hasSetup {
		actions = append(actions, "setup")
	}
	return actions, nil
}

func normalizedSetupCommands(setup *SetupConfig) []string {
	if setup == nil || len(setup.Commands) == 0 {
		return []string{}
	}
	commands := make([]string, 0, len(setup.Commands))
	for _, command := range setup.Commands {
		commands = append(commands, strings.TrimSpace(command))
	}
	return commands
}

// ApplyTemplate applies the populated sections of a named project template
// while preserving every unrelated configuration field.
func ApplyTemplate(rootDir, templateName string) (string, templates.Template, error) {
	rootDir, err := absoluteDir(rootDir)
	if err != nil {
		return "", templates.Template{}, err
	}

	templatesPath := filepath.Join(rootDir, ConfigDirName, TemplatesFileName)
	selected, err := templates.LookupFile(templatesPath, templateName)
	if err != nil {
		return "", templates.Template{}, err
	}

	path := filepath.Join(rootDir, ConfigDirName, ConfigFileName)
	value, err := readConfigFile(rootDir, path)
	if err != nil {
		return "", templates.Template{}, err
	}
	value, _, err = migrateFileConfig(value)
	if err != nil {
		return "", templates.Template{}, fmt.Errorf("Konfiguration %s kann nicht aktualisiert werden: %w", path, err)
	}
	if len(selected.NoParameter) > 0 {
		actions, err := normalizedNoParameter(NoParameterConfig(selected.NoParameter))
		if err != nil {
			return "", templates.Template{}, fmt.Errorf("Template %s enthält ungültige no-parameter-Aktionen: %w", selected.Name, err)
		}
		value.NoParameter = NoParameterConfig(actions)
	}
	if selected.Setup != nil {
		value.Setup = &SetupConfig{Commands: append([]string(nil), selected.Setup.Commands...)}
	}
	if err := validateFileConfig(value); err != nil {
		return "", templates.Template{}, fmt.Errorf("Template %s erzeugt eine ungültige Konfiguration: %w", selected.Name, err)
	}
	if err := writeConfigFile(path, value); err != nil {
		return "", templates.Template{}, err
	}
	if _, err := Load(rootDir, ""); err != nil {
		return "", templates.Template{}, fmt.Errorf("gespeicherte Konfiguration ist ungültig: %w", err)
	}

	return path, selected, nil
}

// ApplySetupTemplate remains as a compatibility wrapper for older callers.
func ApplySetupTemplate(rootDir, templateName string) (string, string, []string, error) {
	path, selected, err := ApplyTemplate(rootDir, templateName)
	if err != nil {
		return "", "", nil, err
	}
	commands := []string{}
	if selected.Setup != nil {
		commands = append(commands, selected.Setup.Commands...)
	}
	return path, selected.Name, commands, nil
}

// Format reads, validates, and returns config.json in normalized pretty-printed form.
func Format(rootDir string) (string, string, error) {
	rootDir, err := absoluteDir(rootDir)
	if err != nil {
		return "", "", err
	}
	path := filepath.Join(rootDir, ConfigDirName, ConfigFileName)
	value, err := readConfigFile(rootDir, path)
	if err != nil {
		return "", "", err
	}
	value, _, err = migrateFileConfig(value)
	if err != nil {
		return "", "", fmt.Errorf("Konfiguration %s kann nicht formatiert werden: %w", path, err)
	}
	if err := validateFileConfig(value); err != nil {
		return "", "", fmt.Errorf("ungültige Konfiguration %s: %w", path, err)
	}
	actions, err := normalizedNoParameter(value.NoParameter)
	if err != nil {
		return "", "", fmt.Errorf("ungültige no-parameter-Konfiguration: %w", err)
	}
	value.NoParameter = NoParameterConfig(actions)
	if _, err := Load(rootDir, ""); err != nil {
		return "", "", err
	}
	data, err := marshalPretty(value)
	if err != nil {
		return "", "", fmt.Errorf("Konfiguration kann nicht formatiert werden: %w", err)
	}
	return path, string(data), nil
}

func Upgrade(rootDir string) (UpgradeResult, error) {
	rootDir, err := absoluteDir(rootDir)
	if err != nil {
		return UpgradeResult{}, err
	}

	path := filepath.Join(rootDir, ConfigDirName, ConfigFileName)
	original, err := readConfigFile(rootDir, path)
	if err != nil {
		return UpgradeResult{}, err
	}
	upgraded, changed, err := migrateFileConfig(original)
	if err != nil {
		return UpgradeResult{}, fmt.Errorf("Konfiguration %s kann nicht aktualisiert werden: %w", path, err)
	}
	if err := validateFileConfig(upgraded); err != nil {
		return UpgradeResult{}, fmt.Errorf("aktualisierte Konfiguration ist ungültig: %w", err)
	}

	templatesPath := filepath.Join(rootDir, ConfigDirName, TemplatesFileName)
	templatesCreated, templatesUpdated, err := templates.EnsureCatalog(templatesPath, mustGlobalTemplatesFile())
	if err != nil {
		return UpgradeResult{}, fmt.Errorf("templates.json kann nicht aktualisiert werden: %w", err)
	}
	result := UpgradeResult{
		ConfigFile:       path,
		PreviousSchema:   original.SchemaVersion,
		CurrentSchema:    SchemaVersion,
		Changed:          changed || templatesCreated || templatesUpdated,
		ProjectName:      upgraded.ProjectName,
		TemplatesFile:    templatesPath,
		TemplatesCreated: templatesCreated,
		TemplatesUpdated: templatesUpdated,
	}
	if !changed {
		return result, nil
	}

	backupPath, err := backupConfigFile(path, original.SchemaVersion)
	if err != nil {
		return UpgradeResult{}, err
	}
	result.BackupFile = backupPath
	if err := writeConfigFile(path, upgraded); err != nil {
		return UpgradeResult{}, err
	}
	if _, err := Load(rootDir, ""); err != nil {
		if restoreErr := writeConfigFile(path, original); restoreErr != nil {
			return UpgradeResult{}, fmt.Errorf("aktualisierte Konfiguration ist ungültig (%v) und die vorherige Konfiguration konnte nicht wiederhergestellt werden: %w", err, restoreErr)
		}
		return UpgradeResult{}, fmt.Errorf("aktualisierte Konfiguration kann nicht geladen werden; vorherige Konfiguration wurde wiederhergestellt: %w", err)
	}
	return result, nil
}

func mustGlobalTemplatesFile() string {
	path, err := buildconfig.GlobalTemplatesFile()
	if err != nil {
		return ""
	}
	return path
}

func defaultFileConfig(projectName string) FileConfig {
	return FileConfig{
		SchemaVersion: SchemaVersion,
		ProjectName:   strings.TrimSpace(projectName),
		Source: &SourceConfig{
			Type:   "download",
			Folder: buildconfig.Current().DefaultDownloadFolder,
		},
		ReleaseDir:  "release",
		CurrentDir:  "current",
		NoParameter: NoParameterConfig{"help"},
		Setup:       &SetupConfig{Commands: []string{}},
		Backup:      &BackupConfig{Directory: "backup", Keep: 3},
		Retention:   &RetentionConfig{Releases: 5},
	}
}

func migrateFileConfig(value FileConfig) (FileConfig, bool, error) {
	original, err := marshalPretty(value)
	if err != nil {
		return FileConfig{}, false, err
	}
	if value.SchemaVersion > SchemaVersion {
		return FileConfig{}, false, fmt.Errorf(
			"schemaVersion %d ist neuer als die von diesem Programm unterstützte Version %d",
			value.SchemaVersion,
			SchemaVersion,
		)
	}
	if value.SchemaVersion < 1 {
		return FileConfig{}, false, fmt.Errorf("schemaVersion fehlt oder ist ungültig: %d", value.SchemaVersion)
	}

	value.SchemaVersion = SchemaVersion
	if value.Source == nil {
		folder := strings.TrimSpace(value.DownloadDir)
		if folder == "" {
			folder = buildconfig.Current().DefaultDownloadFolder
		}
		value.Source = &SourceConfig{Type: "download", Folder: folder}
	}
	value.Source.Type = strings.ToLower(strings.TrimSpace(value.Source.Type))
	if value.Source.Type == "" {
		value.Source.Type = "download"
	}
	if value.Source.Type == "download" && strings.TrimSpace(value.Source.Folder) == "" {
		value.Source.Folder = buildconfig.Current().DefaultDownloadFolder
	}
	value.DownloadDir = ""
	if strings.TrimSpace(value.ReleaseDir) == "" {
		value.ReleaseDir = "release"
	}
	if strings.TrimSpace(value.CurrentDir) == "" {
		value.CurrentDir = "current"
	}
	actions, err := normalizedNoParameter(value.NoParameter)
	if err != nil {
		return FileConfig{}, false, err
	}
	value.NoParameter = NoParameterConfig(actions)
	if value.Setup == nil {
		value.Setup = &SetupConfig{Commands: []string{}}
	}
	if value.Setup.Commands == nil {
		value.Setup.Commands = []string{}
	}
	if value.Backup == nil {
		value.Backup = &BackupConfig{Directory: "backup", Keep: 3}
	} else if strings.TrimSpace(value.Backup.Directory) == "" {
		value.Backup.Directory = "backup"
	}
	if value.Retention == nil {
		value.Retention = &RetentionConfig{Releases: 5}
	}

	migrated, err := marshalPretty(value)
	if err != nil {
		return FileConfig{}, false, err
	}
	return value, !bytes.Equal(original, migrated), nil
}

func backupConfigFile(path string, schemaVersion int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("Konfiguration kann vor dem Upgrade nicht gesichert werden: %w", err)
	}
	stamp := time.Now().Format("20060102-150405")
	base := fmt.Sprintf("%s.backup-v%d-%s", path, schemaVersion, stamp)
	candidate := base
	for index := 2; ; index++ {
		file, err := os.OpenFile(candidate, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			candidate = fmt.Sprintf("%s-%d", base, index)
			continue
		}
		if err != nil {
			return "", fmt.Errorf("Konfigurations-Backup kann nicht erstellt werden: %w", err)
		}
		if _, err := file.Write(data); err != nil {
			file.Close()
			os.Remove(candidate)
			return "", fmt.Errorf("Konfigurations-Backup kann nicht geschrieben werden: %w", err)
		}
		if err := file.Sync(); err != nil {
			file.Close()
			os.Remove(candidate)
			return "", fmt.Errorf("Konfigurations-Backup kann nicht synchronisiert werden: %w", err)
		}
		if err := file.Close(); err != nil {
			return "", fmt.Errorf("Konfigurations-Backup kann nicht geschlossen werden: %w", err)
		}
		return candidate, nil
	}
}

func marshalPretty(value FileConfig) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func validateProjectName(projectName string) error {
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		return errors.New("projectName fehlt oder ist leer")
	}
	if !projectNamePattern.MatchString(projectName) {
		return fmt.Errorf(
			"ungültiger projectName %q: erlaubt sind Buchstaben, Ziffern, Punkt, Unterstrich und Bindestrich",
			projectName,
		)
	}
	return nil
}

func resolveProjectDirectory(rootDir, configured, field string) (string, error) {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return "", fmt.Errorf("%s fehlt oder ist leer", field)
	}
	if filepath.IsAbs(configured) {
		return "", fmt.Errorf("%s muss relativ zum Projektordner sein: %s", field, configured)
	}

	cleaned := filepath.Clean(configured)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("%s verweist außerhalb des Projektordners: %s", field, configured)
	}
	if cleaned == ConfigDirName || strings.HasPrefix(cleaned, ConfigDirName+string(os.PathSeparator)) {
		return "", fmt.Errorf("%s darf nicht innerhalb von %s liegen", field, ConfigDirName)
	}

	resolved := filepath.Join(rootDir, cleaned)
	relative, err := filepath.Rel(rootDir, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("%s verweist außerhalb des Projektordners: %s", field, configured)
	}
	return resolved, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("config.json enthält zusätzliche ungültige Daten: %w", err)
	}
	return errors.New("config.json enthält mehrere JSON-Werte")
}

func absoluteDir(path string) (string, error) {
	path, err := expandAndAbs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("Projektordner ist nicht verfügbar: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("Projektpfad ist kein Ordner: %s", path)
	}
	return path, nil
}

func expandAndAbs(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("leerer Pfad")
	}
	path = os.ExpandEnv(path)
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return filepath.Abs(path)
}

func hasProjectConfiguration(path string) bool {
	return fileExists(filepath.Join(path, ConfigDirName, ConfigFileName))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func samePath(left, right string) bool {
	left, _ = filepath.Abs(left)
	right, _ = filepath.Abs(right)
	return filepath.Clean(left) == filepath.Clean(right)
}

func applySourceToFileConfig(value *FileConfig, sourceType, folder, sourceURL, repository string) error {
	kind := strings.ToLower(strings.TrimSpace(sourceType))
	provided := 0
	if strings.TrimSpace(folder) != "" {
		provided++
		if kind == "" {
			kind = "download"
		}
	}
	if strings.TrimSpace(sourceURL) != "" {
		provided++
		if kind == "" {
			kind = "url"
		}
	}
	if strings.TrimSpace(repository) != "" {
		provided++
		if kind == "" {
			kind = "repository"
		}
	}
	if provided > 1 {
		return errors.New("--folder, --url und --repository schließen sich gegenseitig aus")
	}
	if kind == "download" && (strings.TrimSpace(sourceURL) != "" || strings.TrimSpace(repository) != "") {
		return errors.New("--from download kann nur mit --folder kombiniert werden")
	}
	if kind == "url" && (strings.TrimSpace(folder) != "" || strings.TrimSpace(repository) != "") {
		return errors.New("--from url kann nur mit --url kombiniert werden")
	}
	if kind == "repository" && (strings.TrimSpace(folder) != "" || strings.TrimSpace(sourceURL) != "") {
		return errors.New("--from repository kann nur mit --repository kombiniert werden")
	}
	if kind == "" {
		return nil
	}
	switch kind {
	case "download":
		selected := strings.TrimSpace(folder)
		if selected == "" {
			selected = value.Source.Folder
		}
		value.Source.Type = "download"
		value.Source.Folder = selected
	case "url":
		selected := strings.TrimSpace(sourceURL)
		if selected == "" {
			selected = value.Source.URL
		}
		if selected == "" {
			return errors.New("URL-Quelle benötigt --url")
		}
		value.Source.Type = "url"
		value.Source.URL = selected
	case "repository":
		selected := strings.TrimSpace(repository)
		if selected == "" {
			selected = value.Source.Repository
		}
		if selected == "" {
			return errors.New("Repository-Quelle benötigt --repository")
		}
		value.Source.Type = "repository"
		value.Source.Repository = selected
	default:
		return fmt.Errorf("--from muss download, url oder repository sein; erhalten: %q", kind)
	}
	return nil
}

// WithSourceOverrides returns a copy of cfg with command-line source settings
// applied. Supplying folder, url, or repository also selects the matching type.
func WithSourceOverrides(cfg Config, sourceType, folder, sourceURL, repository string) (Config, error) {
	kind := strings.ToLower(strings.TrimSpace(sourceType))
	provided := 0
	if strings.TrimSpace(folder) != "" {
		provided++
		if kind == "" {
			kind = "download"
		}
	}
	if strings.TrimSpace(sourceURL) != "" {
		provided++
		if kind == "" {
			kind = "url"
		}
	}
	if strings.TrimSpace(repository) != "" {
		provided++
		if kind == "" {
			kind = "repository"
		}
	}
	if provided > 1 {
		return Config{}, errors.New("--folder, --url und --repository schließen sich gegenseitig aus")
	}
	if kind == "" {
		kind = cfg.SourceType
	}
	switch kind {
	case "download":
		value := strings.TrimSpace(folder)
		if value == "" {
			value = cfg.SourceFolder
		}
		if value == "" {
			value = buildconfig.Current().DefaultDownloadFolder
		}
		resolved, err := expandAndAbs(value)
		if err != nil {
			return Config{}, fmt.Errorf("Quellordner ist ungültig: %w", err)
		}
		cfg.SourceFolder = resolved
		cfg.DownloadDir = resolved
	case "url":
		if strings.TrimSpace(sourceURL) != "" {
			cfg.SourceURL = strings.TrimSpace(sourceURL)
		}
		if cfg.SourceURL == "" {
			return Config{}, errors.New("URL-Quelle benötigt --url oder source.url")
		}
	case "repository":
		if strings.TrimSpace(repository) != "" {
			cfg.SourceRepository = strings.TrimSpace(repository)
		}
		if cfg.SourceRepository == "" {
			return Config{}, errors.New("Repository-Quelle benötigt --repository oder source.repository")
		}
	default:
		return Config{}, fmt.Errorf("--from muss download, url oder repository sein; erhalten: %q", kind)
	}
	cfg.SourceType = kind
	return cfg, nil
}
