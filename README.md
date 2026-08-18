# Update CLI

![Update CLI — transactional updates and project setup automation](docs/update-cli-readme-header.png)

**Update CLI** is a transactional release updater and project setup/automation runner for versioned applications. It installs validated releases into a stable `current/` directory, keeps immutable release copies and recovery state, and can prepare, check, test, build, deploy, start, stop, migrate, and verify projects through `setup.yaml`.

Current release: **1.0.0**

### 1.0 stability baseline

Version 1.0.0 promotes the hardened 0.8.x line to the stable release line. It keeps the existing CLI, command aliases, config schema, setup schema and transactional workflow compatible while tightening crash recovery and reducing unnecessary I/O. Notable 1.0 hardening includes unique release/transaction staging paths, recoverable incomplete locks, validated-only `restore latest`, canonical backup path checks, crash-tolerant trailing history records, and single-pass ZIP extraction during update/verify.

The Update CLI-specific version policy explicitly treats **1.0.0 as newer than both 0.8.x and the pre-reset 2.x/3.x development line**. Version ordering for every other managed project remains normal semantic versioning.

See [CODE_REVIEW.md](CODE_REVIEW.md) for the complete pre-1.0 review.

## Quickstart

The normal day-to-day workflow is intentionally short: initialize a project once, generate or maintain `setup.yaml`, make a versioned release available, and then run Update CLI **without parameters**.

New projects created with `--init` use `check` as their default no-parameter action. This means a bare `update-cli` invocation checks for a newer release first instead of installing anything immediately.

### 1. Initialize the project

Create or enter the project directory, then initialize Update CLI:

```bash
mkdir nvidia-cli
cd nvidia-cli
update-cli --init nvidia-cli
```

This creates `.updater-cli/config.json` and the standard `release/`, `current/`, and backup structure. A new project starts with:

```json
{
  "no parameter": ["check"]
}
```

Therefore the normal command after initialization is simply:

```bash
update-cli
```

which is equivalent to running the configured `check` action.

![Quickstart — initialize project](doc/images/quickstart/01-init.png)

### 2. Generate project setup automation

Let Update CLI inspect the project and generate the current schemaVersion-2 setup manifest:

```bash
update-cli --create-yaml --from project
update-cli --create-setup-script
```

The detector can combine multiple stacks, for example `go+node+docker`. Review the generated `setup.yaml` before using it in production.

If the project already contains a legacy `setup.sh`, convert that instead:

```bash
update-cli --create-yaml --from setup-script
```

or optionally refine the deterministic conversion with the configured AI provider:

```bash
update-cli --create-yaml --from setup-script --with-ai
```

![Quickstart — generate setup.yaml](doc/images/quickstart/02-create-yaml.png)

### 3. Choose how setup should run after an update

There are two useful no-parameter configurations.

The default created by `--init` is:

```json
{
  "no parameter": ["check"]
}
```

With this configuration the bare command performs this flow:

```text
update-cli
   ↓
check for a newer release
   ↓
Update jetzt installieren?     YES is selected by default
   ↓ Enter
transactional update
   ↓
Projekt-Setup jetzt ausführen? YES is selected by default
   ↓ Enter
setup
```

If you normally want the setup to run **automatically after an accepted update**, change the setting through the CLI:

```bash
update-cli config --set no-parameter="check,setup"
```

This updates `.updater-cli/config.json` to:

```json
{
  "no parameter": ["check", "setup"]
}
```

Then the normal workflow becomes:

```text
update-cli
   ↓
check for a newer release
   ↓
Update jetzt installieren?     YES is selected by default
   ↓ Enter
transactional update
   ↓
setup automatically            no second setup question
```

This is usually the most convenient configuration for projects whose `setup.yaml` is safe and expected to run after every release.

### 4. Make a release available

For the default `download` source, place a versioned ZIP in `$HOME/Downloads` using the naming convention:

```text
nvidia-cli-v0.1.5.zip
```

The archive name must match the configured project name and semantic version pattern:

```text
<PROJECT>-v<MAJOR>.<MINOR>.<PATCH>.zip
```

You can also configure an HTTPS or Git repository source; the daily workflow remains the same.

### 5. Check and install the update

With a newly initialized project, run:

```bash
update-cli
```

You can always request the check explicitly:

```bash
update-cli --check
```

If a newer release is available, the fullscreen UI displays the installed and available versions and opens the confirmation modal. **YES is preselected**, so pressing Enter immediately starts the update. Use `←` / `→` or Tab when you want to change the selection.

![Quickstart — check for update](doc/images/quickstart/03-check.png)

The update is transactional. Update CLI validates the artifact, creates a transaction snapshot, prepares the immutable release, synchronizes `current/`, verifies the installation, preserves configured persistent paths, and automatically restores the previous state if a later phase fails. The project `.gitignore` is always protected during rsync synchronization and restore.

After the TUI closes, Update CLI leaves a compact final status line in the normal shell scrollback. After a successful update it contains both the Update CLI version and the project version that was installed:

```text
Update CLI Version 1.0.0 | nvidia-cli | Aktualisiert auf Version: v1.2.4
```

If a check completes without installing anything, the final line reports the currently installed project version instead:

```text
Update CLI Version 1.0.0 | nvidia-cli | Installierte Version: v1.2.4
```

This status line is also emitted in `--no-ui` mode. JSON output remains machine-readable and does not receive the extra line.

If the selected release is already installed, Update CLI treats that as a successful no-op rather than a failed update. The version-policy step completes normally and the fullscreen content area shows a green notice:

```text
Version 1.2.4 ist bereits installiert
```

The footer remains in the normal close state (`Update beenden | Enter zum Schließen`) and the process exits with code `0`. Use `--force` only when you intentionally want to reinstall the same version.

![Quickstart — transactional update](doc/images/quickstart/04-update.png)

### 6. Run setup explicitly when needed

Setup can also be run independently at any time:

```bash
update-cli --setup
```

To install a release directly and force setup without a setup confirmation:

```bash
update-cli --update --setup
```

To install a release without running setup:

```bash
update-cli --update --no-setup
```

For scripts or CI where fullscreen rendering is undesirable:

```bash
update-cli --update --setup --no-ui
```

`--no-ui` streams child-process stdout/stderr directly to the terminal.

![Quickstart — project setup](doc/images/quickstart/05-setup.png)

### 7. Verify the final state

Inspect the active version and run environment diagnostics:

```bash
update-cli --status
update-cli --doctor
```

`--status` shows the active/current release state; `--doctor` verifies configuration, release metadata, setup availability, required tools, and other project prerequisites.

![Quickstart — verify status](doc/images/quickstart/06-status.png)

### Typical workflows at a glance

| Goal | Configuration / command | Prompts |
|---|---|---|
| Safe default for a new project | `"no parameter": ["check"]` + `update-cli` | update confirmation, then setup confirmation |
| Check first, then always run setup | `"no parameter": ["check", "setup"]` + `update-cli` | update confirmation only |
| Direct update and setup | `update-cli --update --setup` | none |
| Direct update without setup | `update-cli --update --no-setup` | none |
| Setup only | `update-cli --setup` | none |
| CI/plain terminal execution | `update-cli --update --setup --no-ui` | none |

If you use a shell alias such as `u=update-cli`, all examples above work identically with `u`.

## Highlights

- transactional updates with automatic recovery of `current/`
- crash-safe unique transaction/release staging and recoverable stale locks
- local ZIP, HTTPS URL, and Git repository release sources
- single-pass ZIP extraction/verification path with duplicate-path rejection
- semantic version handling and downgrade protection
- protected persistent paths such as `.env`, `data/`, `storage/`, and uploads
- temporary transaction snapshots plus optional persistent backups
- validated-only `restore latest` with canonical backup path protection
- Docker Compose stop/start state preservation
- post-update setup and health checks
- rollback, restore, cleanup, history, status, doctor, and archive verification
- fullscreen terminal UI with fixed Header / Info / Steps / Footer regions
- confirmation modals with selectable `YES` / `NO` buttons
- `--no-ui` mode for direct stdout/stderr streaming
- declarative `setup.yaml` schemaVersion 2 with workflows, tasks, conditions, variables, and typed operations
- automatic `setup.yaml` generation from project files
- deterministic conversion of legacy `setup.sh` into schemaVersion 2
- optional AI refinement of `setup.sh` conversions
- schemaVersion-1 setup compatibility and legacy `setup.sh` fallback

## Installation layout

The default installation locations embedded in the binary are:

```text
Binary             /usr/local/bin/update-cli
Global config      /usr/local/etc/update-cli
Download folder    $HOME/Downloads
```

A managed project normally looks like this:

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
│   ├── setup.yaml
│   └── ... application files ...
├── backup/
└── .release-update.lock/
```

Update CLI can resolve the project root while invoked from `current/` or a deeper subdirectory by walking upward to the nearest `.updater-cli/config.json`. An explicit `--root` always takes precedence.

## Build from source

```bash
git clone <repository>
cd update-cli
just build
```

Or without `just`:

```bash
go vet ./...
go test ./...
go build -trimpath -ldflags "-s -w -X main.version=$(cat VERSION)" -o dist/update-cli .
```

Deploy the locally built binary and global setup template with the project's setup workflow or the Justfile deployment recipes.

## Machine-readable CLI discovery

Update CLI exposes a deterministic command contract for tools such as **command-ui**:

```bash
update-cli --help --json
```

The command writes **JSON only** to stdout and does not initialize the fullscreen TUI, perform release discovery, mutate configuration, or execute setup/update operations. The equivalent command-token form is:

```bash
update-cli help --json
```

The discovery document uses `schemaVersion: 1`, reports the currently running Update CLI version, describes command arguments and relevant options, and exposes dynamic value sources for rollback releases, restore backups, setup tasks, and setup workflows.

If command-ui is installed, validate and open Update CLI with:

```bash
command-ui validate update-cli
command-ui inspect update-cli
command-ui update-cli
```

### Command-token aliases

Update CLI remains fully backward compatible with the established flag-based interface. Command-token forms are aliases that are normalized to the same internal options and execution path:

```bash
update-cli check                       # same as: update-cli --check
update-cli update release.zip          # same as: update-cli --update release.zip
update-cli backup                      # same as: update-cli --backup
update-cli rollback 1.2.3              # same as: update-cli --rollback 1.2.3
update-cli restore latest              # same as: update-cli --restore latest
update-cli status --json               # same as: update-cli --status --json
update-cli list --json                 # same as: update-cli --list --json
update-cli verify release.zip          # same as: update-cli --verify release.zip
update-cli doctor                      # same as: update-cli --doctor
update-cli cleanup                     # same as: update-cli --cleanup
update-cli history                     # same as: update-cli --history
update-cli init my-project             # same as: update-cli --init my-project
update-cli upgrade                     # same as: update-cli --upgrade
update-cli unlock                      # same as: update-cli --unlock
```

Setup automation also has a command hierarchy:

```bash
update-cli setup
update-cli setup list
update-cli setup list --json
update-cli setup task build
update-cli setup workflow ci
update-cli setup manifest ./setup.yaml
```

The existing forms `--setup`, `--setup-list`, `--setup-task`, `--setup-workflow`, and `--setup-manifest` remain supported. The YAML lifecycle commands also accept token aliases:

```bash
update-cli convert-yaml
update-cli create-yaml --from project
update-cli create-setup-script
```

Configuration and template operations can be written command-first as well:

```bash
update-cli config
update-cli config list
update-cli config edit
update-cli config use-template go
update-cli config --set no-parameter="check,setup"

update-cli templates list
update-cli templates edit
update-cli templates use go
```

Rollback and restore selectors in the discovery document use the structured inventory from `update-cli list --json`. Setup task/workflow selectors use `update-cli setup list --json`.

## CLI reference

### Release and recovery

```bash
update-cli --check [--no-ask] [--wait|--no-wait] [--no-ui]
update-cli --update [ARCHIVE.zip] [--backup] [--setup|--no-setup] [--force]
update-cli --update --plan [--json]
update-cli --backup
update-cli --rollback [VERSION] [--setup]
update-cli --restore latest
update-cli --verify ARCHIVE.zip
update-cli --clean [--keep N] [--plan]      # release folder only
update-cli --cleanup [--keep N] [--plan]    # release + backup retention
update-cli --unlock
```

### Typo suggestions

Unknown command-line options are checked against the supported flag set. Close matches produce a correction hint instead of only a generic parser error:

```bash
update-cli --vesion
# ERROR  unbekannter Parameter "--vesion"; meinten Sie "--version"?
```

Update CLI does not silently execute the suggested option; correct the command and run it again.

### Information and diagnostics

```bash
update-cli --status [--json]
update-cli --list [--json]
update-cli --doctor
update-cli --history [--limit N]
update-cli --howto
update-cli --version
```

### Configuration and templates

The preferred configuration command is `update-cli config`. The historical `--config` spelling remains supported.

```bash
update-cli config
update-cli config --list
update-cli config --edit
update-cli config --use-template NAME
update-cli config --set KEY=VALUE
update-cli --templates --list [--details]
update-cli --init PROJECTNAME
update-cli --upgrade
```

`config --set` can change any supported value in `.updater-cli/config.json`. Nested JSON fields use dotted paths; key matching accepts JSON camelCase as well as kebab-case, snake_case, and spaces. Lists can be supplied as comma-separated values or as a JSON array.

Typical examples:

```bash
# Change the no-parameter workflow
update-cli config --set no-parameter="check,setup"

# Retention values are parsed as integers
update-cli config --set backup.keep=7
update-cli config --set retention.releases=10

# Boolean values retain their JSON type
update-cli config --set security.allow-http=true

# Lists are accepted as comma-separated values
update-cli config --set sync.preserve=".git/,.gitignore,.env,data/,storage/"

# Multiple changes are validated together and written as one operation
update-cli config \
  --set source.type=url \
  --set source.url=https://example.org/releases/my-app-v1.4.0.zip
```

Examples of supported dotted paths include:

```text
projectName
source.type
source.folder
source.url
source.repository
source.ref
source.commit
source.version
source.sha256
releaseDir
currentDir
no-parameter              -> JSON key "no parameter"
setup.commands
backup.directory
backup.keep
retention.releases
sync.preserve
security.allowHttp
security.maxArchiveBytes
security.maxUncompressedBytes
security.maxFileBytes
security.maxEntries
security.maxCompressionRatio
healthcheck.type
healthcheck.url
healthcheck.command
healthcheck.timeoutSeconds
docker.lifecycle
```

All `--set` assignments are applied in memory first. Update CLI validates the complete resulting configuration and writes it atomically only if the combined configuration is valid. Unknown keys, invalid types, or invalid configurations are rejected without changing `config.json`.

## Docker lifecycle

Docker Compose handling during update transactions is controlled by the project configuration in `.updater-cli/config.json`. The setting is **not** part of `setup.yaml`. Existing projects without a `docker` block automatically behave as `auto`.

```json
{
  "docker": {
    "lifecycle": "auto"
  }
}
```

Supported values:

- `auto` — default and backward-compatible mode. If no Compose file exists, Update CLI does nothing. If a Compose file exists and Docker status can be determined, previously running services are stopped before replacing `current` and restarted afterward. If Docker, Compose, the daemon, or `compose ps` status detection is unavailable, Update CLI emits a warning and continues the filesystem transaction without Docker lifecycle management.
- `disabled` — never interact with Docker during update transactions or transaction recovery. Compose files may remain in `current/`; they are simply ignored by the updater lifecycle.
- `required` — Docker lifecycle handling is mandatory whenever a Compose file exists. Docker/Compose/status failures abort the transaction. If no Compose file exists, this mode is allowed and behaves as a no-op for Docker.

Set the value through the CLI:

```bash
update-cli config --set docker.lifecycle=auto
update-cli config --set docker.lifecycle=disabled
update-cli config --set docker.lifecycle=required
```

For a project such as **Life OS**, where `docker-compose.yml` is only an optional deployment mechanism and Docker Desktop may be stopped, use:

```bash
update-cli config --set docker.lifecycle=disabled
```

which persists conceptually as:

```json
{
  "docker": {
    "lifecycle": "disabled"
  }
}
```

Then:

```bash
update-cli --update --no-ui
```

updates the project without invoking Docker and without requiring the Compose file to be renamed or moved.

In `auto` mode, a degraded Docker status check is reported as a warning rather than an update failure. In `required` mode the same condition remains fatal. `update-cli --status` exposes the configured Docker lifecycle, and `update-cli --doctor` reports disabled as skipped/OK, auto failures as warnings, and required failures as errors.

### Setup execution

```bash
update-cli --setup [--details] [--wait|--no-wait] [--no-ui]
update-cli --setup-list
update-cli --setup-task NAME [--details] [--no-ui]
update-cli --setup-workflow NAME [--details] [--no-ui]
update-cli --setup-manifest ./setup.yaml
update-cli --setup-manifest ./setup.yaml --setup-list
update-cli --setup-manifest ./setup.yaml --setup-task NAME
update-cli --setup-manifest ./setup.yaml --setup-workflow NAME
```

### Setup-file management

```bash
update-cli --convert-yaml [--dry-run]
update-cli --create-yaml [--from project|setup-script] [--with-ai] [--force] [--dry-run]
update-cli --create-setup-script [--force] [--dry-run]
```

The compatibility spelling `-create-setup-script` is accepted, but `--create-setup-script` is the documented form.

## Fullscreen TUI

The header shows the project name together with the currently installed project version when available.


Interactive `--check`, real `--update`, and setup execution use the fullscreen UI when stdout/stdin are terminals and colors are enabled.

The screen has four independent regions:

```text
┌──────────────────────────────────────────────────────────────┐
│ Update CLI Version 1.0.0   |   my-project v1.2.4   |   Setup       │
└──────────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────────┐
│ Project / update / setup information                         │
└──────────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────────┐
│ Scrollable steps and command stdout/stderr                   │
│ [01/08] Prepare project                                  ✓   │
│          │ PREPARE   workspace ... OK                         │
│ [02/08] Run tests                                        …   │
│          │ TEST      package ./... ... OK                      │
└──────────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────────┐
│ High-level status / final state only                         │
└──────────────────────────────────────────────────────────────┘
```

Only the step/output area scrolls. Process output never owns the footer. During setup, fullscreen stdout/stderr is rendered in a fixed gutter directly below its step. Setup metadata shown in the scrollable content region uses that same gutter, so `Projekt:` and `Schema:` align exactly with the command output below:

```text
│ Projekt-Setup
│         │ Projekt: Update CLI
│         │ Schema: 2 | Tasks: 5 | Schritte: 14
│ [01/14] Go-Module laden                                               ✓
│         │ ! go: no module dependencies to download
```

Leading padding emitted by child tools is normalized, so all output begins at the same column; wrapped continuation lines keep the same indentation. stderr uses `│ !` in the same gutter.

### Confirmation modal

Update and post-update setup confirmations are displayed as centered modal dialogs:

```text
┌──────────────────────────────────────────────────────────┐
│                      Bestätigung                         │
│                                                          │
│                Update jetzt installieren?                │
│                                                          │
│       ┌───────────────┐       ┌───────────────┐          │
│       │      YES      │       │      NO       │          │
│       └───────────────┘       └───────────────┘          │
│   ←/→ auswählen · Enter bestätigen · j/y YES · n NO     │
└──────────────────────────────────────────────────────────┘
```

Controls:

- `←` selects **YES**
- `→` selects **NO**
- `Enter` confirms the highlighted button
- `j`, `ja`, `y`, `yes` immediately choose **YES**
- `n`, `nein`, `no` immediately choose **NO**
- `Tab` toggles the highlighted button

Both update and setup confirmations default to **YES**. In fullscreen mode, YES is preselected; in plain/`--no-ui` mode the confirmation suffix is `[J/n]`.

### TUI modes

```bash
UPDATE_CLI_TUI=auto update-cli --update
UPDATE_CLI_TUI=fullscreen update-cli --check
UPDATE_CLI_TUI=plain update-cli --update
update-cli --update --no-ui
update-cli --setup --no-ui
update-cli --setup --noui     # alias for --no-ui
update-cli --update --no-wait
```

`--no-ui` completely disables the alternate-screen UI and streams command stdout/stderr directly. It does not disable colors; combine it with `--no-color` if plain uncolored output is required.

Setup output in `--no-ui` mode is step-centric. Task headings and separate task rules are intentionally omitted; every visible block starts directly with its numbered step. Step stdout/stderr is indented behind a vertical guide, and a closing status line marks the end of each step:

```text
[01/03] Install Python dependencies ────────────────────────────────────
│  ❯ .venv/bin/python -m pip install -r requirements.txt
│  INSTALL   Python dependencies ... OK
└─ ✓ Install Python dependencies

[02/03] Install project CLI ────────────────────────────────────────────
│  ❯ python static/scripts/install_cli.py
│  INSTALL   dp-cli + digital-product-cli ... OK
└─ ✓ Install project CLI
```

This keeps long command output visibly attached to the step that produced it. stderr lines use the same guide and receive an additional `!` marker. Skipped steps use a matching `└─ – ...` closing line.

`--noui` is accepted as an alternative spelling of `--no-ui`. The historical compatibility spelling `---no-ui` also remains accepted. `--no-ui` remains the documented canonical option.

`NO_COLOR=1`, `--no-color`, JSON output, non-interactive output, and `UPDATE_CLI_TUI=plain` avoid fullscreen rendering.

External command failures now include the executed command, working directory, exit code, and captured stdout/stderr where available. This includes Docker Compose status/start/stop failures during update transactions.

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

Release archives use:

```text
<PROJECT>-v<MAJOR>.<MINOR>.<PATCH>.zip
```

Example:

```text
nvidia-cli-v0.1.5.zip
```

### HTTPS URL

```json
{
  "source": {
    "type": "url",
    "url": "https://downloads.example.com/demo-v1.2.3.zip",
    "sha256": "optional expected SHA-256"
  }
}
```

Plain HTTP is rejected unless `security.allowHttp` is enabled. Metadata checks use `HEAD` with a range-request fallback where necessary, so `--check` does not normally download the full artifact.

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

Repository content is validated under the same file policy as ZIP releases; unsupported special files and symbolic links are rejected.

## No-parameter behavior

New projects created with `--init` default to:

```json
{
  "no parameter": ["check"]
}
```

A bare invocation therefore checks for a newer release. If an update is accepted and project setup is available, setup is offered separately; both confirmation modals select **YES** by default.

To automatically run setup after an accepted update without a second setup question, configure:

```json
{
  "no parameter": ["check", "setup"]
}
```

This preserves the safe **check first** workflow: Update CLI still asks before installing the release, but once the update is accepted the configured project setup becomes part of the update transaction automatically.

The configured list selects the primary action only for a genuinely argument-free invocation. Explicit commands such as `--upgrade`, `--doctor`, or `--version` are not combined with that default. The `setup` modifier of `["check", "setup"]` is also honored when `--check` is invoked explicitly, so an accepted checked update behaves consistently. For an explicit direct update with automatic setup, use `update-cli --update --setup`.

## Transactional update model

A normal update is represented as 13 explicit phases:

1. resolve the release source
2. validate target version and update policy
3. validate archive/repository content
4. prepare the versioned release
5. create the transaction snapshot of `current/`
6. optionally create a persistent user backup
7. synchronize the release to `current/`
8. verify the installed `current/` state
9. run or skip project setup
10. restore previously running Docker services
11. run the configured health check
12. activate the versioned release
13. write status/history and commit the transaction

A failure after the transaction begins restores the previous `current/` snapshot. If a Compose stack was running before the update, Update CLI attempts to restore that running state after recovery.

If setup is accepted interactively, the step/output area is cleared before setup starts so installation phases and setup output are not mixed on one screen.

## Persistent paths

The default protected paths are:

```text
.git/
.gitignore
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

Override them in project configuration:

```json
{
  "sync": {
    "preserve": [".gitignore", ".env", "data/", "storage/"]
  }
}
```

Protected paths are neither overwritten by release content nor removed by release synchronization.

## Backups and snapshots

### Transaction snapshots

Created automatically for update, rollback, and restore. They are exact temporary recovery copies and are removed after a successful commit.

### Persistent user backups

Create one explicitly:

```bash
update-cli --backup
update-cli --update --backup
```

Persistent backups exclude regenerable dependencies and secret-bearing `.env` files and are retained according to the configured backup policy.

## Health checks

HTTP health check:

```json
{
  "healthcheck": {
    "type": "http",
    "url": "http://localhost:8080/health",
    "timeoutSeconds": 30
  }
}
```

Command health check:

```json
{
  "healthcheck": {
    "type": "command",
    "command": "./app doctor",
    "timeoutSeconds": 30
  }
}
```

A failed post-update health check causes transaction recovery.

## Archive and path security

Project configuration schema v6 supports limits such as:

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

- absolute paths and traversal (`../`)
- symbolic links
- unsupported special files
- excessive file/archive sizes
- excessive entry counts
- excessive expanded size
- suspicious compression ratios
- CRC/read failures

Configured project/release/current/backup paths are canonicalized to prevent symlink-based containment escapes.

## Locks

The update lock stores PID, host, timestamp, and command metadata. A lock whose local owner PID is no longer alive is treated as stale.

Explicitly remove a recoverable stale lock with:

```bash
update-cli --unlock
```

Active or ambiguous locks are not silently removed.

## `setup.yaml` schemaVersion 2

SchemaVersion 2 turns `setup.yaml` into a declarative project automation manifest. Reusable **tasks** contain steps; **workflows** compose tasks into entry points such as `setup`, `ci`, `build`, or `clean`.

```yaml
schemaVersion: 2

project:
  name: Example CLI
  type: go
  description: Build, test and deploy Example CLI

variables:
  binary: example
  distDir: dist

defaults:
  failFast: true
  timeout: 10m

requirements:
  commands:
    - go

workflows:
  setup:
    tasks:
      - deploy

  ci:
    tasks:
      - verify

tasks:
  prepare:
    steps:
      - id: modules
        name: Download Go modules
        go:
          action: mod-download
        when:
          fileExists: go.mod

  check:
    requires: [prepare]
    steps:
      - id: vet
        name: Static analysis
        go:
          action: vet

  test:
    requires: [check]
    steps:
      - id: test
        name: Tests
        go:
          action: test

  build:
    requires: [test]
    steps:
      - id: build
        name: Build binary
        shell: |
          mkdir -p "{{ distDir }}"
          go build -o "{{ distDir }}/{{ binary }}" .

  verify:
    requires: [build]
    steps:
      - id: verify-binary
        name: Verify binary
        assert:
          executable: "{{ distDir }}/{{ binary }}"

  deploy:
    requires: [verify]
    steps:
      - id: deploy
        name: Deploy binary
        deploy:
          source: "{{ distDir }}/{{ binary }}"
          target: "/usr/local/bin/{{ binary }}"
          mode: "0755"
```

### Workflow and task execution

```bash
update-cli --setup
update-cli --setup-list
update-cli --setup-workflow ci
update-cli --setup-task test
update-cli --setup-task build
update-cli --setup-task clean
```

Task dependencies are topologically resolved, de-duplicated, and checked for cycles.

### Conditions

Simple condition:

```yaml
when:
  fileExists: go.mod
```

Compound condition:

```yaml
when:
  all:
    - fileExists: compose.yaml
    - commandExists: docker
    - not:
        envSet: SKIP_DOCKER
```

Supported condition families include:

```text
fileExists
fileNotExists
directoryExists
commandExists
envSet
os
arch
compose
all
any
not
```

### Variables

```yaml
variables:
  binary: app
  deployPath: "{{ env.DEPLOY_PATH | /usr/local/bin }}"
```

Built-ins include project metadata, OS/architecture values, and environment-variable references.

### Per-step controls

SchemaVersion-2 steps can define:

```yaml
cwd: backend

env:
  APP_ENV: testing

timeout: 5m
retries: 2
allowFailure: false
```

### Typed operations

The current engine supports:

| Category | Operations |
|---|---|
| Generic execution | `command`, `shell` |
| Filesystem | `mkdir`, `copy`, `move`, `remove`, `chmod`, `symlink`, `touch`, `write`, `deploy` |
| Validation | `assert` |
| Python | `pythonVenv`, `pip` |
| JavaScript | `npm`, `pnpm`, `yarn` |
| PHP/Laravel | `composer`, `artisan` |
| Go | `go` |
| Containers | `dockerCompose` |
| Network | `httpCheck`, `download` |
| Archives | `extract` |

`command` uses structured executable/argument handling. `shell: |` remains the escape hatch for project-specific logic that cannot safely be represented by a typed operation.

The full schema is documented in [`doc/setup-schema.md`](doc/setup-schema.md).

## SchemaVersion-1 and legacy setup compatibility

Existing schemaVersion-1 manifests remain executable:

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
```

A legacy `setup.sh` can still be run when no manifest is available. Nested legacy scripts are executed without their own hidden wait/fullscreen cycle so the parent Update CLI owns the user interface.

## Setup-file lifecycle

### Convert an existing YAML

```bash
update-cli --convert-yaml
```

SchemaVersion 1 is converted to schemaVersion 2. The generated result is parsed before replacing the original and a timestamped backup is retained.

Preview only:

```bash
update-cli --convert-yaml --dry-run
```

### Generate YAML from project files

```bash
update-cli --create-yaml
update-cli --create-yaml --from project
```

Project detection is additive and currently recognizes:

- Go: `go.mod`
- Python: `pyproject.toml`, `requirements.txt`, `setup.py`, `Pipfile`
- Node: `package.json` and npm/pnpm/yarn lockfiles
- Laravel: `artisan`, `composer.json`, `laravel/framework`
- Docker Compose: `compose.yml`, `compose.yaml`, `docker-compose.yml`, `docker-compose.yaml`

A mixed project may therefore be described as, for example, `go+node+docker` and receive tasks for all detected stacks.

### Generate YAML from `setup.sh`

```bash
update-cli --create-yaml --from setup-script
```

The deterministic converter analyzes the existing shell setup and recognizes template-style setup arrays and common Go, Python, Node, Composer, Docker, deployment, and command patterns. When a safe typed mapping is not possible, the original behavior is preserved as an ordered `shell: |` step rather than guessed away.

### AI-assisted setup-script conversion

```bash
update-cli --create-yaml --from setup-script --with-ai
```

The conversion pipeline is:

```text
setup.sh
   ↓
deterministic converter
   ↓
schemaVersion-2 draft
   ↓
AI refinement
   ↓
schemaVersion-2 parser validation
   ↓
setup.yaml
```

AI receives both the **original setup script** and the **deterministic draft**. The deterministic conversion is always performed first. An AI result is accepted only if Update CLI can parse it as a valid schemaVersion-2 manifest.

Supported provider identifiers:

```text
ollama
openai-compatible
nvidia
```

Default AI configuration file:

```text
/usr/local/etc/update-cli/ai.json
```

Example:

```json
{
  "provider": "ollama",
  "baseUrl": "http://localhost:11434",
  "model": "qwen3:8b",
  "timeout": "2m"
}
```

Environment overrides:

```text
UPDATE_CLI_AI_PROVIDER
UPDATE_CLI_AI_BASE_URL
UPDATE_CLI_AI_MODEL
UPDATE_CLI_AI_API_KEY
UPDATE_CLI_AI_API_KEY_ENV
UPDATE_CLI_AI_CONFIG
UPDATE_CLI_AI_PROMPT
OPENAI_API_KEY
```

The conversion prompt is shipped in the repository at:

```text
prompts/setup-script-to-yaml.txt
```

and installed by the Update CLI setup workflow to:

```text
/usr/local/etc/update-cli/prompts/setup-script-to-yaml.txt
```

### Generate the setup wrapper

```bash
update-cli --create-setup-script
```

This creates a generic executable `setup.sh` that resolves a compatible local/platform Update CLI binary and delegates manifest execution to the CLI setup engine.

Generation commands protect existing files unless `--force` is specified. Use `--dry-run` to inspect generated output without writing files.

## Global setup template

The standard installation includes:

```text
/usr/local/etc/update-cli/setup-template.sh
```

It can be copied into a project or executed from a project/current directory. For schemaVersion 2 it deliberately avoids handing the manifest to an incompatible old CLI; it prefers matching local/platform binaries and can bootstrap from the Go source tree when necessary.

Wrapper examples:

```bash
./setup.sh
./setup.sh --list
./setup.sh --task build
./setup.sh --workflow ci
./setup.sh --details
./setup.sh --no-ui
./setup.sh --no-wait
```

## Rollback and cleanup are local-only

Rollback and cleanup use local release/backup inventory only. They do not need the configured URL or repository source, so recovery remains available during source-server or network outages.

## Development and validation

Recommended local gate:

```bash
just check
```

Build native binary:

```bash
just build
```

Build supported release targets:

```bash
just build-all
```

The project test gate includes:

```bash
gofmt
go vet ./...
go test ./...
go test -race ./...
```

CI also exercises fullscreen PTY flows and builds:

- macOS amd64
- macOS arm64
- Linux amd64

## Release packaging

Release ZIP names follow:

```text
update-cli-v<MAJOR>.<MINOR>.<PATCH>.zip
```

Example:

```text
update-cli-v3.3.1.zip
```
