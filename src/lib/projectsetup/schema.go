package projectsetup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const SchemaVersion = 2

// SchemaJSON returns the canonical JSON Schema for update-cli.yaml schemaVersion 2.
// Project-dependent update settings may be versioned in update-cli.yaml.
// Host/user policy remains in .update-cli/config.json.
func SchemaJSON() ([]byte, error) {
	operations := []string{
		"command", "shell", "mkdir", "copy", "move", "remove", "chmod", "symlink", "touch", "write",
		"assert", "pythonVenv", "pip", "npm", "pnpm", "yarn", "composer", "artisan", "dockerCompose", "go",
		"httpCheck", "download", "extract", "deploy",
	}

	stepProperties := map[string]any{
		"id":              map[string]any{"type": "string"},
		"name":            map[string]any{"type": "string"},
		"cwd":             map[string]any{"type": "string"},
		"env":             map[string]any{"$ref": "#/$defs/environment"},
		"timeout":         map[string]any{"type": "string"},
		"retries":         map[string]any{"type": "integer", "minimum": 0},
		"allowFailure":    map[string]any{"type": "boolean"},
		"continueOnError": map[string]any{"type": "boolean"},
		"when":            map[string]any{"$ref": "#/$defs/condition"},
	}
	operationChoices := make([]any, 0, len(operations))
	for _, operation := range operations {
		stepProperties[operation] = map[string]any{}
		forbidden := make([]any, 0, len(operations)-1)
		for _, other := range operations {
			if other != operation {
				forbidden = append(forbidden, map[string]any{"required": []string{other}})
			}
		}
		operationChoices = append(operationChoices, map[string]any{
			"required": []string{operation},
			"not":      map[string]any{"anyOf": forbidden},
		})
	}

	schema := map[string]any{
		"$schema":     "https://json-schema.org/draft/2020-12/schema",
		"$id":         "https://github.com/r14r/update-cli/schema/update-cli.schema.json",
		"title":       "Update CLI project automation manifest",
		"description": "Canonical schema for update-cli.yaml schemaVersion 2. Project-dependent update settings may override config.json; host security policy remains in config.json.",
		"type":        "object",
		"required":    []string{"schemaVersion"},
		"properties": map[string]any{
			"schemaVersion": map[string]any{"const": SchemaVersion},
			"version":       map[string]any{"type": []string{"string", "number"}},
			"project": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"name":        map[string]any{"type": "string"},
					"slug":        map[string]any{"type": "string"},
					"description": map[string]any{"type": "string"},
					"type":        map[string]any{"type": "string"},
				},
			},
			"defaults": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"timeout":  map[string]any{"type": "string"},
					"failFast": map[string]any{"type": "boolean"},
				},
			},
			"variables": map[string]any{"$ref": "#/$defs/environment"},
			"requirements": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"commands":         map[string]any{"$ref": "#/$defs/stringList"},
					"optionalCommands": map[string]any{"$ref": "#/$defs/stringList"},
				},
			},
			"workflows": map[string]any{
				"type": "object",
				"additionalProperties": map[string]any{
					"$ref": "#/$defs/workflow",
				},
			},
			"tasks": map[string]any{
				"type": "object",
				"additionalProperties": map[string]any{
					"$ref": "#/$defs/task",
				},
			},
			"update": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"mode": map[string]any{"enum": []string{"update", "pull"}},
					"source": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]any{
							"type":       map[string]any{"enum": []string{"download", "url", "repository"}},
							"folder":     map[string]any{"type": "string"},
							"url":        map[string]any{"type": "string"},
							"repository": map[string]any{"type": "string"},
							"ref":        map[string]any{"type": "string"},
							"commit":     map[string]any{"type": "string"},
							"version":    map[string]any{"type": "string"},
							"sha256":     map[string]any{"type": "string"},
						},
					},
					"releaseDir": map[string]any{"type": "string", "minLength": 1},
					"currentDir": map[string]any{"type": "string", "minLength": 1},
					"backup": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]any{
							"directory": map[string]any{"type": "string", "minLength": 1},
							"keep":      map[string]any{"type": "integer", "minimum": 0},
						},
					},
					"retention": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties":           map[string]any{"releases": map[string]any{"type": "integer", "minimum": 0}},
					},
					"sync": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]any{
							"preserve":         map[string]any{"$ref": "#/$defs/stringList"},
							"keepOnSetupError": map[string]any{"type": "boolean"},
						},
					},
					"docker": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties":           map[string]any{"lifecycle": map[string]any{"enum": []string{"auto", "disabled", "required"}}},
					},
					"healthcheck": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]any{
							"type":           map[string]any{"enum": []string{"none", "http", "command"}},
							"url":            map[string]any{"type": "string"},
							"command":        map[string]any{"type": "string"},
							"timeoutSeconds": map[string]any{"type": "integer", "minimum": 0},
						},
					},
				},
			},
			"run": map[string]any{
				"oneOf": []any{
					map[string]any{"type": "string", "minLength": 1},
					map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]any{
							"description":      map[string]any{"type": "string"},
							"command":          map[string]any{"type": "string", "minLength": 1},
							"cwd":              map[string]any{"type": "string"},
							"workingDirectory": map[string]any{"type": "string"},
							"env":              map[string]any{"$ref": "#/$defs/environment"},
							"steps":            map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"$ref": "#/$defs/step"}},
						},
						"oneOf": []any{
							map[string]any{"required": []string{"command"}, "not": map[string]any{"required": []string{"steps"}}},
							map[string]any{"required": []string{"steps"}, "not": map[string]any{"required": []string{"command"}}},
						},
					},
				},
			},
			"legacySetup": map[string]any{"type": "object"},
		},
		"additionalProperties": false,
		"anyOf": []any{
			map[string]any{"required": []string{"tasks"}},
			map[string]any{"required": []string{"run"}},
			map[string]any{"required": []string{"legacySetup"}},
			map[string]any{"required": []string{"update"}},
		},
		"$defs": map[string]any{
			"stringList": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"environment": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": []string{"string", "number", "integer", "boolean"}},
			},
			"condition": map[string]any{
				"oneOf": []any{
					map[string]any{"type": "string"},
					map[string]any{"type": "object", "minProperties": 1, "maxProperties": 1},
				},
			},
			"step": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties":           stepProperties,
				"oneOf":                operationChoices,
			},
			"task": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"description": map[string]any{"type": "string"},
					"requires":    map[string]any{"$ref": "#/$defs/stringList"},
					"steps":       map[string]any{"type": "array", "items": map[string]any{"$ref": "#/$defs/step"}},
				},
				"anyOf": []any{
					map[string]any{"required": []string{"requires"}},
					map[string]any{"required": []string{"steps"}},
				},
			},
			"workflow": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"tasks"},
				"properties": map[string]any{
					"description": map[string]any{"type": "string"},
					"tasks":       map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string"}},
				},
			},
		},
	}

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("JSON-Schema konnte nicht erzeugt werden: %w", err)
	}
	return append(data, '\n'), nil
}

// SaveSchema writes the canonical update-cli.yaml JSON Schema to filename.
func SaveSchema(filename string) (string, error) {
	filename = filepath.Clean(filename)
	if filename == "." || filename == "" {
		return "", fmt.Errorf("Schema-Dateiname fehlt")
	}
	data, err := SchemaJSON()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(filename)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return "", err
	}
	return abs, nil
}
