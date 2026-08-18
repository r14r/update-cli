# Update CLI

![Update CLI — transactional updates and project setup automation](docs/update-cli-readme-header.png)

**Update CLI** is a transactional release updater and project setup/automation runner for versioned applications. It installs validated releases into a stable `current/` directory, keeps immutable release copies and recovery state, and can prepare, check, test, build, deploy, start, stop, migrate, and verify projects through `setup.yaml`.

Current release: **1.0.2**

### 1.0.2 documentation release

Version 1.0.2 refreshes the README around the actual stable CLI contract. The Quickstart now follows the real onboarding/update flow, explicitly documents that setup-file generation targets the configured `current/` directory, and uses terminal captures produced from an isolated demo project. A new complete Documentation section covers command-token and flag forms, subcommands, supported option variations, compatibility aliases, source overrides, JSON discovery, configuration/templates, setup selectors, retention/recovery commands, and accompanying command screenshots. Runtime behavior and configuration/setup schemas are unchanged.

### 1.0.1 patch release

Version 1.0.1 keeps the 1.0.0 behavior unchanged and adds regression coverage for normal stable-line patch updates (`1.0.0 -> 1.0.1`) and newest-release selection. This release is intended to verify that the corrected Update CLI release-epoch policy continues with ordinary SemVer ordering inside the stable 1.x line.

### 1.0 stability baseline

Version 1.0.0 promotes the hardened 0.8.x line to the stable release line. It keeps the existing CLI, command aliases, config schema, setup schema and transactional workflow compatible while tightening crash recovery and reducing unnecessary I/O. Notable 1.0 hardening includes unique release/transaction staging paths, recoverable incomplete locks, validated-only `restore latest`, canonical backup path checks, crash-tolerant trailing history records, and single-pass ZIP extraction during update/verify.

The Update CLI-specific version policy explicitly treats **1.0.0 as newer than both 0.8.x and the pre-reset 2.x/3.x development line**. Version ordering for every other managed project remains normal semantic versioning.

See [CODE_REVIEW.md](CODE_REVIEW.md) for the complete pre-1.0 review.

## Quickstart

The normal workflow has two parts: **one-time project onboarding** and the **daily update cycle**. The preferred command style uses command tokens (`update-cli check`, `update-cli update`, ...); the established flag forms (`--check`, `--update`, ...) remain fully supported.

### 1. Verify the installed CLI

```bash
update-cli --version
update-cli --help
```

`--version` confirms which Update CLI binary is on `PATH`; `--help` shows the compact command overview. For machine-readable discovery use `update-cli --help --json`.

### 2. Initialize a managed project

Create the updater metadata in the project root:

```bash
mkdir demo-app
cd demo-app
update-cli init demo-app
```

Equivalent legacy form:

```bash
update-cli --init demo-app
```

Initialization creates `.updater-cli/config.json` and configures the standard `release/`, `current/`, and `backup/` locations. New projects start with `check` as their no-parameter action:

```json
{
  "no parameter": ["check"]
}
```

![Quickstart — initialize project](doc/images/quickstart/01-init.png)

### 3. Make the first release available

For the default `download` source, place a versioned ZIP in the configured download directory, normally `$HOME/Downloads`:

```text
demo-app-v0.1.0.zip
```

The required archive naming convention is:

```text
<PROJECT>-v<MAJOR>.<MINOR>.<PATCH>.zip
```

Check the available release without installing it:

```bash
update-cli check --no-ask
```

The same operation using the original flag form is:

```bash
update-cli --check --no-ask
```

![Quickstart — check for update](doc/images/quickstart/03-check.png)

### 4. Preview, then install the release

Preview the complete transaction first:

```bash
update-cli update --plan
```

Install the selected release:

```bash
update-cli update
```

To install a specific local archive:

```bash
update-cli update ~/Downloads/demo-app-v0.1.0.zip
```

For a plain terminal or CI job, disable the fullscreen TUI:

```bash
update-cli update --no-ui
```

Update CLI validates the archive and version policy, creates a transaction snapshot, prepares the immutable release, synchronizes `current/`, preserves configured persistent paths, verifies the installation, optionally runs setup/health checks, and restores the previous state automatically if a later transaction phase fails.

![Quickstart — transactional update](doc/images/quickstart/04-update.png)

### 5. Create or maintain `setup.yaml`

`--create-yaml` operates on the configured **`current/` project directory**. Use it after `current/` contains the application to generate a schemaVersion-2 setup manifest from the detected project:

```bash
update-cli create-yaml --from project --dry-run
update-cli create-yaml --from project
```

If the project already has a `setup.sh`, convert its detected operations instead:

```bash
update-cli create-yaml --from setup-script --dry-run
update-cli create-yaml --from setup-script
```

AI-assisted refinement is available only for the `setup-script` source:

```bash
update-cli create-yaml --from setup-script --with-ai
```

Generate the generic setup wrapper when required:

```bash
update-cli create-setup-script
```

Always review generated automation before using it for production deployment.

![Quickstart — generate setup.yaml](doc/images/quickstart/02-create-yaml.png)

### 6. Run project setup

Run the default `setup` workflow independently:

```bash
update-cli setup
```

Run setup automatically after a direct update:

```bash
update-cli update --setup
```

Explicitly suppress post-update setup:

```bash
update-cli update --no-setup
```

For streaming child-process output without the fullscreen interface:

```bash
update-cli setup --no-ui
update-cli update --setup --no-ui
```

![Quickstart — project setup](doc/images/quickstart/05-setup.png)

### 7. Configure the normal no-parameter workflow

The safe default is:

```json
{
  "no parameter": ["check"]
}
```

A bare invocation therefore checks first:

```bash
update-cli
```

To run setup automatically after an accepted update, configure:

```bash
update-cli config --set no-parameter="check,setup"
```

The resulting daily workflow is:

```text
update-cli
   ↓
check for a newer release
   ↓
confirm update when one is available
   ↓
transactional update
   ↓
setup automatically
```

### 8. Verify the final state

```bash
update-cli status
update-cli doctor
update-cli list
```

`status` summarizes the active installation and available release, `doctor` validates project prerequisites and updater state, and `list` shows validated releases and backups.

![Quickstart — status and doctor](doc/images/quickstart/06-status.png)

### Quickstart workflows at a glance

| Goal | Command |
|---|---|
| Check only | `update-cli check --no-ask` |
| Preview update | `update-cli update --plan` |
| Install newest release | `update-cli update` |
| Install a specific ZIP | `update-cli update release.zip` |
| Update and force setup | `update-cli update --setup` |
| Update without setup | `update-cli update --no-setup` |
| Setup only | `update-cli setup` |
| Plain/CI output | `update-cli update --setup --no-ui` |
| Inspect state | `update-cli status` |
| Diagnose environment | `update-cli doctor` |
| Default interactive workflow | `update-cli` |

If you use a shell alias such as `u=update-cli`, every example works identically with `u`.

## Documentation

This section is the complete CLI command reference for the current release. It documents the preferred command-token syntax, established flag syntax, subcommands, short aliases, compatibility aliases, positional arguments, and command-specific modifiers.

### Command conventions

Both styles execute the same internal command path:

```bash
update-cli check
update-cli --check

update-cli update release.zip
update-cli --update release.zip
```

Common conventions:

| Option | Meaning | Scope |
|---|---|---|
| `--root DIR`, `-r DIR` | Use another updater project root | Most project commands |
| `--json` | Request structured JSON where supported | Discovery/status/list/plan/diagnostic commands |
| `--no-color` | Disable ANSI colors | Most output commands |
| `--no-ui`, `--noui` | Disable fullscreen TUI and stream process output | Check/update/rollback/setup |
| `---no-ui` | Historical compatibility spelling for `--no-ui` | Same as `--no-ui` |
| `--wait` / `--no-wait` | Control waiting before leaving interactive output | Check/update/rollback/setup |
| `--force`, `-f` | Force replacement/reinstall where explicitly supported | Update/init/setup-file generation |
| `--dry-run`, `-n` | Preview without writing/applying | Update/setup-file generation |
| `--downloads DIR`, `-d DIR` | Override download/source directory | Release discovery commands |
| `--from TYPE` | Override source type: `download`, `url`, `repository` | Release discovery/init; create-yaml uses `project`/`setup-script` |
| `--folder DIR` | Override release source folder | Release discovery/init |
| `--url URL` | Override release URL | Release discovery/init |
| `--repository REPO` | Override release repository | Release discovery/init |

![Documentation — help and version](doc/images/documentation/01-help-version.png)

### Help, discovery and version

| Command | Supported forms and variations |
|---|---|
| Help | `update-cli --help`, `update-cli -h`, `update-cli help` |
| Machine-readable help | `update-cli --help --json`, `update-cli help --json` |
| Extended operating notes | `update-cli --howto` |
| Version | `update-cli --version`, `update-cli -V` |

`--help --json` is deterministic, side-effect free, TUI-free, and intended for tools such as **command-ui**. It exposes `schemaVersion: 1`, command arguments, options, and dynamic selectors.

### Check and update

| Command | Purpose | Supported variations |
|---|---|---|
| `check` / `--check` | Find the newest applicable release | `--no-ask`, `--json`, `--wait`, `--no-wait`, `--no-ui`/`--noui`, `--no-color`, source overrides, `--root` |
| `update [ARCHIVE.zip]` / `--update [ARCHIVE.zip]` | Install a release transactionally | Positional ZIP or `--archive/-a`; `--dry-run/-n`; `--plan [--json]`; `--allow-downgrade`; `--backup`; `--setup` or `--no-setup`; `--force/-f`; wait/UI/color/source/root modifiers |

Typical variations:

```bash
update-cli check
update-cli check --no-ask
update-cli check --json
update-cli check --no-ui

update-cli update
update-cli update release.zip
update-cli update --archive release.zip
update-cli update --plan
update-cli update --plan --json
update-cli update --dry-run
update-cli update --backup
update-cli update --setup
update-cli update --no-setup
update-cli update --force
update-cli update --allow-downgrade
update-cli update --setup --no-ui
```

`--allow-downgrade` disables the normal version-order safety check for an intentional downgrade. `--force` is required to reinstall the same version where the updater would otherwise treat it as a no-op.

![Documentation — check and update plan](doc/images/documentation/02-check-update.png)

![Documentation — update execution](doc/images/documentation/03-update-execution.png)

### Backup, rollback and restore

| Command | Supported forms and variations |
|---|---|
| Backup | `update-cli backup`, `update-cli --backup`; optional `--json`, `--root`, `--no-color` |
| Rollback | `update-cli rollback [VERSION]`, `update-cli --rollback [VERSION]`; optional `--setup`, `--json`, `--wait`, `--no-wait`, `--no-ui`, `--root`, `--no-color` |
| Restore | `update-cli restore BACKUP`, `update-cli --restore BACKUP`; `BACKUP` can be `latest` or a validated backup name; optional `--json`, `--root`, `--no-color` |

Examples:

```bash
update-cli backup
update-cli backup --json
update-cli rollback
update-cli rollback 1.4.2
update-cli rollback 1.4.2 --setup --no-ui
update-cli restore latest
update-cli restore 20260818-184500-v1.4.2
```

![Documentation — backup, rollback and restore](doc/images/documentation/04-backup-rollback-restore.png)

### Status, inventory, verification and diagnostics

| Command | Purpose | Supported variations |
|---|---|---|
| `status` / `--status` | Active/available release state | `--json`, `--no-color`, source overrides, `--root` |
| `list` / `--list` | Validated releases and backups | `--json`, `--no-color`, source overrides, `--root` |
| `verify ARCHIVE.zip` / `--verify ARCHIVE.zip` | Validate a release archive without installing | Positional ZIP or `--archive/-a`; `--json`, `--no-color`, source overrides, `--root` |
| `doctor` / `--doctor` | Diagnose updater/project prerequisites | `--json`, `--no-color`, `--root` |

```bash
update-cli status
update-cli status --json
update-cli list
update-cli list --json
update-cli verify release.zip
update-cli verify --archive release.zip --json
update-cli doctor
update-cli doctor --json
```

![Documentation — status, list and doctor](doc/images/documentation/05-status-list-doctor.png)

![Documentation — verify and history](doc/images/documentation/06-verify-history.png)

### History and retention

| Command | Purpose | Supported variations |
|---|---|---|
| `history` / `--history` | Show transaction history | `--limit N` (minimum 1, default 20), `--json`, `--root`, `--no-color` |
| `clean` / `--clean` | Remove obsolete **release-directory entries only** | `--keep N`, `--plan`, `--json`, `--root`, `--no-color` |
| `cleanup` / `--cleanup` | Apply release **and backup** retention | `--keep N`, `--plan`, `--json`, `--root`, `--no-color` |

```bash
update-cli history
update-cli history --limit 10
update-cli history --limit 10 --json
update-cli clean --plan
update-cli clean --keep 3
update-cli cleanup --plan
update-cli cleanup --keep 3
```

Use `--plan` before destructive cleanup when you want to inspect the retention decision without deleting anything.

![Documentation — clean and cleanup](doc/images/documentation/07-clean-cleanup.png)

### Project initialization, config migration and locks

| Command | Supported forms and variations |
|---|---|
| Init | `update-cli init PROJECTNAME`, `update-cli --init PROJECTNAME`; source options; `--use-template NAME`; `--force/-f`; `--root`; `--no-color` |
| Upgrade config | `update-cli upgrade`, `update-cli --upgrade`; optional `--json`, `--root`, `--no-color` |
| Unlock | `update-cli unlock`, `update-cli --unlock`; optional `--root` |

Initialization source examples:

```bash
update-cli init demo-app
update-cli init demo-app --from download --folder ~/Downloads
update-cli init demo-app --from url --url https://example.org/demo-app-v1.0.0.zip
update-cli init demo-app --from repository --repository github.com/acme/demo-app
update-cli init demo-app --use-template Go
update-cli init demo-app --force
```

`unlock` removes a stale update lock; it is not a replacement for terminating an active updater process.

![Documentation — init, upgrade and unlock](doc/images/documentation/08-init-upgrade-unlock.png)

### Setup execution

The default setup command executes workflow `setup` from the configured `current/setup.yaml` or `current/setup.yml`:

```bash
update-cli setup
update-cli --setup
```

Supported modifiers:

```bash
update-cli setup --details
update-cli setup --wait
update-cli setup --no-wait
update-cli setup --no-ui
update-cli setup --noui
update-cli setup --no-color
update-cli setup --root /path/to/project
```

`--setup` can also modify `update` and `rollback`:

```bash
update-cli update --setup
update-cli rollback 1.4.2 --setup
```

![Documentation — setup workflow](doc/images/documentation/09-setup.png)

### Setup catalog, task, workflow and external manifest

| Operation | Preferred form | Flag form |
|---|---|---|
| List catalog | `update-cli setup list` | `update-cli --setup-list` |
| List catalog as JSON | `update-cli setup list --json` | `update-cli --setup-list --json` |
| Run task | `update-cli setup task NAME` | `update-cli --setup-task NAME` |
| Run workflow | `update-cli setup workflow NAME` | `update-cli --setup-workflow NAME` |
| Use external manifest | `update-cli setup manifest FILE` | `update-cli --setup-manifest FILE` |

Task/workflow execution accepts `--details`, `--no-ui`/`--noui`, `--no-color`, and `--root`. External manifest execution additionally accepts selectors and wait behavior:

```bash
update-cli setup manifest ./setup.yaml
update-cli setup manifest ./setup.yaml --setup-list
update-cli setup manifest ./setup.yaml --setup-task build
update-cli setup manifest ./setup.yaml --setup-workflow ci
update-cli setup manifest ./setup.yaml --details --no-ui
update-cli setup manifest ./setup.yaml --wait
update-cli setup manifest ./setup.yaml --no-wait
```

`--setup-task` and `--setup-workflow` are mutually exclusive.

![Documentation — setup selectors](doc/images/documentation/10-setup-selectors.png)

### `setup.yaml` lifecycle commands

| Command | Supported forms and variations |
|---|---|
| Convert existing manifest | `update-cli convert-yaml`, `update-cli --convert-yaml`; `--dry-run/-n`, `--force/-f`, `--details`, `--root`, `--no-color` |
| Generate from project | `update-cli create-yaml --from project`, `update-cli --create-yaml --from project`; `--dry-run/-n`, `--force/-f`, `--details`, `--root`, `--no-color` |
| Generate from `setup.sh` | `update-cli create-yaml --from setup-script`; same modifiers plus optional `--with-ai` |
| Generate setup wrapper | `update-cli create-setup-script`, `update-cli --create-setup-script`; `--dry-run/-n`, `--force/-f`, `--details`, `--root`, `--no-color` |

Compatibility spelling retained for older scripts:

```bash
update-cli -create-setup-script
```

Important constraints:

- `--with-ai` is valid only with `create-yaml --from setup-script`.
- `convert-yaml`, `create-yaml`, and `create-setup-script` are mutually exclusive primary operations.
- `--force` is required before replacing an existing generated file where overwrite protection applies.

![Documentation — setup.yaml lifecycle](doc/images/documentation/11-yaml-management.png)

### Configuration commands

Preferred command forms:

```bash
update-cli config
update-cli config list
update-cli config edit
update-cli config use-template NAME
update-cli config --set KEY=VALUE
```

Historical flag forms remain supported:

```bash
update-cli --config
update-cli --config --list
update-cli --config --edit
update-cli --config --use-template NAME
update-cli --config --set KEY=VALUE
```

`--set` is repeatable and all assignments are validated together before the configuration is written atomically:

```bash
update-cli config \
  --set no-parameter="check,setup" \
  --set backup.keep=7 \
  --set retention.releases=10
```

Nested paths use dotted notation. Common examples include:

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
no-parameter
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

Lists may be comma-separated or supplied as JSON arrays; booleans and numeric values retain their JSON types.

![Documentation — configuration](doc/images/documentation/12-config.png)

### Configuration templates

```bash
update-cli templates list
update-cli templates list --details
update-cli templates edit
update-cli templates edit NAME
update-cli templates use NAME
```

Equivalent flag forms:

```bash
update-cli --templates --list
update-cli --templates --list --details
update-cli --templates --edit
update-cli --templates --edit NAME
update-cli --templates --use NAME
```

`templates edit` opens the template for editing; `templates use NAME` applies the selected configuration template to the current project.

![Documentation — templates](doc/images/documentation/13-templates.png)

### JSON output and machine-readable discovery

Important structured-output forms include:

```bash
update-cli --help --json
update-cli help --json
update-cli check --json
update-cli update --plan --json
update-cli backup --json
update-cli rollback 1.4.2 --json
update-cli restore latest --json
update-cli status --json
update-cli list --json
update-cli verify release.zip --json
update-cli doctor --json
update-cli clean --plan --json
update-cli cleanup --plan --json
update-cli history --json
update-cli setup list --json
update-cli upgrade --json
```

`update --json` is intentionally restricted to `update --plan --json`; a real update remains an interactive/plain transaction rather than a JSON mutation stream.

If **command-ui** is installed, the discovery contract can be consumed directly:

```bash
command-ui validate update-cli
command-ui inspect update-cli
command-ui update-cli
```

![Documentation — JSON and discovery](doc/images/documentation/14-json-discovery.png)

### Release-source overrides

Release discovery commands (`check`, `update`, `status`, `list`, `verify`) can override the configured source for one invocation.

Download/folder source:

```bash
update-cli check --downloads ~/Downloads
update-cli check --from download --folder ~/Downloads
```

URL source:

```bash
update-cli check \
  --from url \
  --url https://example.org/releases/demo-app-v1.2.0.zip
```

Repository source:

```bash
update-cli check \
  --from repository \
  --repository github.com/acme/demo-app
```

The same source modifiers can be combined with `update`, `status`, `list`, and `verify` where advertised by `--help --json`.

![Documentation — source overrides](doc/images/documentation/15-source-overrides.png)

### Command aliases and compatibility spellings

The command-token interface is an alias layer over the established flag interface. The following pairs are equivalent:

```text
check                         --check
update release.zip            --update release.zip
backup                        --backup
rollback 1.2.3                --rollback 1.2.3
restore latest                --restore latest
status                        --status
list                          --list
verify release.zip            --verify release.zip
doctor                        --doctor
clean                         --clean
cleanup                       --cleanup
history                       --history
init demo-app                 --init demo-app
upgrade                       --upgrade
unlock                        --unlock
setup                         --setup
setup list                    --setup-list
setup task build              --setup-task build
setup workflow ci             --setup-workflow ci
setup manifest setup.yaml     --setup-manifest setup.yaml
convert-yaml                  --convert-yaml
create-yaml                   --create-yaml
create-setup-script           --create-setup-script
config                        --config
templates list                --templates --list
```

Short and compatibility options:

```text
-r DIR       --root DIR
-a ZIP       --archive ZIP
-d DIR       --downloads DIR
-n           --dry-run
-f           --force
-V           --version
--noui       --no-ui
---no-ui     --no-ui
```

![Documentation — aliases](doc/images/documentation/16-aliases.png)

### No-parameter behavior

With no explicit primary command, Update CLI executes the actions configured in `.updater-cli/config.json` under `"no parameter"`.

Default initialized configuration:

```json
{
  "no parameter": ["check"]
}
```

Automatic setup after an accepted update:

```bash
update-cli config --set no-parameter="check,setup"
```

Then:

```bash
update-cli
```

uses the configured sequence instead of requiring explicit command flags.


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
│ Update CLI Version 1.0.2   |   my-project v1.2.4   |   Setup       │
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
