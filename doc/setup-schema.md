# Update CLI setup manifest schema

Update CLI 2.11 combines versioned project update policy with setup automation while keeping host/user policy in JSON.

## File responsibilities

```text
.update-cli/config.json   host/user runtime defaults and local fallbacks
update-cli.yaml           preferred project update/setup/run manifest
current/update-cli.yaml   installed release manifest (doctor also checks it)
setup.yaml                legacy setup manifest fallback
```

The manifest does not need to contain updater settings. When project-dependent values are present under `project.slug` or `update:`, they override the corresponding JSON fallback values.

## Discovery

Setup manifest discovery order:

1. `update-cli.yaml`
2. `setup.yaml`
3. `setup.sh`
4. `just build` then `just install` fallback when a Justfile exists

If both YAML names exist, `update-cli.yaml` wins.

## SchemaVersion 2

Example:

```yaml
schemaVersion: 2

project:
  name: Demo
  slug: demo
  type: go

defaults:
  timeout: 10m
  failFast: true

run:
  command: ./dist/demo

workflows:
  setup:
    tasks: [deploy]

tasks:
  build:
    steps:
      - shell: go build -o dist/demo .

  deploy:
    requires: [build]
    steps:
      - deploy:
          source: dist/demo
          target: /usr/local/bin/demo
          mode: "0755"
```

Schema 2 supports reusable tasks, workflows, variables, requirements, structured conditions, retries, timeouts, environment values and typed operations.

Schema 2 supports a project-versioned `update:` block for settings that belong to the project/release: source and mode, release/current directories, backup/retention, `sync.preserve`, setup recovery policy, Docker lifecycle and healthcheck. `security.*`, `source.defaultUser`, CLI defaults and templates remain JSON-only host/user policy.

## Legacy `setup.yaml`

`setup.yaml` is accepted without renaming. If it has no `schemaVersion`/legacy integer `version`, Update CLI infers the schema:

- task/workflow/run/defaults/variables/requirements structures → schema 2
- older step-oriented structures → schema 1

Error messages use the actual manifest filename where possible.

## SchemaVersion 1

Historical schemaVersion-1 manifests remain supported, including the simple `steps:` format and the structured legacy project/build/runtime/setup format.

Migration to schema 2 is recommended but not required for existing projects that still run correctly.

## Project update configuration

A project can place portable updater values in the manifest:

```yaml
schemaVersion: 2
project:
  name: Demo
  slug: demo

update:
  mode: update
  source:
    type: download
    folder: $HOME/Downloads
  sync:
    preserve: [.env, .env.*, data/, uploads/]
    keepOnSetupError: false
  docker:
    lifecycle: auto
  healthcheck:
    type: none

run:
  command: ./dist/demo
```

The runtime JSON remains the place for host/user defaults and security policy:

```json
{
  "schemaVersion": 9,
  "projectName": "demo",
  "source": {
    "type": "download",
    "defaultUser": "r1r",
    "folder": "$HOME/Downloads"
  },
  "security": {
    "allowHttp": false,
    "maxArchiveBytes": 2147483648,
    "maxUncompressedBytes": 8589934592,
    "maxFileBytes": 2147483648,
    "maxEntries": 100000,
    "maxCompressionRatio": 200
  }
}
```

Priority is `global config.json < local .update-cli/config.json < update-cli.yaml < explicit CLI options`.

Legacy `.update-cli/config.json` remains readable and can be migrated with:

```bash
update-cli config --migrate
```
