package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSetNoParameterViaKebabAlias(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	if _, err := Init(root, InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads}); err != nil {
		t.Fatal(err)
	}
	res, err := Set(root, []string{"no-parameter=check,setup"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changes) != 1 || res.Changes[0].Key != "no parameter" {
		t.Fatalf("unexpected changes: %#v", res.Changes)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.NoParameterActions, []string{"check", "setup"}) {
		t.Fatalf("no parameter = %#v, want [check setup]", cfg.NoParameterActions)
	}

	var raw map[string]any
	b, err := os.ReadFile(filepath.Join(root, ConfigDirName, ConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	if got := raw["no parameter"]; !reflect.DeepEqual(got, []any{"check", "setup"}) {
		t.Fatalf("persisted no parameter = %#v", got)
	}
}

func TestSetNestedTypedValuesTransactionally(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	if _, err := Init(root, InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads}); err != nil {
		t.Fatal(err)
	}
	_, err := Set(root, []string{
		"backup.keep=9",
		"retention.releases=11",
		"security.allow-http=true",
		"healthcheck.type=command",
		"healthcheck.command=./bin/demo doctor",
		"sync.preserve=.env,data/,.cache/",
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.KeepBackups != 9 || cfg.KeepReleases != 11 || !cfg.Security.AllowHTTP {
		t.Fatalf("typed values not applied: %#v", cfg)
	}
	if cfg.Healthcheck.Type != "command" || cfg.Healthcheck.Command != "./bin/demo doctor" {
		t.Fatalf("healthcheck not applied: %#v", cfg.Healthcheck)
	}
	wantPreserve := []string{".env", "data/", ".cache/", ".gitignore"}
	if !reflect.DeepEqual(cfg.Preserve, wantPreserve) {
		t.Fatalf("preserve = %#v, want %#v", cfg.Preserve, wantPreserve)
	}
}

func TestSetCanChangeSourceWithMultipleAssignments(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	if _, err := Init(root, InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads}); err != nil {
		t.Fatal(err)
	}
	_, err := Set(root, []string{
		"source.type=url",
		"source.url=https://example.invalid/demo-v1.2.3.zip",
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Source.Type != "url" || cfg.Source.URL != "https://example.invalid/demo-v1.2.3.zip" {
		t.Fatalf("source = %#v", cfg.Source)
	}
}

func TestSetRejectsUnknownKeyWithoutWriting(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	if _, err := Init(root, InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ConfigDirName, ConfigFileName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Set(root, []string{"does-not-exist=value"}); err == nil {
		t.Fatal("expected unknown key error")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("config changed after rejected key")
	}
}

func TestSetRejectsInvalidCombinedConfigurationWithoutWriting(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	if _, err := Init(root, InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ConfigDirName, ConfigFileName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Set(root, []string{"backup.keep=-1"}); err == nil {
		t.Fatal("expected validation error")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("config changed after rejected value")
	}
}

func TestSetDockerLifecycle(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	if _, err := Init(root, InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads}); err != nil {
		t.Fatal(err)
	}
	if _, err := Set(root, []string{"docker.lifecycle=disabled"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Docker.Lifecycle != "disabled" {
		t.Fatalf("Docker lifecycle = %q", cfg.Docker.Lifecycle)
	}
}
