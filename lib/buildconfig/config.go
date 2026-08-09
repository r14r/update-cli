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

type Config struct {
	SchemaVersion         int    `json:"schemaVersion"`
	DefaultDownloadFolder string `json:"defaultDownloadFolder"`
	DefaultDeploymentPath string `json:"defaultDeploymentPath"`
	DefaultConfigPath     string `json:"defaultConfigPath"`
}

var mu sync.RWMutex
var current = Config{SchemaVersion: 1, DefaultDownloadFolder: "$HOME/Downloads", DefaultDeploymentPath: "/usr/local/bin", DefaultConfigPath: "/usr/local/etc/update-cli"}

func Parse(data []byte) (Config, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var c Config
	if err := dec.Decode(&c); err != nil {
		return c, fmt.Errorf("build-config.json enthält ungültiges JSON: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return c, err
		}
		return c, errors.New("build-config.json enthält mehrere JSON-Werte")
	}
	if err := Validate(c); err != nil {
		return c, err
	}
	return c, nil
}
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return Parse(data)
}
func Validate(c Config) error {
	if c.SchemaVersion != 1 {
		return fmt.Errorf("nicht unterstützte schemaVersion %d", c.SchemaVersion)
	}
	for k, v := range map[string]string{"defaultDownloadFolder": c.DefaultDownloadFolder, "defaultDeploymentPath": c.DefaultDeploymentPath, "defaultConfigPath": c.DefaultConfigPath} {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("%s fehlt oder ist leer", k)
		}
	}
	return nil
}
func Set(c Config) error {
	if err := Validate(c); err != nil {
		return err
	}
	mu.Lock()
	current = c
	mu.Unlock()
	return nil
}
func Current() Config { mu.RLock(); defer mu.RUnlock(); return current }
func ExpandPath(v string) (string, error) {
	v = os.ExpandEnv(strings.TrimSpace(v))
	if v == "" {
		return "", errors.New("Pfad ist leer")
	}
	if v == "~" || strings.HasPrefix(v, "~/") {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		v = filepath.Join(h, strings.TrimPrefix(v, "~/"))
	}
	a, err := filepath.Abs(v)
	if err != nil {
		return "", err
	}
	return filepath.Clean(a), nil
}
func GlobalTemplatesFile() (string, error) {
	p, err := ExpandPath(Current().DefaultConfigPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(p, "templates.json"), nil
}
