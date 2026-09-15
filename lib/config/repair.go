package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/r14r/update-cli/lib/buildconfig"
)

// RepairFileResult describes one config.json repair performed by `update-cli fix`.
type RepairFileResult struct {
	Scope          string   `json:"scope"`
	Path           string   `json:"path"`
	BackupPath     string   `json:"backupPath,omitempty"`
	Changed        bool     `json:"changed"`
	Created        bool     `json:"created,omitempty"`
	RemovedFields  []string `json:"removedFields,omitempty"`
	MigratedFields []string `json:"migratedFields,omitempty"`
	Normalized     []string `json:"normalizedValues,omitempty"`
}

// RepairResult contains local and optional global runtime-config repair results.
type RepairResult struct {
	Local  RepairFileResult  `json:"local"`
	Global *RepairFileResult `json:"global,omitempty"`
}

// RepairProjectConfig repairs the project-local runtime config and, when it
// exists, the installation-wide partial runtime config. The repair parser is
// deliberately lenient so malformed/legacy field structure can be normalized
// before the normal strict parser is invoked.
func RepairProjectConfig(root string) (RepairResult, error) {
	abs, err := absoluteDir(root)
	if err != nil {
		return RepairResult{}, err
	}
	localPath, _ := configFilePath(abs)
	local, err := repairLocalConfig(abs, localPath)
	if err != nil {
		return RepairResult{Local: local}, err
	}
	result := RepairResult{Local: local}

	globalDir, err := GlobalConfigDir()
	if err != nil {
		return result, err
	}
	globalPath := filepath.Join(globalDir, ConfigFileName)
	if _, statErr := os.Stat(globalPath); statErr == nil {
		global, repairErr := repairGlobalConfig(globalPath)
		result.Global = &global
		if repairErr != nil {
			return result, repairErr
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return result, statErr
	}

	// Final validation uses the normal strict merged loader. This guarantees
	// fix never reports success for a config that the rest of update-cli rejects.
	if _, err := Load(abs, ""); err != nil {
		return result, fmt.Errorf("reparierte Konfiguration ist weiterhin ungültig: %w", err)
	}
	return result, nil
}

// PreviewRepairProjectConfig inspects the same deterministic repairs as
// RepairProjectConfig without modifying files. It is used by `doctor --fix`
// to show the exact planned config changes before asking for confirmation.
func PreviewRepairProjectConfig(root string) (RepairResult, error) {
	abs, err := absoluteDir(root)
	if err != nil {
		return RepairResult{}, err
	}
	localPath, _ := configFilePath(abs)
	local, err := previewLocalConfig(abs, localPath)
	if err != nil {
		return RepairResult{Local: local}, err
	}
	result := RepairResult{Local: local}

	globalDir, err := GlobalConfigDir()
	if err != nil {
		return result, err
	}
	globalPath := filepath.Join(globalDir, ConfigFileName)
	if _, statErr := os.Stat(globalPath); statErr == nil {
		global, previewErr := previewGlobalConfig(globalPath)
		result.Global = &global
		if previewErr != nil {
			return result, previewErr
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return result, statErr
	}
	return result, nil
}

func previewLocalConfig(root, path string) (RepairFileResult, error) {
	res := RepairFileResult{Scope: "local", Path: path}
	raw, err := readLooseJSONObject(path)
	if errors.Is(err, os.ErrNotExist) {
		res.Created = true
		res.Changed = true
		res.Normalized = append(res.Normalized, "fehlende lokale config.json mit aktuellen Defaults erstellen")
		return res, nil
	}
	if err != nil {
		return res, err
	}
	fc, changes := sanitizeLocalConfig(root, raw)
	res.RemovedFields = changes.removed
	res.MigratedFields = changes.migrated
	res.Normalized = changes.normalized
	if err := validateFile(fc); err != nil {
		return res, fmt.Errorf("config.json kann nicht sicher repariert werden: %w", err)
	}
	canonical, err := marshalPretty(fc)
	if err != nil {
		return res, err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return res, err
	}
	res.Changed = !jsonEquivalent(original, canonical)
	return res, nil
}

func previewGlobalConfig(path string) (RepairFileResult, error) {
	res := RepairFileResult{Scope: "global", Path: path}
	raw, err := readLooseJSONObject(path)
	if err != nil {
		return res, err
	}
	clean, changes := sanitizeGlobalConfig(raw)
	res.RemovedFields = changes.removed
	res.MigratedFields = changes.migrated
	res.Normalized = changes.normalized
	canonical, err := marshalLooseJSON(clean)
	if err != nil {
		return res, err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return res, err
	}
	res.Changed = !jsonEquivalent(original, canonical)
	return res, nil
}

func repairLocalConfig(root, path string) (RepairFileResult, error) {
	res := RepairFileResult{Scope: "local", Path: path}
	raw, err := readLooseJSONObject(path)
	if errors.Is(err, os.ErrNotExist) {
		fc := defaultFile(filepath.Base(root))
		if err := writeConfigFile(filepath.Join(root, ConfigDirName, ConfigFileName), fc); err != nil {
			return res, err
		}
		res.Path = filepath.Join(root, ConfigDirName, ConfigFileName)
		res.Created = true
		res.Changed = true
		res.Normalized = append(res.Normalized, "fehlende lokale config.json mit aktuellen Defaults erstellt")
		return res, nil
	}
	if err != nil {
		return res, err
	}

	fc, changes := sanitizeLocalConfig(root, raw)
	res.RemovedFields = changes.removed
	res.MigratedFields = changes.migrated
	res.Normalized = changes.normalized
	if err := validateFile(fc); err != nil {
		return res, fmt.Errorf("config.json kann nicht sicher repariert werden: %w", err)
	}
	canonical, err := marshalPretty(fc)
	if err != nil {
		return res, err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return res, err
	}
	res.Changed = !jsonEquivalent(original, canonical)
	if !res.Changed {
		return res, nil
	}
	backup, err := backupRawFile(path, "fix")
	if err != nil {
		return res, err
	}
	res.BackupPath = backup
	if err := writeConfigFile(path, fc); err != nil {
		return res, err
	}
	return res, nil
}

func repairGlobalConfig(path string) (RepairFileResult, error) {
	res := RepairFileResult{Scope: "global", Path: path}
	raw, err := readLooseJSONObject(path)
	if err != nil {
		return res, err
	}
	clean, changes := sanitizeGlobalConfig(raw)
	res.RemovedFields = changes.removed
	res.MigratedFields = changes.migrated
	res.Normalized = changes.normalized
	canonical, err := marshalLooseJSON(clean)
	if err != nil {
		return res, err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return res, err
	}
	res.Changed = !jsonEquivalent(original, canonical)
	if !res.Changed {
		return res, nil
	}
	backup, err := backupRawFile(path, "fix")
	if err != nil {
		return res, fmt.Errorf("globale config.json benötigt Reparatur, Backup/Schreibzugriff fehlgeschlagen: %w", err)
	}
	res.BackupPath = backup
	if err := atomicWriteBytes(path, canonical, 0o644); err != nil {
		return res, fmt.Errorf("globale config.json benötigt Reparatur, Schreiben fehlgeschlagen: %w", err)
	}
	return res, nil
}

type repairChanges struct {
	removed    []string
	migrated   []string
	normalized []string
}

func sanitizeLocalConfig(root string, raw map[string]any) (FileConfig, repairChanges) {
	changes := repairChanges{}
	knownTop := map[string]bool{
		"schemaVersion": true, "projectName": true, "mode": true, "downloadDir": true,
		"defaultUser": true, "source": true, "releaseDir": true, "currentDir": true,
		"keepRsyncOnError": true, "keepOnSetupError": true,
		"no parameter": true, "no-param": true, "no-parameter": true, "noParameter": true,
		"setup": true, "backup": true, "retention": true,
		"sync": true, "security": true, "docker": true, "healthcheck": true,
	}
	for key := range raw {
		if !knownTop[key] {
			changes.removed = append(changes.removed, key)
		}
	}

	project := filepath.Base(root)
	if s, ok := looseString(raw["projectName"]); ok && projectNamePattern.MatchString(strings.TrimSpace(s)) {
		project = strings.TrimSpace(s)
	} else if _, exists := raw["projectName"]; exists {
		changes.normalized = append(changes.normalized, "projectName auf Projektordner zurückgesetzt")
	}
	if !projectNamePattern.MatchString(project) {
		project = sanitizeProjectName(project)
	}
	fc := defaultFile(project)
	fc.SchemaVersion = SchemaVersion
	if old, ok := looseInt(raw["schemaVersion"]); !ok || old != SchemaVersion {
		changes.normalized = append(changes.normalized, fmt.Sprintf("schemaVersion → %d", SchemaVersion))
	}

	legacyDefaultUser, legacyPresent := looseString(raw["defaultUser"])
	legacyDownload, legacyDownloadPresent := looseString(raw["downloadDir"])
	if legacyPresent {
		changes.migrated = append(changes.migrated, "defaultUser → source.defaultUser")
	}
	if legacyDownloadPresent {
		changes.migrated = append(changes.migrated, "downloadDir → source.folder")
	}

	sourceRaw, _ := raw["source"].(map[string]any)
	source := SourceConfig{Type: "download", Folder: buildconfig.Current().DefaultDownloadFolder, DefaultUser: DefaultRepositoryUser}
	if sourceRaw != nil {
		known := map[string]bool{"type": true, "defaultUser": true, "folder": true, "url": true, "repository": true, "ref": true, "commit": true, "version": true, "sha256": true}
		for key := range sourceRaw {
			if !known[key] {
				changes.removed = append(changes.removed, "source."+key)
			}
		}
		for key, target := range map[string]*string{
			"folder": &source.Folder, "url": &source.URL, "repository": &source.Repository,
			"ref": &source.Ref, "commit": &source.Commit, "version": &source.Version, "sha256": &source.SHA256,
		} {
			if s, ok := looseString(sourceRaw[key]); ok {
				*target = strings.TrimSpace(s)
			} else if _, exists := sourceRaw[key]; exists {
				changes.normalized = append(changes.normalized, "source."+key+" entfernt (falscher Werttyp)")
			}
		}
		if user, ok := looseString(sourceRaw["defaultUser"]); ok && validateRepositoryUser(strings.TrimSpace(user)) == nil {
			source.DefaultUser = strings.TrimSpace(user)
		} else if _, exists := sourceRaw["defaultUser"]; exists {
			changes.normalized = append(changes.normalized, "source.defaultUser → "+DefaultRepositoryUser)
		}
		if typ, ok := looseString(sourceRaw["type"]); ok {
			source.Type = strings.ToLower(strings.TrimSpace(typ))
		}
	}
	if legacyPresent && strings.TrimSpace(legacyDefaultUser) != "" && validateRepositoryUser(strings.TrimSpace(legacyDefaultUser)) == nil {
		source.DefaultUser = strings.TrimSpace(legacyDefaultUser)
	}
	if legacyDownloadPresent && strings.TrimSpace(legacyDownload) != "" && source.Folder == buildconfig.Current().DefaultDownloadFolder {
		source.Folder = strings.TrimSpace(legacyDownload)
	}
	if source.Type != "download" && source.Type != "url" && source.Type != "repository" {
		switch {
		case source.Repository != "":
			source.Type = "repository"
		case source.URL != "":
			source.Type = "url"
		default:
			source.Type = "download"
		}
		changes.normalized = append(changes.normalized, "source.type auf gültige Quelle normalisiert")
	}
	if source.Type == "download" && strings.TrimSpace(source.Folder) == "" {
		source.Folder = buildconfig.Current().DefaultDownloadFolder
		changes.normalized = append(changes.normalized, "source.folder auf Default gesetzt")
	}
	if source.Type == "url" && strings.TrimSpace(source.URL) == "" {
		source.Type = "download"
		source.Folder = buildconfig.Current().DefaultDownloadFolder
		changes.normalized = append(changes.normalized, "unvollständige URL-Quelle → download")
	}
	if source.Type == "repository" && strings.TrimSpace(source.Repository) == "" {
		source.Type = "download"
		source.Folder = buildconfig.Current().DefaultDownloadFolder
		changes.normalized = append(changes.normalized, "unvollständige Repository-Quelle → download")
	}
	fc.Source = &source

	mode := ""
	if s, ok := looseString(raw["mode"]); ok {
		mode = strings.ToLower(strings.TrimSpace(s))
	}
	if mode != ModeUpdate && mode != ModePull {
		if source.Type == "repository" {
			mode = ModePull
		} else {
			mode = ModeUpdate
		}
		if _, exists := raw["mode"]; exists {
			changes.normalized = append(changes.normalized, "mode passend zur Quelle normalisiert")
		}
	}
	if source.Type == "repository" && mode != ModePull {
		mode = ModePull
		changes.normalized = append(changes.normalized, "mode → pull für Repository-Quelle")
	} else if source.Type != "repository" && mode != ModeUpdate {
		mode = ModeUpdate
		changes.normalized = append(changes.normalized, "mode → update für ZIP/URL-Quelle")
	}
	fc.Mode = mode

	if s, ok := looseString(raw["releaseDir"]); ok && safeRelativeProjectPath(s) {
		fc.ReleaseDir = strings.TrimSpace(s)
	} else if _, exists := raw["releaseDir"]; exists {
		changes.normalized = append(changes.normalized, "releaseDir → release")
	}
	if s, ok := looseString(raw["currentDir"]); ok && safeRelativeProjectPath(s) {
		fc.CurrentDir = strings.TrimSpace(s)
	} else if _, exists := raw["currentDir"]; exists {
		changes.normalized = append(changes.normalized, "currentDir → current")
	}

	noParameterKey := ""
	for _, key := range []string{"no parameter", "no-param", "no-parameter", "noParameter"} {
		if _, exists := raw[key]; exists {
			noParameterKey = key
			break
		}
	}
	if noParameterKey != "" {
		if values, ok := looseStringList(raw[noParameterKey]); ok {
			clean := normalizeNoParameterRepair(values)
			fc.NoParameter = NoParameterConfig(clean)
			if noParameterKey != "no parameter" {
				changes.migrated = append(changes.migrated, noParameterKey+" → no parameter")
			}
			if strings.Join(values, "\x00") != strings.Join(clean, "\x00") {
				changes.normalized = append(changes.normalized, "no parameter bereinigt")
			}
		} else {
			changes.normalized = append(changes.normalized, "no parameter → [\"check\"]")
		}
	}

	policySet := false
	if setup, ok := raw["setup"].(map[string]any); ok {
		for key := range setup {
			if key != "commands" && key != "keepRsyncOnError" && key != "keepOnSetupError" {
				changes.removed = append(changes.removed, "setup."+key)
			}
		}
		if commands, ok := looseStringList(setup["commands"]); ok {
			fc.Setup.Commands = commands
		}
		if value, ok := looseBool(setup["keepRsyncOnError"]); ok {
			fc.Setup.KeepRsyncOnError = &value
			policySet = true
		} else if _, exists := setup["keepRsyncOnError"]; exists {
			changes.normalized = append(changes.normalized, "setup.keepRsyncOnError entfernt (ungültiger Boolean)")
		}
		if _, exists := setup["keepOnSetupError"]; exists {
			if !policySet {
				if value, ok := looseBool(setup["keepOnSetupError"]); ok {
					fc.Setup.KeepRsyncOnError = &value
					policySet = true
				}
			}
			changes.migrated = append(changes.migrated, "setup.keepOnSetupError → setup.keepRsyncOnError")
		}
	} else if _, exists := raw["setup"]; exists {
		changes.normalized = append(changes.normalized, "setup auf aktuelle Struktur zurückgesetzt")
	}

	if backup, ok := raw["backup"].(map[string]any); ok {
		for key := range backup {
			if key != "directory" && key != "keep" {
				changes.removed = append(changes.removed, "backup."+key)
			}
		}
		if s, ok := looseString(backup["directory"]); ok && safeRelativeProjectPath(s) {
			fc.Backup.Directory = strings.TrimSpace(s)
		}
		if n, ok := looseInt(backup["keep"]); ok && n >= 0 {
			fc.Backup.Keep = n
		}
	}
	if retention, ok := raw["retention"].(map[string]any); ok {
		for key := range retention {
			if key != "releases" {
				changes.removed = append(changes.removed, "retention."+key)
			}
		}
		if n, ok := looseInt(retention["releases"]); ok && n >= 0 {
			fc.Retention.Releases = n
		}
	}
	if syncMap, ok := raw["sync"].(map[string]any); ok {
		for key := range syncMap {
			if key != "preserve" && key != "keepRsyncOnError" && key != "keepOnSetupError" {
				changes.removed = append(changes.removed, "sync."+key)
			}
		}
		for _, alias := range []string{"keepRsyncOnError", "keepOnSetupError"} {
			if _, exists := syncMap[alias]; !exists {
				continue
			}
			if !policySet {
				if value, ok := looseBool(syncMap[alias]); ok {
					fc.Setup.KeepRsyncOnError = &value
					policySet = true
				}
			}
			changes.migrated = append(changes.migrated, "sync."+alias+" → setup.keepRsyncOnError")
		}
		if values, ok := looseStringList(syncMap["preserve"]); ok {
			fc.Sync = &SyncConfig{Preserve: ensurePreserve(safePreserveList(values), ".gitignore")}
		}
	}

	if fc.Sync == nil {
		fc.Sync = &SyncConfig{Preserve: defaultPreserve()}
	}

	for _, alias := range []string{"keepRsyncOnError", "keepOnSetupError"} {
		if _, exists := raw[alias]; !exists {
			continue
		}
		if !policySet {
			if value, ok := looseBool(raw[alias]); ok {
				fc.Setup.KeepRsyncOnError = &value
				policySet = true
			}
		}
		changes.migrated = append(changes.migrated, alias+" → setup.keepRsyncOnError")
	}

	if security, ok := raw["security"].(map[string]any); ok {
		known := map[string]bool{"allowHttp": true, "maxArchiveBytes": true, "maxUncompressedBytes": true, "maxFileBytes": true, "maxEntries": true, "maxCompressionRatio": true}
		for key := range security {
			if !known[key] {
				changes.removed = append(changes.removed, "security."+key)
			}
		}
		if b, ok := looseBool(security["allowHttp"]); ok {
			fc.Security.AllowHTTP = b
		}
		if n, ok := looseInt64(security["maxArchiveBytes"]); ok && n > 0 {
			fc.Security.MaxArchiveBytes = n
		}
		if n, ok := looseInt64(security["maxUncompressedBytes"]); ok && n > 0 {
			fc.Security.MaxUncompressedBytes = n
		}
		if n, ok := looseInt64(security["maxFileBytes"]); ok && n > 0 {
			fc.Security.MaxFileBytes = n
		}
		if n, ok := looseInt(security["maxEntries"]); ok && n > 0 {
			fc.Security.MaxEntries = n
		}
		if n, ok := looseFloat(security["maxCompressionRatio"]); ok && n > 0 {
			fc.Security.MaxCompressionRatio = n
		}
	}
	if docker, ok := raw["docker"].(map[string]any); ok {
		for key := range docker {
			if key != "lifecycle" {
				changes.removed = append(changes.removed, "docker."+key)
			}
		}
		if s, ok := looseString(docker["lifecycle"]); ok {
			s = strings.ToLower(strings.TrimSpace(s))
			if s == "auto" || s == "disabled" || s == "required" {
				fc.Docker.Lifecycle = s
			} else {
				changes.normalized = append(changes.normalized, "docker.lifecycle → auto")
			}
		}
	}
	if fc.Source.Type == "url" && strings.HasPrefix(strings.ToLower(strings.TrimSpace(fc.Source.URL)), "http://") && !fc.Security.AllowHTTP {
		fc.Source.Type = "download"
		fc.Source.URL = ""
		fc.Source.Folder = buildconfig.Current().DefaultDownloadFolder
		fc.Mode = ModeUpdate
		changes.normalized = append(changes.normalized, "unsichere HTTP-Quelle deaktiviert → download")
	}

	if health, ok := raw["healthcheck"].(map[string]any); ok {
		known := map[string]bool{"type": true, "url": true, "command": true, "timeoutSeconds": true}
		for key := range health {
			if !known[key] {
				changes.removed = append(changes.removed, "healthcheck."+key)
			}
		}
		h := &HealthcheckConfig{}
		if s, ok := looseString(health["type"]); ok {
			h.Type = strings.ToLower(strings.TrimSpace(s))
		}
		if s, ok := looseString(health["url"]); ok {
			h.URL = strings.TrimSpace(s)
		}
		if s, ok := looseString(health["command"]); ok {
			h.Command = s
		}
		if n, ok := looseInt(health["timeoutSeconds"]); ok && n >= 0 {
			h.TimeoutSeconds = n
		}
		if h.Type != "" && h.Type != "none" && h.Type != "http" && h.Type != "command" {
			h.Type = ""
			changes.normalized = append(changes.normalized, "healthcheck.type entfernt")
		}
		if h.Type == "http" && h.URL == "" {
			h.Type = ""
			changes.normalized = append(changes.normalized, "unvollständiger HTTP-Healthcheck deaktiviert")
		}
		if h.Type == "command" && strings.TrimSpace(h.Command) == "" {
			h.Type = ""
			changes.normalized = append(changes.normalized, "unvollständiger Command-Healthcheck deaktiviert")
		}
		fc.Healthcheck = h
	}
	return fc, changes
}

func sanitizeGlobalConfig(raw map[string]any) (map[string]any, repairChanges) {
	changes := repairChanges{}
	allowedTop := map[string]bool{"schemaVersion": true, "projectName": true, "mode": true, "downloadDir": true, "defaultUser": true, "source": true, "releaseDir": true, "currentDir": true, "no parameter": true, "no-param": true, "no-parameter": true, "noParameter": true, "setup": true, "backup": true, "retention": true, "sync": true, "security": true, "docker": true, "healthcheck": true, "keepRsyncOnError": true, "keepOnSetupError": true}
	out := map[string]any{}
	for key, value := range raw {
		if !allowedTop[key] {
			changes.removed = append(changes.removed, key)
			continue
		}
		out[key] = value
	}
	if legacy, ok := looseString(out["defaultUser"]); ok {
		source, _ := out["source"].(map[string]any)
		if source == nil {
			source = map[string]any{}
		}
		if _, exists := source["defaultUser"]; !exists && validateRepositoryUser(strings.TrimSpace(legacy)) == nil {
			source["defaultUser"] = strings.TrimSpace(legacy)
		}
		out["source"] = source
		delete(out, "defaultUser")
		changes.migrated = append(changes.migrated, "defaultUser → source.defaultUser")
	}
	// Sanitize nested fields and primitive types. Invalid global values are
	// removed so project-local or built-in defaults can take over.
	if source, ok := out["source"].(map[string]any); ok {
		clean := map[string]any{}
		for _, key := range []string{"type", "defaultUser", "folder", "url", "repository", "ref", "commit", "version", "sha256"} {
			if s, valid := looseString(source[key]); valid {
				clean[key] = strings.TrimSpace(s)
			}
		}
		if user, exists := clean["defaultUser"].(string); exists && validateRepositoryUser(user) != nil {
			delete(clean, "defaultUser")
			changes.normalized = append(changes.normalized, "global source.defaultUser entfernt (ungültig)")
		}
		for key := range source {
			if _, exists := clean[key]; !exists {
				changes.removed = append(changes.removed, "source."+key)
			}
		}
		if len(clean) == 0 {
			delete(out, "source")
		} else {
			out["source"] = clean
		}
	} else if _, exists := out["source"]; exists {
		delete(out, "source")
		changes.normalized = append(changes.normalized, "global source entfernt (keine Map)")
	}

	sanitizeGlobalObject := func(name string, allowed map[string]string) {
		obj, ok := out[name].(map[string]any)
		if !ok {
			if _, exists := out[name]; exists {
				delete(out, name)
				changes.normalized = append(changes.normalized, "global "+name+" entfernt (keine Map)")
			}
			return
		}
		clean := map[string]any{}
		for key, kind := range allowed {
			value, exists := obj[key]
			if !exists {
				continue
			}
			switch kind {
			case "string":
				if s, ok := looseString(value); ok {
					clean[key] = strings.TrimSpace(s)
				}
			case "bool":
				if b, ok := looseBool(value); ok {
					clean[key] = b
				}
			case "int":
				if n, ok := looseInt(value); ok {
					clean[key] = n
				}
			case "int64":
				if n, ok := looseInt64(value); ok {
					clean[key] = n
				}
			case "float":
				if n, ok := looseFloat(value); ok {
					clean[key] = n
				}
			case "strings":
				if a, ok := looseStringList(value); ok {
					clean[key] = uniqueNonEmpty(a)
				}
			}
		}
		for key := range obj {
			if _, exists := allowed[key]; !exists {
				changes.removed = append(changes.removed, name+"."+key)
			}
		}
		if len(clean) == 0 {
			delete(out, name)
		} else {
			out[name] = clean
		}
	}
	// Runtime policy aliases emitted by transitional releases are normalized
	// before nested object sanitizing. Canonical config.json uses
	// setup.keepRsyncOnError.
	setupObj, _ := out["setup"].(map[string]any)
	if setupObj == nil {
		setupObj = map[string]any{}
	}
	_, canonicalPolicy := setupObj["keepRsyncOnError"]
	if value, exists := setupObj["keepOnSetupError"]; exists {
		if !canonicalPolicy {
			if b, ok := looseBool(value); ok {
				setupObj["keepRsyncOnError"] = b
				canonicalPolicy = true
			}
		}
		delete(setupObj, "keepOnSetupError")
		changes.migrated = append(changes.migrated, "setup.keepOnSetupError → setup.keepRsyncOnError")
	}
	if syncObj, ok := out["sync"].(map[string]any); ok {
		for _, alias := range []string{"keepRsyncOnError", "keepOnSetupError"} {
			if value, exists := syncObj[alias]; exists {
				if !canonicalPolicy {
					if b, ok := looseBool(value); ok {
						setupObj["keepRsyncOnError"] = b
						canonicalPolicy = true
					}
				}
				delete(syncObj, alias)
				changes.migrated = append(changes.migrated, "sync."+alias+" → setup.keepRsyncOnError")
			}
		}
		out["sync"] = syncObj
	}
	for _, alias := range []string{"keepRsyncOnError", "keepOnSetupError"} {
		if value, exists := out[alias]; exists {
			if !canonicalPolicy {
				if b, ok := looseBool(value); ok {
					setupObj["keepRsyncOnError"] = b
					canonicalPolicy = true
				}
			}
			delete(out, alias)
			changes.migrated = append(changes.migrated, alias+" → setup.keepRsyncOnError")
		}
	}
	if len(setupObj) > 0 {
		out["setup"] = setupObj
	}

	sanitizeGlobalObject("setup", map[string]string{"commands": "strings", "keepRsyncOnError": "bool"})
	sanitizeGlobalObject("backup", map[string]string{"directory": "string", "keep": "int"})
	sanitizeGlobalObject("retention", map[string]string{"releases": "int"})
	sanitizeGlobalObject("sync", map[string]string{"preserve": "strings"})
	if syncObj, ok := out["sync"].(map[string]any); ok {
		if values, ok := looseStringList(syncObj["preserve"]); ok {
			syncObj["preserve"] = safePreserveList(values)
		}
	}
	sanitizeGlobalObject("security", map[string]string{"allowHttp": "bool", "maxArchiveBytes": "int64", "maxUncompressedBytes": "int64", "maxFileBytes": "int64", "maxEntries": "int", "maxCompressionRatio": "float"})
	sanitizeGlobalObject("docker", map[string]string{"lifecycle": "string"})
	sanitizeGlobalObject("healthcheck", map[string]string{"type": "string", "url": "string", "command": "string", "timeoutSeconds": "int"})

	for _, key := range []string{"projectName", "mode", "downloadDir", "releaseDir", "currentDir"} {
		if value, exists := out[key]; exists {
			if s, ok := looseString(value); ok {
				out[key] = strings.TrimSpace(s)
			} else {
				delete(out, key)
				changes.normalized = append(changes.normalized, "global "+key+" entfernt (falscher Werttyp)")
			}
		}
	}
	if value, exists := out["schemaVersion"]; exists {
		if n, ok := looseInt(value); ok {
			out["schemaVersion"] = n
		} else {
			delete(out, "schemaVersion")
		}
	}
	for _, alias := range []string{"no-param", "no-parameter", "noParameter"} {
		if value, exists := out[alias]; exists {
			if _, canonicalExists := out["no parameter"]; !canonicalExists {
				out["no parameter"] = value
			}
			delete(out, alias)
			changes.migrated = append(changes.migrated, alias+" → no parameter")
		}
	}
	if value, exists := out["no parameter"]; exists {
		if a, ok := looseStringList(value); ok {
			out["no parameter"] = normalizeNoParameterRepair(a)
		} else {
			delete(out, "no parameter")
		}
	}

	// Semantic cleanup: global config is a partial layer, so invalid values are
	// removed rather than guessed. The local layer or built-in defaults then
	// provide a valid value during the normal merge/load path.
	out["schemaVersion"] = SchemaVersion
	if value, ok := out["projectName"].(string); ok && validateProjectName(value) != nil {
		delete(out, "projectName")
		changes.normalized = append(changes.normalized, "global projectName entfernt (ungültig)")
	}
	if value, ok := out["mode"].(string); ok {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != ModeUpdate && value != ModePull {
			delete(out, "mode")
			changes.normalized = append(changes.normalized, "global mode entfernt (ungültig)")
		} else {
			out["mode"] = value
		}
	}
	for _, key := range []string{"releaseDir", "currentDir"} {
		if value, ok := out[key].(string); ok && !safeRelativeProjectPath(value) {
			delete(out, key)
			changes.normalized = append(changes.normalized, "global "+key+" entfernt (unsicherer Pfad)")
		}
	}
	if source, ok := out["source"].(map[string]any); ok {
		if typ, exists := source["type"].(string); exists {
			typ = strings.ToLower(strings.TrimSpace(typ))
			if typ != "download" && typ != "url" && typ != "repository" {
				delete(source, "type")
				changes.normalized = append(changes.normalized, "global source.type entfernt (ungültig)")
			} else {
				source["type"] = typ
			}
		}
	}
	if backup, ok := out["backup"].(map[string]any); ok {
		if directory, exists := backup["directory"].(string); exists && !safeRelativeProjectPath(directory) {
			delete(backup, "directory")
			changes.normalized = append(changes.normalized, "global backup.directory entfernt (unsicherer Pfad)")
		}
		if keep, exists := backup["keep"].(int); exists && keep < 0 {
			delete(backup, "keep")
			changes.normalized = append(changes.normalized, "global backup.keep entfernt (negativ)")
		}
	}
	if retention, ok := out["retention"].(map[string]any); ok {
		if releases, exists := retention["releases"].(int); exists && releases < 0 {
			delete(retention, "releases")
			changes.normalized = append(changes.normalized, "global retention.releases entfernt (negativ)")
		}
	}
	if syncObj, ok := out["sync"].(map[string]any); ok {
		if values, exists := syncObj["preserve"].([]string); exists {
			syncObj["preserve"] = safePreserveList(values)
		}
	}
	if security, ok := out["security"].(map[string]any); ok {
		for _, key := range []string{"maxArchiveBytes", "maxUncompressedBytes", "maxFileBytes", "maxEntries", "maxCompressionRatio"} {
			if value, exists := security[key]; exists && !positiveNumber(value) {
				delete(security, key)
				changes.normalized = append(changes.normalized, "global security."+key+" entfernt (muss > 0 sein)")
			}
		}
	}
	if docker, ok := out["docker"].(map[string]any); ok {
		if lifecycle, exists := docker["lifecycle"].(string); exists {
			lifecycle = strings.ToLower(strings.TrimSpace(lifecycle))
			if lifecycle != "auto" && lifecycle != "disabled" && lifecycle != "required" {
				delete(docker, "lifecycle")
				changes.normalized = append(changes.normalized, "global docker.lifecycle entfernt (ungültig)")
			} else {
				docker["lifecycle"] = lifecycle
			}
		}
	}
	if health, ok := out["healthcheck"].(map[string]any); ok {
		if typ, exists := health["type"].(string); exists {
			typ = strings.ToLower(strings.TrimSpace(typ))
			if typ != "" && typ != "none" && typ != "http" && typ != "command" {
				delete(health, "type")
				changes.normalized = append(changes.normalized, "global healthcheck.type entfernt (ungültig)")
			} else {
				health["type"] = typ
			}
		}
		if timeout, exists := health["timeoutSeconds"].(int); exists && timeout < 0 {
			delete(health, "timeoutSeconds")
			changes.normalized = append(changes.normalized, "global healthcheck.timeoutSeconds entfernt (negativ)")
		}
	}
	return out, changes
}

func positiveNumber(value any) bool {
	switch v := value.(type) {
	case int:
		return v > 0
	case int64:
		return v > 0
	case float64:
		return v > 0
	default:
		return false
	}
}

func normalizeNoParameterRepair(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		switch value {
		case "help", "check", "update", "setup", "no-setup":
			seen[value] = true
		}
	}
	if seen["help"] {
		return []string{"help"}
	}
	if seen["check"] {
		seen["update"] = false
		seen["no-setup"] = false
	}
	if seen["no-setup"] && !seen["update"] {
		seen["no-setup"] = false
	}
	if seen["no-setup"] {
		seen["setup"] = false
	}
	out := []string{}
	for _, key := range []string{"check", "update", "setup", "no-setup"} {
		if seen[key] {
			out = append(out, key)
		}
	}
	if len(out) == 0 {
		return []string{"check"}
	}
	return out
}

func readLooseJSONObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, fmt.Errorf("%s enthält ungültiges JSON: %w", path, err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s enthält mehrere JSON-Werte", path)
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: JSON-Wurzel muss ein Objekt sein", path)
	}
	return object, nil
}

func looseString(v any) (string, bool) { s, ok := v.(string); return s, ok }
func looseBool(v any) (bool, bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	case string:
		b, err := strconv.ParseBool(strings.TrimSpace(x))
		return b, err == nil
	default:
		return false, false
	}
}
func looseInt(v any) (int, bool) { n, ok := looseInt64(v); return int(n), ok && int64(int(n)) == n }
func looseInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case json.Number:
		n, err := x.Int64()
		return n, err == nil
	case float64:
		n := int64(x)
		return n, float64(n) == x
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return n, err == nil
	case int:
		return int64(x), true
	case int64:
		return x, true
	default:
		return 0, false
	}
}
func looseFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case json.Number:
		n, err := x.Float64()
		return n, err == nil
	case float64:
		return x, true
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return n, err == nil
	default:
		return 0, false
	}
}
func looseStringList(v any) ([]string, bool) {
	if s, ok := v.(string); ok {
		return []string{s}, true
	}
	values, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		s, ok := value.(string)
		if !ok {
			continue
		}
		out = append(out, s)
	}
	return out, true
}
func uniqueNonEmpty(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
func safePreserveList(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = filepath.ToSlash(strings.TrimSpace(value))
		clean := filepath.ToSlash(filepath.Clean(value))
		if value == "" || filepath.IsAbs(value) || clean == ".." || strings.HasPrefix(clean, "../") || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func safeRelativeProjectPath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || filepath.IsAbs(value) {
		return false
	}
	clean := filepath.Clean(value)
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, ".."+string(os.PathSeparator)) && clean != ConfigDirName && !strings.HasPrefix(clean, ConfigDirName+string(os.PathSeparator))
}
func sanitizeProjectName(value string) string {
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-._")
	if out == "" {
		return "project"
	}
	return out
}
func jsonEquivalent(a, b []byte) bool {
	var av, bv any
	if json.Unmarshal(a, &av) != nil || json.Unmarshal(b, &bv) != nil {
		return bytes.Equal(bytes.TrimSpace(a), bytes.TrimSpace(b))
	}
	return fmt.Sprintf("%#v", av) == fmt.Sprintf("%#v", bv)
}
func marshalLooseJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func backupRawFile(path, label string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	base := fmt.Sprintf("%s.backup-%s-%s", path, label, time.Now().Format("20060102-150405"))
	for i := 0; ; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i+1)
		}
		f, err := os.OpenFile(candidate, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if _, err := f.Write(data); err != nil {
			f.Close()
			os.Remove(candidate)
			return "", err
		}
		if err := f.Close(); err != nil {
			return "", err
		}
		return candidate, nil
	}
}
func atomicWriteBytes(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".fix-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
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
