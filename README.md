# Update CLI

`update-cli` installs versioned application releases into a stable `current/` directory while keeping validated release copies, backups, history, setup automation, rollback, and recovery state.

Version: **3.1.2**

## Core safety model

A real update is transactional:

1. Resolve and validate the release source.
2. Build a validated hidden release staging directory.
3. Detect whether the existing Docker Compose stack is actually running.
4. Stop that stack when necessary.
5. Create an exact temporary transaction snapshot of `current/`.
6. Optionally create a persistent user backup with `--backup`.
7. Synchronize the staged release into `current/` while protecting configured persistent paths.
8. Verify the installed marker and checksum-level rsync equivalence.
9. Run `setup.yaml` when requested or confirmed interactively.
10. Restore the previous service-running state.
11. Run the configured health check.
12. Activate the immutable release directory and commit the transaction.

Any failure after the transaction begins restores the previous `current/` snapshot. A Compose stack that was running before the operation is brought back after recovery.

## Installation layout

Defaults embedded into the binary:

- binary: `/usr/local/bin/update-cli`
- global configuration: `/usr/local/etc/update-cli`
- local release source: `$HOME/Downloads`

Project layout:

```text
project/
├── .updater-cli/
│   ├── config.json
│   ├── templates.json
│   ├── history.jsonl
│   ├── logs/
│   └── transactions/
├── release/
├── current/
├── backup/
└── .release-update.lock/
```

## Basic commands

```bash
update-cli --init my-project
update-cli --check
update-cli --update
update-cli --update --backup
update-cli --update --plan
update-cli --update --force
update-cli --rollback
update-cli --restore latest
update-cli --status
update-cli --list
update-cli --doctor
update-cli --history
update-cli --cleanup
update-cli --setup
update-cli --unlock
```

Use `update-cli --help` for the short command list and `update-cli --howto` for operational details.

## Fullscreen TUI

Interactive `--check`, real `--update`, and setup runs use the fullscreen TUI by default. The behavior is compatible with the 2.14 line:

- fixed header and footer
- dedicated project/setup information area above the steps
- framed, automatically scrolling installation-step area
- one persistent row per step; successful rows end with a green `✓` instead of emitting a second `OK ... abgeschlossen` line
- centered confirmation modal with separate `YES` / `NO` buttons; the footer remains status-only
- automatic terminal line wrap disabled while fullscreen is active
- rune-based width handling so German umlauts do not shift vertical borders
- compact output after leaving the alternate screen
- wait for Enter after success or failure by default

Controls:

```bash
UPDATE_CLI_TUI=auto update-cli --update
UPDATE_CLI_TUI=fullscreen update-cli --check
UPDATE_CLI_TUI=plain update-cli --update
update-cli --update --no-wait
update-cli --check --no-ask --no-wait
update-cli --update --no-ui
update-cli --setup --no-ui
```

`--no-ui` disables the TUI entirely and streams setup/process stdout and stderr directly to the terminal. It keeps normal color output unless `--no-color`/`NO_COLOR=1` is also used. The compatibility spelling `---no-ui` is accepted, but `--no-ui` is the documented form.

`--no-color`, `NO_COLOR=1`, JSON output, and non-interactive stdout automatically avoid fullscreen rendering. `--wait` remains accepted explicitly; `--no-wait` is the opt-out from the default fullscreen wait behavior.

## Release sources

### Local download directory

```json
{
  "source": {
    "type": "download",
    "folder": "$HOME/Downloads"
  }
}
```

Archive naming:

```text
<PROJECT>-v<MAJOR>.<MINOR>.<PATCH>.zip
```

### HTTPS URL

```json
{
  "source": {
    "type": "url",
    "url": "https://downloads.example.com/demo-v1.2.3.zip",
    "sha256": "...optional expected SHA-256..."
  }
}
```

HTTP is blocked unless `security.allowHttp` is explicitly enabled. URL metadata checks use `HEAD`, with a range-request fallback for servers that reject `HEAD`; a normal `--check` does not need to download the complete archive.

### Git repository

```json
{
  "source": {
    "type": "repository",
    "repository": "https://github.com/example/project.git",
    "ref": "v1.2.3",
    "commit": "optional-pinned-commit",
    "version": "1.2.3"
  }
}
```

Repository releases are normalized through the same artifact policy as ZIP releases. Symbolic links and unsupported special files are rejected.

## No-parameter action

A project can define what `update-cli` does when started without CLI arguments. The historical JSON key remains `"no parameter"`.

```json
{
  "no parameter": ["check"]
}
```

Supported values are `help`, `check`, `update`, and `setup`. `check` and `help` are standalone actions; `update` may be combined with `setup`.

## Persistent paths

The following paths are protected by default during `release -> current` synchronization:

```text
.git/
.venv/
.env
.env.*
data/
storage/
uploads/
media/
logs/
var/
```

Customize them in `config.json`:

```json
{
  "sync": {
    "preserve": [".env", "data/", "storage/"]
  }
}
```

Protected paths are neither overwritten by release content nor deleted by rsync.

## Backups vs transaction snapshots

These are deliberately different concepts.

### Transaction snapshot

Created automatically for update, rollback, and restore. It is an exact temporary copy used only for failure recovery and is removed after a successful commit.

### User backup

Created with:

```bash
update-cli --backup
update-cli --update --backup
```

Regenerable dependencies and secret-bearing `.env` files are excluded. User backups are retained according to `backup.keep`.

## Archive security limits

Schema v6 adds resource limits:

```json
{
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

ZIP validation rejects:

- absolute paths and path traversal
- symbolic links
- oversized entries
- excessive entry counts
- excessive total expanded size
- suspicious compression ratios
- CRC/read failures

## Setup-Dateien verwalten

Update CLI 3.2.0 kann die Setup-Dateien eines Projekts selbst migrieren bzw. erzeugen.
Die Befehle verwenden bei einem konfigurierten Update-CLI-Projekt den `currentDir`; ohne Updater-Konfiguration den aktuellen Projektordner.

```bash
update-cli --convert-yaml
update-cli --create-yaml
update-cli --create-setup-script
```

`--convert-yaml` migriert ein vorhandenes schemaVersion-1-Manifest auf das aktuelle schemaVersion 2 und legt vor dem Ersetzen ein Backup an. Ein bereits aktuelles Manifest bleibt unverändert.

`--create-yaml` erkennt vorhandene Projektdateien und erzeugt ein schemaVersion-2-Beispiel. Erkannt werden Go (`go.mod`), Python (`pyproject.toml`, `requirements.txt`, `setup.py`, `Pipfile`), Node (`package.json` und Lockfiles), Laravel (`artisan`/`composer.json` bzw. `laravel/framework`) und Docker Compose. Mehrere Stacks können gleichzeitig erkannt werden.

`--create-setup-script` erzeugt ein ausführbares generisches `setup.sh`, das die Ausführung an einen kompatiblen Update-CLI-Setup-Handler delegiert. Die Kompatibilitätsform `-create-setup-script` wird ebenfalls akzeptiert.

Vorhandene erzeugte Dateien werden ohne `--force` nicht überschrieben. Mit `--dry-run` wird die geplante YAML-/Script-Ausgabe nur angezeigt.

## `setup.yaml`

Update CLI 3.1.0 introduces **schemaVersion 2** as a declarative project automation format while keeping all schema-1 manifests compatible.

Schema 2 separates reusable **tasks** from entry-point **workflows**. A project can therefore describe operations such as `prepare`, `clean`, `check`, `test`, `build`, `verify`, `deploy`, `start`, `stop`, and `restart` once and compose them into workflows.

```yaml
schemaVersion: 2

project:
  name: Example CLI
  type: go

variables:
  binary: example
  distDir: dist

requirements:
  commands: [go]

workflows:
  setup:
    tasks: [deploy]
  ci:
    tasks: [verify]

tasks:
  prepare:
    steps:
      - name: Download modules
        go:
          action: mod-download

  check:
    requires: [prepare]
    steps:
      - name: Static analysis
        go:
          action: vet
      - name: Tests
        go:
          action: test

  build:
    requires: [check]
    steps:
      - name: Build
        shell: |
          mkdir -p "{{ distDir }}"
          go build -o "{{ distDir }}/{{ binary }}" .

  verify:
    requires: [build]
    steps:
      - name: Verify binary
        assert:
          executable: "{{ distDir }}/{{ binary }}"

  deploy:
    requires: [verify]
    steps:
      - name: Deploy
        deploy:
          source: "{{ distDir }}/{{ binary }}"
          target: /usr/local/bin/example
          mode: "0755"
```

Run the default `setup` workflow or select individual project operations:

```bash
update-cli --setup
update-cli --setup-list
update-cli --setup-task test
update-cli --setup-task clean
update-cli --setup-workflow ci
update-cli --setup-workflow setup --no-ui
```

Schema 2 includes:

- task dependencies with cycle detection and de-duplication
- variables and `{{ env.NAME | fallback }}` expansion
- required/optional tool checks
- `all` / `any` / `not` conditions
- file, directory, command, environment, OS and architecture conditions
- per-step `cwd`, `env`, `timeout`, `retries`, and `allowFailure`
- structured `command` plus unrestricted `shell` as the escape hatch
- filesystem operations: `mkdir`, `copy`, `move`, `remove`, `chmod`, `symlink`, `touch`, `write`, `deploy`
- assertions for files, directories, executables, commands, environment, ports, and HTTP
- environment helpers: `pythonVenv`, `pip`
- Go, npm/pnpm/yarn, Composer/Artisan and Docker Compose operations
- HTTP checks, checksum-capable downloads, and safe ZIP extraction

The complete schema is documented in `doc/setup-schema.md`.

Existing schema-1 files remain valid, including the established 2.14 format:

```yaml
schemaVersion: 1
project:
  name: Existing Project
  type: go
steps:
  - id: test
    name: Run tests
    when: file:go.mod
    run: go test ./...
    cwd: .
    allowFailure: false
```

The reusable setup wrapper supports task/workflow selection too:

```bash
./setup.sh
./setup.sh --list
./setup.sh --task build
./setup.sh --workflow ci
./setup.sh --details
./setup.sh --no-ui
```

A manifest can also be selected explicitly:

```bash
update-cli --setup-manifest ./setup.yaml
update-cli --setup-manifest ./setup.yaml --setup-list
update-cli --setup-manifest ./setup.yaml --setup-task build
```

After an interactive update, Update CLI still runs the manifest's default `setup` workflow when setup is selected. In fullscreen mode, confirmations are shown in a centered modal with separate `YES` / `NO` buttons and default to **NO**. Plain and `--no-ui` output retain the compact `[j/N]` prompt. `--setup` and `--no-setup` make that choice deterministic.

## Update transaction progress and errors

A real update is rendered as an explicit 13-step transaction:

1. resolve the release source
2. validate target version/update policy
3. validate archive/repository content
4. prepare the versioned release stage
5. prepare the transaction and current snapshot
6. create the optional persistent backup
7. synchronize the release to `current`
8. verify the installed `current` tree
9. run or explicitly skip project setup
10. restart services that were running before the update
11. run the configured health check
12. activate the versioned release
13. write state and commit the transaction

Optional stages are shown as `SKIP` with the reason. On failure, the fullscreen TUI shows the named phase, source and version context, concrete error, and history location after recovery. Setup command failures additionally show the failed command and the tail of captured stdout/stderr.

## Health checks

HTTP:

```json
{
  "healthcheck": {
    "type": "http",
    "url": "http://localhost:8080/health",
    "timeoutSeconds": 30
  }
}
```

Command:

```json
{
  "healthcheck": {
    "type": "command",
    "command": "./app doctor",
    "timeoutSeconds": 30
  }
}
```

A failed health check triggers transaction recovery.

## Locks

The update lock contains PID, host, timestamp, and command metadata. Locks whose owning local PID is no longer alive are recognized as stale and may be removed with:

```bash
update-cli --unlock
```

An active or ambiguous lock is not removed.

## Local-only rollback and cleanup

Rollback and cleanup operate on local release/backup inventory only. They do not contact configured URL or repository sources, so recovery remains available during network outages.

## Build and validation

```bash
just check
just build
just build-all
```

Equivalent commands:

```bash
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
go build -trimpath -ldflags "-s -w -X main.version=$(cat VERSION)" -o dist/update-cli .
```

CI runs format, vet, tests, race tests, native builds on Linux/macOS, and cross-builds for:

- macOS amd64
- macOS arm64
- Linux amd64
