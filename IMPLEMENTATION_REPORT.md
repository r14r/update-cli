# Update CLI 3.2.2 implementation report


## 3.2.2 confirmation modal

Fullscreen confirmations no longer reuse the footer. Both update confirmation and setup-after-update confirmation are rendered as centered overlay dialogs with `YES` and `NO` button boxes. Default-No dialogs visually emphasize `NO`; Enter therefore declines. The modal does not mutate the content, info, or footer state. Once an answer is read, the underlying screen is repainted. Plain and `--no-ui` modes keep the `[j/N]` line prompt for script/terminal compatibility.

## 3.2.1 TUI handoff

When an interactive update detects project setup and the user confirms the `[j/N]` question with `j`, the scrollable content region is cleared before setup begins. The surrounding header/info frame is preserved and the footer changes to `RUN  Projekt-Setup läuft`. Declining setup does not clear the update history.


## Scope

3.1.0 redesigns project setup around `setup.yaml` schemaVersion 2 while retaining schemaVersion 1 compatibility. The update transaction, source, backup, rollback, TUI and configuration behavior from 3.0.x remains intact.

## Setup management commands

3.2.0 adds setup-file lifecycle management:

```text
--convert-yaml
--create-yaml
--create-setup-script
-create-setup-script   (compatibility alias)
```

Conversion validates the generated schema-2 manifest before replacing the original and stores a timestamped schema-1 backup. Project generation detects Go, Python, Node, Laravel and Docker Compose markers and may combine multiple detected stacks into one manifest. `--force` is required to overwrite generated files and `--dry-run` prints the proposed output without writing.

## Setup schemaVersion 2

Implemented:

- named workflows
- reusable named tasks
- task dependencies (`requires`)
- dependency de-duplication
- dependency cycle detection
- project metadata
- defaults (`timeout`, `failFast`)
- variables and built-ins
- environment-variable fallback syntax: `{{ env.NAME | fallback }}`
- required and optional commands
- selected task/workflow execution
- setup catalog/listing

CLI:

```text
--setup
--setup-list
--setup-task NAME
--setup-workflow NAME
--setup-manifest FILE [--setup-list|--setup-task NAME|--setup-workflow NAME]
```

`--setup` runs workflow `setup` (or task `setup` when no workflow exists).

## Conditions

Schema 2 supports structured conditions:

- `fileExists`
- `fileNotExists`
- `directoryExists`
- `commandExists`
- `envSet`
- `os`
- `arch`
- `compose`
- `all`
- `any`
- `not`

Schema-1 string conditions remain unchanged.

## Step controls

All schema-2 operations support:

- `id`
- `name`
- `cwd`
- `env`
- `timeout`
- `retries`
- `allowFailure` / `continueOnError`
- `when`

## Operation handlers

Implemented typed operations:

### Generic execution

- `command` (`exec` + `args`)
- `shell`

The shell operation is the universal escape hatch for project-specific behavior that does not warrant a dedicated handler.

### Filesystem / environment preparation

- `mkdir`
- `copy`
- `move`
- `remove`
- `chmod`
- `symlink`
- `touch`
- `write`
- `deploy`

Project-local file operations enforce project containment. `deploy` is the explicit operation that permits an absolute destination.

### Validation

- `assert.fileExists`
- `assert.directoryExists`
- `assert.executable`
- `assert.commandExists`
- `assert.envSet`
- `assert.portAvailable`
- `assert.http`

### Language/toolchain operations

- `go`: mod-download, vet, test, generate, build
- `pythonVenv`
- `pip`
- `npm`
- `pnpm`
- `yarn`
- `composer`
- `artisan`
- `dockerCompose`

### Network/artifact operations

- `httpCheck`
- `download` with optional SHA-256
- `extract` for ZIP archives with traversal/symlink rejection

## Backward compatibility

The existing schemaVersion 1 parser and executor remain active for:

- Update CLI 2.14 style `schemaVersion/id/name/when/run/cwd/allowFailure`
- typed `version: 1` handlers from Update CLI 3.0
- legacy `setup.sh`
- legacy configured setup commands

Plain `--setup` continues to fall back to legacy `setup.sh`; only schema-2-specific selectors require `setup.yaml`.

## Self-hosting / dogfooding

The Update CLI project itself now uses schemaVersion 2 with:

- tasks: `prepare`, `check`, `build`, `verify`, `deploy`, `clean`
- workflows: `setup`, `ci`, `build`, `clean`

The `ci` workflow was executed end-to-end using Update CLI itself and produced a binary reporting `Update CLI 3.1.0`.

## Wrapper and Just integration

`setup.sh` and the installed `setup-template.sh` now accept:

```text
--list
--task NAME
--workflow NAME
```

The Justfile adds:

```text
setup-list
setup-task TASK
setup-workflow WORKFLOW
```

## Validation

Validated locally:

- `gofmt`
- `bash -n setup.sh`
- `bash -n setup-template.sh`
- `go vet ./...`
- `go test ./...`
- `go test -race ./...`
- schema-2 parser tests
- task/dependency/cycle tests
- structured condition tests
- filesystem/environment operation tests
- HTTP/download/checksum/extract tests
- CLI selector tests
- wrapper forwarding tests
- standalone current-directory task selection
- fullscreen PTY regression suite
- self-hosted schema-2 `ci` workflow
- macOS amd64 build
- macOS arm64 build
- Linux amd64 build
