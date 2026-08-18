package config

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

// SetResult describes a successful config mutation.
type SetResult struct {
	ConfigFile string      `json:"configFile"`
	Changes    []SetChange `json:"changes"`
}

// SetChange contains the canonical JSON path and the resulting value.
type SetChange struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

// Set updates one or more values in .updater-cli/config.json. Assignments use
// dotted JSON paths, for example "retention.releases=7". Path matching is
// tolerant of camelCase, kebab-case, snake_case and spaces, so
// "no-parameter=check,setup" addresses the JSON key "no parameter".
//
// All assignments are applied in memory and the complete configuration is
// validated before it is written. Multiple --set arguments are therefore
// transactional with respect to validation.
func Set(root string, assignments []string) (SetResult, error) {
	if len(assignments) == 0 {
		return SetResult{}, errors.New("config --set benötigt KEY=VALUE")
	}
	root, err := absoluteDir(root)
	if err != nil {
		return SetResult{}, err
	}
	path := filepath.Join(root, ConfigDirName, ConfigFileName)
	original, err := os.ReadFile(path)
	if err != nil {
		return SetResult{}, err
	}
	fc, err := readConfigFile(root, path)
	if err != nil {
		return SetResult{}, err
	}
	result := SetResult{ConfigFile: path}
	for _, assignment := range assignments {
		key, raw, ok := strings.Cut(assignment, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return SetResult{}, fmt.Errorf("ungültige --set Angabe %q; erwartet KEY=VALUE", assignment)
		}
		canonical, value, err := setConfigPath(reflect.ValueOf(&fc), key, raw)
		if err != nil {
			return SetResult{}, err
		}
		result.Changes = append(result.Changes, SetChange{Key: canonical, Value: value})
	}

	fc, _, err = migrate(fc)
	if err != nil {
		return SetResult{}, fmt.Errorf("Konfigurationsänderung ist ungültig: %w", err)
	}
	if err := validateFile(fc); err != nil {
		return SetResult{}, fmt.Errorf("Konfigurationsänderung ist ungültig: %w", err)
	}
	if err := writeConfigFile(path, fc); err != nil {
		return SetResult{}, err
	}
	if _, err := Load(root, ""); err != nil {
		if restoreErr := writeRawConfigFile(path, original); restoreErr != nil {
			return SetResult{}, fmt.Errorf("Konfigurationsänderung ist ungültig (%v); Wiederherstellung der vorherigen config.json fehlgeschlagen: %w", err, restoreErr)
		}
		return SetResult{}, fmt.Errorf("Konfigurationsänderung ist ungültig; vorherige config.json wurde wiederhergestellt: %w", err)
	}
	return result, nil
}

func setConfigPath(root reflect.Value, path, raw string) (string, any, error) {
	segments := strings.Split(path, ".")
	for _, segment := range segments {
		if strings.TrimSpace(segment) == "" {
			return "", nil, fmt.Errorf("ungültiger Config-Key %q", path)
		}
	}

	v := root
	canonical := make([]string, 0, len(segments))
	for i, segment := range segments {
		for v.Kind() == reflect.Pointer {
			if v.IsNil() {
				if !v.CanSet() {
					return "", nil, fmt.Errorf("Config-Key %q kann nicht gesetzt werden", path)
				}
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return "", nil, fmt.Errorf("Config-Key %q ist kein gültiger Pfad", path)
		}
		field, jsonName, ok := findJSONField(v, segment)
		if !ok {
			return "", nil, fmt.Errorf("unbekannter Config-Key %q", strings.Join(segments[:i+1], "."))
		}
		canonical = append(canonical, jsonName)
		if i == len(segments)-1 {
			if err := assignConfigValue(field, raw); err != nil {
				return "", nil, fmt.Errorf("Config-Key %q: %w", strings.Join(canonical, "."), err)
			}
			return strings.Join(canonical, "."), configValueInterface(field), nil
		}
		v = field
	}
	return "", nil, fmt.Errorf("ungültiger Config-Key %q", path)
}

func findJSONField(v reflect.Value, requested string) (reflect.Value, string, bool) {
	wanted := normalizedConfigKey(requested)
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		fieldType := t.Field(i)
		if fieldType.PkgPath != "" { // unexported
			continue
		}
		jsonName := strings.Split(fieldType.Tag.Get("json"), ",")[0]
		if jsonName == "" {
			jsonName = fieldType.Name
		}
		if jsonName == "-" {
			continue
		}
		if normalizedConfigKey(jsonName) == wanted || normalizedConfigKey(fieldType.Name) == wanted {
			return v.Field(i), jsonName, true
		}
	}
	return reflect.Value{}, "", false
}

func normalizedConfigKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func assignConfigValue(v reflect.Value, raw string) error {
	if !v.CanSet() {
		return errors.New("Wert kann nicht gesetzt werden")
	}

	if v.Kind() == reflect.Pointer {
		if strings.EqualFold(strings.TrimSpace(raw), "null") {
			v.Set(reflect.Zero(v.Type()))
			return nil
		}
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return assignConfigValue(v.Elem(), raw)
	}

	switch v.Kind() {
	case reflect.String:
		v.SetString(parseConfigString(raw))
		return nil
	case reflect.Bool:
		value, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("%q ist kein Boolean (true/false)", raw)
		}
		v.SetBool(value)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, v.Type().Bits())
		if err != nil {
			return fmt.Errorf("%q ist keine gültige Ganzzahl", raw)
		}
		v.SetInt(value)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value, err := strconv.ParseUint(strings.TrimSpace(raw), 10, v.Type().Bits())
		if err != nil {
			return fmt.Errorf("%q ist keine gültige positive Ganzzahl", raw)
		}
		v.SetUint(value)
		return nil
	case reflect.Float32, reflect.Float64:
		value, err := strconv.ParseFloat(strings.TrimSpace(raw), v.Type().Bits())
		if err != nil {
			return fmt.Errorf("%q ist keine gültige Zahl", raw)
		}
		v.SetFloat(value)
		return nil
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.String {
			return fmt.Errorf("Listen vom Typ %s werden nicht unterstützt", v.Type())
		}
		values, err := parseStringList(raw)
		if err != nil {
			return err
		}
		out := reflect.MakeSlice(v.Type(), len(values), len(values))
		for i, value := range values {
			out.Index(i).SetString(value)
		}
		v.Set(out)
		return nil
	case reflect.Struct:
		if strings.TrimSpace(raw) == "" {
			return errors.New("Objektwert darf nicht leer sein; JSON-Objekt angeben")
		}
		ptr := reflect.New(v.Type())
		if err := json.Unmarshal([]byte(raw), ptr.Interface()); err != nil {
			return fmt.Errorf("ungültiges JSON-Objekt: %w", err)
		}
		v.Set(ptr.Elem())
		return nil
	default:
		return fmt.Errorf("Typ %s wird für --set nicht unterstützt", v.Type())
	}
}

func parseConfigString(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) >= 2 && trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"' {
		var decoded string
		if json.Unmarshal([]byte(trimmed), &decoded) == nil {
			return decoded
		}
	}
	return raw
}

func parseStringList(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}, nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var values []string
		if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
			return nil, fmt.Errorf("ungültige JSON-Liste: %w", err)
		}
		return trimStrings(values), nil
	}
	r := csv.NewReader(strings.NewReader(raw))
	r.TrimLeadingSpace = true
	record, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("ungültige Liste: %w", err)
	}
	return trimStrings(record), nil
}

func trimStrings(values []string) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = strings.TrimSpace(value)
	}
	return out
}

func configValueInterface(v reflect.Value) any {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	return v.Interface()
}

func writeRawConfigFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".config-restore-*.json")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	_ = f.Chmod(0o644)
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
