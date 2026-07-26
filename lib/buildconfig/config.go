package buildconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const SchemaVersion = 1

// Config contains defaults selected by the distributor when update-cli is
// built. The JSON file is embedded into the binary by main.go.
type Config struct {
	SchemaVersion         int    `json:"schemaVersion"`
	DefaultDownloadFolder string `json:"defaultDownloadFolder"`
	DefaultDeploymentPath string `json:"defaultDeploymentPath"`
	DefaultConfigPath     string `json:"defaultConfigPath"`
}

var (
	mu      sync.RWMutex
	current = Config{
		SchemaVersion:         SchemaVersion,
		DefaultDownloadFolder: "$HOME/Downloads",
		DefaultDeploymentPath: "/usr/local/bin",
		DefaultConfigPath:     "/usr/local/etc/update-cli",
	}
)

func Parse(data []byte) (Config, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var value Config
	if err := decoder.Decode(&value); err != nil {
		return Config{}, fmt.Errorf("build-config.json enthält ungültiges JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return Config{}, fmt.Errorf("build-config.json enthält ungültige Zusatzdaten: %w", err)
		}
		return Config{}, errors.New("build-config.json enthält mehrere JSON-Werte")
	}
	if err := Validate(value); err != nil {
		return Config{}, err
	}
	return value, nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("Build-Konfiguration kann nicht gelesen werden: %w", err)
	}
	return Parse(data)
}

func Validate(value Config) error {
	if value.SchemaVersion != SchemaVersion {
		return fmt.Errorf("nicht unterstützte schemaVersion %d in build-config.json; erwartet wird %d", value.SchemaVersion, SchemaVersion)
	}
	for name, raw := range map[string]string{
		"defaultDownloadFolder": value.DefaultDownloadFolder,
		"defaultDeploymentPath": value.DefaultDeploymentPath,
		"defaultConfigPath":     value.DefaultConfigPath,
	} {
		if strings.TrimSpace(raw) == "" {
			return fmt.Errorf("%s fehlt oder ist leer", name)
		}
		if strings.ContainsRune(raw, '\x00') {
			return fmt.Errorf("%s enthält ein ungültiges Nullzeichen", name)
		}
	}
	return nil
}

func Set(value Config) error {
	if err := Validate(value); err != nil {
		return err
	}
	mu.Lock()
	current = value
	mu.Unlock()
	return nil
}

func Current() Config {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

func ExpandPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("Pfad ist leer")
	}
	value = os.ExpandEnv(value)
	if value == "~" || strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("Home-Verzeichnis kann nicht ermittelt werden: %w", err)
		}
		if value == "~" {
			value = home
		} else {
			value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
		}
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absolute), nil
}

func GlobalTemplatesFile() (string, error) {
	path, err := ExpandPath(Current().DefaultConfigPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(path, "templates.json"), nil
}
