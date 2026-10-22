package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const globalConfigDirEnv = "UPDATE_CLI_GLOBAL_CONFIG_DIR"

// GlobalConfigDir returns the installation-wide configuration directory.
// For an executable installed as /usr/local/bin/update-cli this resolves to
// /usr/local/etc/update-cli. UPDATE_CLI_GLOBAL_CONFIG_DIR is intentionally
// supported as an explicit override for tests and non-standard packaging.
func GlobalConfigDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv(globalConfigDirEnv)); override != "" {
		return expandAndAbs(override)
	}
	executable, err := invokedExecutablePath()
	if err != nil {
		return "", fmt.Errorf("globalen Konfigurationspfad ermitteln: %w", err)
	}
	return globalConfigDirFromExecutable(executable)
}

func invokedExecutablePath() (string, error) {
	if len(os.Args) > 0 {
		arg0 := strings.TrimSpace(os.Args[0])
		if arg0 != "" {
			if strings.ContainsRune(arg0, os.PathSeparator) {
				if absolute, err := filepath.Abs(arg0); err == nil {
					return absolute, nil
				}
			}
			if found, err := exec.LookPath(arg0); err == nil {
				if absolute, absErr := filepath.Abs(found); absErr == nil {
					return absolute, nil
				}
			}
		}
	}
	return os.Executable()
}

func globalConfigDirFromExecutable(executable string) (string, error) {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return "", errors.New("Executable-Pfad ist leer")
	}
	absolute, err := filepath.Abs(executable)
	if err != nil {
		return "", err
	}
	installFolder := filepath.Dir(absolute)
	prefix := filepath.Dir(installFolder)
	return filepath.Clean(filepath.Join(prefix, "etc", "update-cli")), nil
}

// readMergedConfigFile loads installation-wide defaults first and overlays the
// project-local config. Objects are merged recursively; arrays and scalar
// values from the local file replace the global value. The local file is
// mandatory, while the global file is optional.
func readMergedConfigFile(localPath, globalPath string) (FileConfig, error) {
	global, err := readJSONObject(globalPath, false)
	if err != nil {
		return FileConfig{}, fmt.Errorf("globale config.json %s: %w", globalPath, err)
	}
	normalizeLegacySetupPolicy(global)
	local, err := readJSONObject(localPath, true)
	if err != nil {
		return FileConfig{}, fmt.Errorf("lokale config.json %s: %w", localPath, err)
	}
	normalizeLegacySetupPolicy(local)
	merged := mergeJSONObject(global, local)
	mergePreserveLists(global, local, merged)
	data, err := json.Marshal(merged)
	if err != nil {
		return FileConfig{}, err
	}
	return decodeFileConfig(data)
}

func readJSONObject(path string, required bool) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && !required {
		return map[string]json.RawMessage{}, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("Updater-Konfiguration fehlt: %s", path)
	}
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	var object map[string]json.RawMessage
	if err := dec.Decode(&object); err != nil {
		return nil, fmt.Errorf("ungültiges JSON: %w", err)
	}
	if object == nil {
		return nil, errors.New("JSON-Wurzel muss ein Objekt sein")
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("enthält mehrere JSON-Werte")
	}
	return object, nil
}

// normalizeLegacySetupPolicy accepts the runtime-policy locations emitted by
// older transitional releases before the strict decoder runs. The canonical
// config.json location remains setup.keepRsyncOnError. Project YAML uses the
// separate update.sync.keepOnSetupError field. Known aliases are removed even
// when their value is invalid so an obsolete field can never make every normal
// command unusable; `update-cli fix` persists the same normalization on disk.
func normalizeLegacySetupPolicy(object map[string]json.RawMessage) {
	if object == nil {
		return
	}

	setup := rawObject(object["setup"])
	if setup == nil {
		setup = map[string]json.RawMessage{}
	}
	_, canonicalExists := setup["keepRsyncOnError"]

	type candidate struct {
		parent map[string]json.RawMessage
		key    string
	}
	syncObject := rawObject(object["sync"])
	candidates := []candidate{
		{setup, "keepOnSetupError"},
	}
	if syncObject != nil {
		candidates = append(candidates, candidate{syncObject, "keepRsyncOnError"}, candidate{syncObject, "keepOnSetupError"})
	}
	candidates = append(candidates, candidate{object, "keepRsyncOnError"}, candidate{object, "keepOnSetupError"})

	for _, item := range candidates {
		raw, exists := item.parent[item.key]
		if !exists {
			continue
		}
		if !canonicalExists {
			if value, ok := rawCompatibilityBool(raw); ok {
				encoded, _ := json.Marshal(value)
				setup["keepRsyncOnError"] = encoded
				canonicalExists = true
			}
		}
		delete(item.parent, item.key)
	}

	if len(setup) > 0 {
		if encoded, err := json.Marshal(setup); err == nil {
			object["setup"] = encoded
		}
	}
	if syncObject != nil {
		if len(syncObject) == 0 {
			delete(object, "sync")
		} else if encoded, err := json.Marshal(syncObject); err == nil {
			object["sync"] = encoded
		}
	}
}

func rawObject(raw json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil {
		return nil
	}
	return object
}

func rawCompatibilityBool(raw json.RawMessage) (bool, bool) {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return false, false
	}
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "yes", "1", "on":
			return true, true
		case "false", "no", "0", "off":
			return false, true
		}
	case float64:
		if v == 0 {
			return false, true
		}
		if v == 1 {
			return true, true
		}
	}
	return false, false
}

// mergePreserveLists treats sync.preserve as cumulative defaults: installation-
// wide entries are kept and project-local entries are appended without
// duplicates. This is the one array with additive semantics because the global
// file defines the minimum rsync preserve/exclude policy.
func mergePreserveLists(global, local, merged map[string]json.RawMessage) {
	globalValues, globalOK := nestedStringArray(global, "sync", "preserve")
	localValues, localOK := nestedStringArray(local, "sync", "preserve")
	if !globalOK || !localOK {
		return
	}
	values := make([]string, 0, len(globalValues)+len(localValues))
	seen := map[string]bool{}
	for _, value := range append(globalValues, localValues...) {
		key := filepath.ToSlash(strings.TrimSpace(value))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		values = append(values, value)
	}
	syncObject := map[string]json.RawMessage{}
	if raw, ok := merged["sync"]; ok {
		_ = json.Unmarshal(raw, &syncObject)
	}
	data, err := json.Marshal(values)
	if err != nil {
		return
	}
	syncObject["preserve"] = data
	if data, err = json.Marshal(syncObject); err == nil {
		merged["sync"] = data
	}
}

func nestedStringArray(object map[string]json.RawMessage, parent, child string) ([]string, bool) {
	raw, ok := object[parent]
	if !ok {
		return nil, false
	}
	var nested map[string]json.RawMessage
	if json.Unmarshal(raw, &nested) != nil || nested == nil {
		return nil, false
	}
	raw, ok = nested[child]
	if !ok {
		return nil, false
	}
	var values []string
	if json.Unmarshal(raw, &values) != nil {
		return nil, false
	}
	return values, true
}

func mergeJSONObject(base, overlay map[string]json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(base)+len(overlay))
	for key, value := range base {
		out[key] = append(json.RawMessage(nil), value...)
	}
	for key, overlayValue := range overlay {
		baseValue, exists := out[key]
		if exists {
			var baseObject map[string]json.RawMessage
			var overlayObject map[string]json.RawMessage
			if json.Unmarshal(baseValue, &baseObject) == nil && baseObject != nil && json.Unmarshal(overlayValue, &overlayObject) == nil && overlayObject != nil {
				merged := mergeJSONObject(baseObject, overlayObject)
				if data, err := json.Marshal(merged); err == nil {
					out[key] = data
					continue
				}
			}
		}
		out[key] = append(json.RawMessage(nil), overlayValue...)
	}
	return out
}

func decodeFileConfig(data []byte) (FileConfig, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var fc FileConfig
	if err := dec.Decode(&fc); err != nil {
		return fc, fmt.Errorf("config.json enthält ungültiges JSON: %w", err)
	}
	return fc, nil
}
