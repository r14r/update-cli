# update-cli

**Transactional project updates from ZIP releases or Git repositories.**

Current release: **2.16.6**

Update CLI separates installation defaults, local runtime state, and versioned project configuration/automation:

```text
INSTALLFOLDER/../etc/update-cli/config.json   installation-wide host/user defaults
.update-cli/config.json                       local runtime state and project fallbacks
update-cli.yaml                               versioned project update/setup/run configuration
current/update-cli.yaml                       installed release manifest, also checked by doctor
setup.yaml                                    legacy setup manifest fallback
```

For the normal installation `/usr/local/bin/update-cli`, the global directory is `/usr/local/etc/update-cli/`. Effective update settings are resolved in this order:

```text
global config.json < local .update-cli/config.json < update-cli.yaml < explicit CLI options
```

Project-dependent settings can therefore travel with the project in `update-cli.yaml`, while host security limits, `source.defaultUser`, CLI defaults and templates remain outside the release-controlled manifest.

`.update-cli/` is reserved runtime state at the project root. It must never be shipped as release payload or appear below `current/`; Update CLI now enforces this during release/current synchronization.

## CLI syntax

All invocations use one canonical grammar:

```bash
update-cli <command> --<parameter> --<parameter>
```

Only the command is written without `--`. Every option and every value-bearing selector is a parameter. There are no command aliases with a leading `--` and no bare secondary subcommands. Examples:

```bash
update-cli version
update-cli help --command setup --details
update-cli update --archive app-v1.2.3.zip --no-setup
update-cli setup --list
update-cli setup --run go-vet
update-cli config --migrate
update-cli schema --version
update-cli releases --list --json
```



## What changed in 2.16.6

### Build and install are separate operations

`just install` no longer depends on `just build`. It installs an already-built `dist/update-cli` plus the global support files into the configured destination. If the binary is missing, the recipe stops with a clear error and asks you to run `just build` first.

The project setup manifest keeps fresh-source setup functional by declaring the build task explicitly as a prerequisite of its install task. This makes the build visible in the setup workflow instead of hiding it inside `just install`.

## What changed in 2.16.5

### Stable Docker timeout regression tests

Docker Compose timeout tests no longer use strict wall-clock upper bounds. Under a heavily loaded macOS host, process startup and scheduler latency can exceed those bounds even when the command timeout and `WaitDelay` behavior are correct.

The tests now verify the actual contract directly: the command returns a timeout error and every relevant `exec.Cmd` receives the configured `WaitDelay`. This preserves regression coverage without making `just build` or `just install` dependent on machine load.

The setup bootstrap scripts also use a Bash-3.2-safe expansion for optional forwarded arguments, so stock macOS `/bin/bash` no longer aborts with `FORWARD_ARGS[@]: unbound variable` when no extra setup options are supplied.

## What changed in 2.16.4

### Docker Compose timeout reliability on macOS

Docker Compose subprocesses now set a bounded `os/exec.Cmd.WaitDelay`. After a context timeout or cancellation, update-cli no longer waits for stdout/stderr pipes that may still be held open by a child process such as the Docker Compose CLI plugin.

The timeout regression tests also allow scheduler headroom while still proving that a simulated five-second hang is terminated well before the child process would finish normally.

## What changed in 2.16.3

### Release archive compatibility

Download-folder and explicit archive updates now accept both supported release names:

```text
<project>-v<MAJOR>.<MINOR>.<PATCH>.zip
<project>-<MAJOR>.<MINOR>.<PATCH>.zip
```

The `v` prefix remains fully backward compatible but is no longer required.

### Docker lifecycle timeout hardening

Docker Compose probing and status checks can no longer block an update indefinitely when Docker Desktop or the Docker Engine is unhealthy. `docker compose version` and `docker compose ... ps -q` use bounded probe/status timeouts, while `up`/`down` lifecycle actions use a bounded action timeout. Timeout failures retain the full command context so `auto` lifecycle mode can skip unavailable Docker cleanly and `required` mode can fail explicitly.

## What changed in 2.16.2

### Doctor manifest selection and interactive repair

`update-cli doctor` treats the root and installed `current/` manifest as alternative valid project-manifest locations. When Update CLI is started from the managed project root and `current/update-cli.yaml` exists, a missing root-level `update-cli.yaml` is shown in the inventory row as `(fehlt)` but is **not** emitted as a warning. An error is reported only when neither location contains a usable manifest.

The `Projektkonfiguration` warning is now conditional. It means that project-specific runtime values still come from local `.update-cli/config.json` without an equivalent override in the active `update-cli.yaml`. Doctor lists the concrete mappings, for example `releaseDir → update.releaseDir` or `source.repository → update.source`. This is an architecture/reproducibility warning, not a schema migration error. Host-specific values such as `security.*`, `source.defaultUser`, and CLI defaults remain intentionally in `config.json`.

Doctor can now repair deterministic schema/structure problems interactively:

```bash
update-cli doctor --fix
```

Before changing any file it prints the planned config/manifest repairs and then asks:

```text
Geplante Reparaturen durchführen? [y/N]
```

Only `y`/`yes` (and German `j`/`ja`) applies the fix. `n`, Enter, or any other answer leaves all files unchanged. Normal timestamped backups are created by the existing repair engine before changed files are written. `doctor --fix` is intentionally separate from the advisory project-configuration warning and cannot be combined with `--json` or `--migrate`.

## What changed in 2.16.1

### Doctor explains the UI migration badge

`update-cli doctor` now reports the exact migration state used by the fullscreen header. The UI and Doctor share one migration-requirement inspection, so they cannot independently disagree about whether **Migration required** should be shown.

Example when a migration is still pending:

```text
Migration required   JA
Migration reason     manifest-current — unbekannte/entfernbare Felder: project.oldField
Migration file       /project/current/update-cli.yaml
```

If the runtime config and all relevant project manifests are current, Doctor explicitly reports:

```text
Migration required   nein
```

`update-cli doctor --json` exposes the same information as `migrationRequired` and `migrationReasons`. A successful `update-cli config --migrate` only updates `.update-cli/config.json`; if the red badge remains, `doctor` now identifies whether the remaining cause is the root manifest or the installed `current/update-cli.yaml`.

## What changed in 2.16.0

### Command-specific detailed help

`--details` can now be combined with help to show the purpose, command-specific options, and examples instead of only the compact usage summary:

```bash
update-cli help --command setup --details
update-cli help --command setup --details
update-cli help --command update --details
update-cli help --details
```

Without `--details`, help remains compact. The JSON discovery output (`help --json`) remains machine-readable and now also advertises the setup step selector.

### Setup step inspection and execution

`setup --list` is now a real standalone setup command and lists workflows, tasks, and every setup step including its `id`, task, name, and operation:

```bash
update-cli setup --list
update-cli setup --list --json
```

A single setup step can be executed by its manifest `id`:

```bash
update-cli setup --run go-vet
update-cli setup --run go-vet --details
```

Only the selected step is executed. Task dependencies and neighboring steps are **not** run automatically. The step's own `when`, timeout, retry, and failure policy still apply. Step IDs should therefore be unique within the manifest; duplicate IDs are rejected as ambiguous.

`setup --list` and `setup --run` are standalone commands and cannot be combined with `update`, `rollback`, or `restore`. This removes the previous contradictory mode errors around `setup --list`.

## What changed in 2.14.6

### Migration badge only for real migrations

The fullscreen **Migration required** badge now reflects only an actual pending runtime-config or project-manifest migration/repair. A valid current-schema `update-cli.yaml` is no longer marked as requiring migration merely because the internal repair renderer would format the YAML differently.

Running `update-cli config --migrate` on an already current schema therefore clears the badge unless a root/current manifest still has a genuine schema mismatch, unknown field, invalid value, legacy filename/structure, or another deterministic structural repair requirement. Pure formatting differences are ignored.

## What changed in 2.14.5

### Canonical runtime directory and migration warning

`.update-cli/` is now the only project runtime-state directory used anywhere in code, tests, help text, packaging, and documentation. `update-cli config --migrate` upgrades `.update-cli/config.json` in place, and its timestamped backup is written beside the source file in `.update-cli/`.

The fullscreen UI now shows a right-aligned **Migration required** badge in white text on a red background whenever the runtime config or the root/current project manifest requires a supported migration or deterministic repair. The badge stays visible while the UI changes between check, update, and setup phases.

## What changed in 2.14.4

### Stable macOS test paths and concise test output

macOS exposes the same temporary directory through both `/var/folders/...` and `/private/var/folders/...`. The root-discovery and install integration tests now compare canonical filesystem paths, so these aliases no longer produce false failures.

`just test` intentionally exercises many independent integration scenarios: setup schema 1/2, migration, failed migration, rollback, Docker lifecycle, `no-setup`, init, install, doctor/fix and other workflows. Go normally suppresses stdout from passing test packages. If one test in a package fails, Go prints the buffered output from the entire package, which can make the individual scenarios look like duplicated setup runs. With the macOS path assertions fixed, a successful `just test` remains compact.

## What changed in 2.14.3

### Parameterless update aliases no longer fall back to an install prompt

The canonical runtime key remains `"no parameter"`, but Update CLI now also accepts the common spellings `"no-param"`, `"no-parameter"`, and `"noParameter"`. A scalar string or a string list is supported. For example:

```json
"no-param": "update"
```

is interpreted exactly like:

```json
"no parameter": ["update"]
```

Therefore a parameterless `update-cli` invocation enters the update path directly and does **not** first execute `check` or ask `Update jetzt installieren?`. `update-cli fix` / `config migrate` normalize alias spellings back to the canonical `"no parameter"` key.

To also suppress the separate project-setup question after a successful update, configure:

```json
"no parameter": ["update", "no-setup"]
```

## What changed in 2.14.2

### Bootstrap-safe commands and runtime configuration

All documented actions are first-class commands without a leading `--`. In particular, version inspection does not need project configuration:

```bash
update-cli version
update-cli version
```

The normal runtime-config path is now resilient. Update CLI first tries the strict parser. If an existing local/global `config.json` uses a known legacy or transition structure, Update CLI creates the normal timestamped backup, repairs the files to the current runtime schema, and retries the strict parser. This includes top-level `defaultUser`, historical setup-policy aliases, unknown obsolete fields, and older/newer schema markers whose known content can be safely normalized.

A parameterless invocation therefore no longer gets permanently blocked by errors such as:

```text
json: unknown field "defaultUser"
schemaVersion 9 ist neuer als unterstützt 7
```

After installing the current binary, the configuration is migrated to runtime schema 9 before the configured no-parameter action is executed. Explicit `update-cli fix` and `update-cli doctor --migrate` remain available for manual diagnostics/migration.

### Command-first syntax

The canonical form uses commands rather than action flags:

```bash
update-cli version
update-cli update
update-cli update --plan --json
update-cli setup
update-cli install
update-cli doctor
update-cli doctor --migrate
update-cli fix
update-cli schema --version
update-cli schema --view
update-cli config --check
update-cli config --migrate
update-cli releases --list --json
```

Legacy action flags such as `--version`, `--update`, `--setup`, and `--doctor` remain compatibility aliases.

## What changed in 2.14.0

- Every public action uses exactly one command without a leading `--`; every following option uses a `--parameter`.
- Version handling introduced in 2.14.0 has been simplified by 2.14.1: `VERSION` is now the only release-version source.

### Self-healing runtime config compatibility

Normal commands now normalize known historical `keepRsyncOnError` placements **before** the strict `config.json` decoder runs. This prevents an older/transitional runtime config from blocking `update`, `upgrade`, `setup`, `doctor`, or parameterless execution before `fix` can be reached.

The canonical runtime JSON remains:

```json
{
  "setup": {
    "keepRsyncOnError": false
  }
}
```

The loader accepts and normalizes these historical aliases in memory:

```text
keepRsyncOnError
sync.keepRsyncOnError
sync.keepOnSetupError
setup.keepOnSetupError
```

Use `update-cli fix` to persist the canonical representation and create the normal timestamped backup. Project YAML continues to use the separate versioned setting `update.sync.keepOnSetupError`, which overrides the JSON fallback.

### Binary version comes directly from VERSION

The compiled CLI embeds `VERSION` directly. The build no longer supplies any second version value through `-ldflags`, so the source file and reported binary version cannot diverge through build configuration.

## What changed in 2.13.2

### One canonical setup/build path

The Update CLI project now uses the same installation path for both manual installation and `update-cli setup`: `just install`. The setup manifest no longer duplicates the Go format/vet/test/race/build/deploy sequence. Before invoking `just install`, the internal `UPDATE_CLI_SETUP_RUNNING` marker is removed from the nested build/test environment so setup-driven validation behaves like a direct shell invocation.

The previous recovery `WARN` lines shown by `lib/updater` tests are expected output from tests that deliberately simulate failed updates, Docker outages and rollback. They are not themselves test failures. Keeping the project setup on the canonical Just pipeline removes the divergent runner path that produced the reported `Go-Tests ausführen` failure while direct `just build` / `just install` succeeded.

### README terminal demos

Two VHS tapes now document the main onboarding paths:

```text
docs/tapes/install.tape
docs/tapes/quickstart.tape
```

Render both with:

```bash
just tapes
```

Generated recordings are written to `docs/videos/`. The INSTALL tape uses `UPDATE_CLI_INSTALL_BIN_DIR` so it installs into a disposable project-local demo prefix instead of modifying `/usr/local`.

## What changed in 2.13.1

### Parameterless update without setup

`"no parameter"` now accepts `"no-setup"` as an `"update"` modifier:

```json
"no parameter": ["update", "no-setup"]
```

With this configuration, invoking `update-cli` without a command performs the update directly, never opens the project-setup confirmation prompt, does not run project setup, and exits after the normal update transaction completes. `no-setup` is valid only together with `update`; `setup` and `no-setup` are mutually exclusive.

Newly initialized projects use `["update", "no-setup"]` as the default parameterless action. Existing projects can enable it with:

```bash
update-cli config --set no-parameter=update,no-setup
```

## What changed in 2.13.0

### Faster transactional rollback preparation

The normal update path no longer copies the complete `current/` tree into a temporary transaction snapshot when the currently installed version already has a valid immutable `release/<VERSION>/` directory. Instead, Update CLI records that release as the rollback basis. On a later activation/setup/service/healthcheck failure it synchronizes the previous release back to `current/` while applying the same `sync.preserve` rules.

This means directories such as `node_modules/`, `vendor/`, build output and other large generated trees are no longer duplicated before every successful update. The old exact full snapshot remains as a safety fallback when no matching previous release is available.

Rollback strategy:

```text
current/VERSION -> release/<VERSION>/ exists and matches
  -> lightweight release-based rollback metadata

no usable previous release
  -> exact temporary current/ snapshot (fallback)
```

`sync.preserve` remains the contract for state that must survive release changes. Preserved paths are not overwritten by release-based recovery.

### More robust `update-cli.yaml` diagnostics and repair

Manifest execution remains strict, but parser failures now include a non-mutating repair preview instead of only the first unknown field. Known unknown fields and safely normalizable values/structures are listed together with the appropriate repair command.

```bash
update-cli doctor
update-cli doctor --migrate
update-cli fix
```

- `doctor` inspects both `./update-cli.yaml` and `./current/update-cli.yaml` and reports all deterministically repairable drift it can identify.
- `doctor --migrate` first performs schema migration and then automatically applies structural repair for known field/value problems.
- `fix` repairs both root and current manifests when both exist and keeps timestamped backups before every actual change.
- YAML syntax/content that cannot be repaired without guessing remains a hard error.

## What changed in 2.12.3

This patch fixes the reported `current/current` setup path and the incomplete 2.12.2 source archive.

- `.update-cli/` is reserved project-root runtime state. Release extraction and deployment never copy them into `release/` or `current/`.
- Existing accidental `current/.update-cli/` directories are removed during the next current synchronization.
- When a command is started directly inside a directory named `current`, the parent project configuration wins even if a stale nested runtime directory exists. This prevents `current/current/update-cli.yaml`.
- Source packaging is now reproducible through `scripts/package-source.sh`; it requires core source packages such as `lib/backup`, excludes runtime/build state, and validates the archive after creation.
- Historical note: this release still used `.release-version`; since 2.14.1, `VERSION` is authoritative and `.release-version` is only a legacy fallback.
- `.update-cli/config.json` keeps the compatible runtime fallback `setup.keepRsyncOnError`; project YAML overrides it through `update.sync.keepOnSetupError`.
- Superseded in 2.14.1: managed releases now use `current/VERSION` as the canonical installed version.
- The Update CLI project manifest contains an early `version-sync` setup step so a stale installed 2.10.x binary can bootstrap the corrected source without first understanding newer manifest fields.

## What changed in 2.12.1

This historical patch attempted to harden the project-manifest bootstrap path around the transitional `update.setup` layout and synchronized the project-root `VERSION` with the active installation. Version 2.12.2 supersedes that YAML layout with `update.sync.keepOnSetupError`.

- `update.sync.keepOnSetupError` is explicitly covered by parser/schema regression tests and remains a supported schema-2 project setting.
- After a successful update, rollback or restore, `<project-root>/VERSION` is written to the same semantic version as `current/VERSION`.
- A standalone managed `update-cli setup` also synchronizes the root `VERSION` after setup succeeds.
- When an update resolves to an already-installed version, the root `VERSION` is repaired to that installed version instead of remaining stale.
- If `setup.keepRsyncOnError=true` keeps a new `current/` after a setup failure, the root `VERSION` follows the retained current version as well.

This prevents a stale root `VERSION` from causing a subsequent `just build`/`just install` to advertise or install an older Update CLI version.

## What changed in 2.12.0

Project releases can now provide a `migrate.sh` that runs automatically immediately before project setup. Migration state is tracked per installed semantic version, so the same release migration is not executed again by a later `update-cli setup`.

The workflow is:

```text
release unpack/rsync
  -> verify current/VERSION
  -> migrate.sh (when setup is actually executed and migration is pending)
  -> write .update-cli/.migration.done.<VERSION>
  -> project setup
```

Rules:

- The migration script is `current/migrate.sh` for a managed project. When setup runs directly from a source project, `migrate.sh` is resolved beside that project's manifest.
- The completion marker is stored persistently as `<project-root>/.update-cli/.migration.done.<VERSION>`; it is deliberately outside replaceable `current/`.
- `migrate.sh` is invoked with `bash`, so the executable bit is not required.
- The script receives `UPDATE_CLI_MIGRATION=1`, `UPDATE_CLI_PROJECT_ROOT`, `UPDATE_CLI_CURRENT_DIR`, and `UPDATE_CLI_VERSION`.
- A marker is created only after a successful migration. A failed migration stops setup, leaves no done marker, and is retried by the next setup attempt.
- If the marker already exists, `update-cli setup` skips `migrate.sh` and continues with setup.
- Installing a different semantic version uses a different marker and therefore runs that version's migration once.
- `--no-setup` or an interactively declined setup also defers migration, because migration is part of the pre-setup workflow. Running `update-cli setup` later performs the pending migration first.

Migration scripts should still be idempotent themselves. The marker prevents normal repeated execution, while script-level idempotency protects manual execution, copied installations, or deliberately removed state markers.

Example project migration:

```bash
#!/usr/bin/env bash
set -Eeuo pipefail

: "${UPDATE_CLI_PROJECT_ROOT:?missing UPDATE_CLI_PROJECT_ROOT}"
: "${UPDATE_CLI_CURRENT_DIR:?missing UPDATE_CLI_CURRENT_DIR}"
: "${UPDATE_CLI_VERSION:?missing UPDATE_CLI_VERSION}"

# Example: create a new persistent directory introduced by this release.
mkdir -p "${UPDATE_CLI_PROJECT_ROOT}/data/cache"

# Example: migrate an old file only when the new target does not yet exist.
if [[ -f "${UPDATE_CLI_PROJECT_ROOT}/data/settings.json"    && ! -f "${UPDATE_CLI_PROJECT_ROOT}/data/config/settings.json" ]]; then
  mkdir -p "${UPDATE_CLI_PROJECT_ROOT}/data/config"
  cp "${UPDATE_CLI_PROJECT_ROOT}/data/settings.json"      "${UPDATE_CLI_PROJECT_ROOT}/data/config/settings.json"
fi
```

## What changed in 2.11.0

Project-dependent updater settings can now be versioned directly in `update-cli.yaml`. The YAML layer is applied after the global and local JSON configuration, so project settings override the corresponding `config.json` fallback values. Explicit command-line source options remain the highest-priority layer.

Supported project-versioned settings are:

- `project.slug` as the updater project/artifact slug
- `update.mode` and `update.source`
- `update.releaseDir` and `update.currentDir`
- `update.backup.directory` / `update.backup.keep`
- `update.retention.releases`
- `update.sync.preserve`
- `update.sync.keepOnSetupError`
- `update.docker.lifecycle`
- `update.healthcheck.*`

The following remain config-only by design: `source.defaultUser`, `security.*`, `no parameter`, global templates and installation-specific configuration paths. In particular, a repository-controlled `update-cli.yaml` cannot weaken host archive/HTTP security limits.

`update-cli doctor` checks the complete project layout. From the project root it inspects `.update-cli/config.json`, `./update-cli.yaml`, and `./current/update-cli.yaml`. Root/current manifests are alternative valid locations: if one valid manifest exists, the missing alternate location is not a warning. When started inside a directory named `current`, Doctor recognizes the parent project and uses `../.update-cli/config.json`. It reports schema versions, root/current differences, effective configuration priority, migration reasons, and only those project-configuration values that still lack a versioned YAML override. `doctor --migrate` performs schema migration; `doctor --fix` previews deterministic repairs, asks y/n, and applies them only after confirmation.

## What changed in 2.10.0

`update-cli install` runs the project's canonical `just install` recipe without running the complete setup workflow.

- In a managed Update CLI project, `current/` is used when it contains `justfile` or `Justfile`.
- In a source project without an active `current/` justfile, the project root is used.
- The command executes exactly `just install` and forwards stdin/stdout/stderr to the terminal.
- Missing `just`, `justfile`, or `Justfile` is reported as an explicit error.
- `--install` remains available as a compatibility alias, while `install` is the canonical command-first syntax.

Example:

```bash
update-cli install
```

## What changed in 2.9.1

Interactive setup rejection is now treated as an intentional deployment-only update. When the prompt
`Projekt-Setup ist verfügbar. Jetzt ausführen?` is answered with `n`, Update CLI keeps the verified
rsync result and does not run the post-setup activation steps that could otherwise undo the update:

- `Projekt-Setup ausführen` remains skipped.
- `Vorher laufende Docker-Dienste starten` is skipped instead of restarting the new Compose stack.
- `Healthcheck der neuen Installation ausführen` is skipped because activation was intentionally deferred.
- A Docker stack that Update CLI stopped before synchronization remains stopped for manual setup/start.
- The release is still activated and the successful update state is committed; no rollback is triggered merely because setup was declined.

This specifically fixes the interactive `n` path. Explicit automation options such as `--no-setup` retain their previous Docker lifecycle semantics.

## What changed in 2.9.0

`update-cli fix` repairs a project when legacy or invalid Update CLI metadata prevents normal commands from loading it. Run it from the project root:

```bash
update-cli fix
```

The command checks and repairs:

- project-local `.update-cli/config.json`
- installation-wide `config.json` when present, for example `/usr/local/etc/update-cli/config.json`
- `update-cli.yaml` in the project root

Known legacy fields are migrated before strict parsing. In particular, the old top-level form:

```json
{
  "defaultUser": "r1r"
}
```

is rewritten to the canonical structure:

```json
{
  "source": {
    "defaultUser": "r1r"
  }
}
```

Unknown fields in supported config/manifest sections are removed, current schema versions are written, and safely repairable wrong values are normalized. Every changed existing file receives a timestamped backup before replacement. After repair, the normal strict config and manifest parsers are run again; `fix` only succeeds when the repaired project is valid. Malformed JSON/YAML syntax that cannot be corrected without guessing user content remains an explicit error.

Structured output is available with:

```bash
update-cli fix --json
```

The public CLI is command-first in 2.9.0. Primary actions no longer require a leading `--`:

```bash
update-cli upgrade
update-cli unlock
update-cli howto
update-cli version
update-cli check --no-ask
update-cli update --archive myapp-v1.2.3.zip --backup --setup
update-cli backup
update-cli rollback --version 1.2.2 --setup
update-cli restore --snapshot latest
update-cli status --json
update-cli releases --list --json
update-cli verify --archive myapp-v1.2.3.zip
```

Options always use `--`, for example `--json`, `--debug`, `--setup`, `--root`, and `--no-ui`. Command-style aliases with a leading `--` and bare secondary subcommands are rejected.
The project `justfile` uses the same command-first syntax; its release inventory recipe is now `just releases` (with `legacy-list` retained only for compatibility testing).

## What changed in 2.8.0

- add the runtime policy `setup.keepRsyncOnError` to `.update-cli/config.json`. The default is `false`, preserving the previous transactional rollback behavior.
- when `setup.keepRsyncOnError=true` and the update reaches the setup phase, a setup failure no longer restores the previous `current/` tree. The verified rsync result remains installed for inspection, repair, or a later standalone `update-cli setup`.
- the matching staged release is promoted into the versioned `release/<version>/` directory before the failed update returns, keeping `release/` and `current/` consistent.
- the setup command still returns an error and history records the update with status `failed` and phase `setup`; the option changes file-state recovery only.
- Preserve rules remain active: existing `.env`, `data/`, `uploads/`, `storage/`, and other configured `sync.preserve` paths keep their local content even when the failed setup's rsync result is retained.
- failures before setup or after setup (rsync, verification, service restart, healthcheck, release activation, metadata) continue to use normal transactional recovery.
- runtime config schemaVersion is now **9**. Global and local config layers support the new setting; an explicit local `false` overrides a global `true`.

## What changed in 2.7.0

- `update-cli doctor` now validates the project manifest directly in the current project folder and no longer requires `.update-cli/config.json`.
- the canonical file checked by doctor is `update-cli.yaml`; a legacy `setup.yaml` is still recognized and reported.
- add `update-cli doctor --migrate` to upgrade an older manifest to the newest supported manifest schema.
- migrating an existing `update-cli.yaml` creates a timestamped backup before replacement.
- when only legacy `setup.yaml` exists, `doctor --migrate` keeps it untouched and creates the canonical `update-cli.yaml` using the newest supported schema.
- `doctor --migrate` is distinct from `config --migrate`: doctor migrates the project manifest, while config migration continues to migrate runtime configuration only.
- doctor reports the detected manifest path, current schema version, latest supported schema version, project name, and declared required commands.
- add CLI discovery/help metadata and regression coverage for doctor validation, migration, canonicalization, and flag compatibility.

## What changed in 2.6.2

- add `update-cli schema --version` to print the canonical `update-cli.yaml` manifest schema version.

- keep standalone `update-cli version` unchanged; it continues to print the Update CLI application version.
- expose the schema version option through CLI discovery/help metadata and cover the parsing semantics with regression tests.
- synchronize release metadata and packaging checks to 2.6.2.

## What changed in 2.6.1

- patch release of the verified 2.6.x implementation with release metadata synchronized to 2.6.1.
- no functional behavior changes from 2.6.0; all formatting, vet, unit, integration, race, init, debug, schema, and setup-fallback checks are rerun before packaging.
- release packaging verifies that the ZIP filename, `VERSION`, README current release, and top `RELEASE_NOTES.md` heading all resolve to 2.6.1.

## What changed in 2.6.0

- `--debug` is now a global option for every command. It switches to direct detailed output, works with the configured no-parameter action (`update-cli --debug`), and reports the resolved project root, global/local config and template paths, merge order, effective source, release/current paths, preserve rules, and rsync deployment semantics.
- accept both `source.defaultUser` and the historical top-level `defaultUser` in `config.json`; migration normalizes the latter to `source.defaultUser`.
- keep both repository bootstrap spellings fully supported and regression-tested:
  - `update-cli init --project PROJECT --from-repository URL`
  - `update-cli init --project PROJECT --from-repository REPOSITORY`
- keep download bootstrap as `update-cli init --project PROJECT`; it uses the configured/default download folder and selects the newest matching `PROJECT-v<MAJOR>.<MINOR>.<PATCH>.zip` or `PROJECT-<MAJOR>.<MINOR>.<PATCH>.zip`.
- repository and download bootstrap continue to use the current working directory as the project root unless `--root` is supplied.
- setup-manifest parsing is forward-compatible for unknown top-level extension/transition fields (including historical metadata such as `update:` in older layouts), while known sections and executable step fields remain strictly validated.
- release version markers are synchronized to 2.6.0 and checked before packaging.

## What changed in 2.5.0

- added installation-wide updater defaults at `INSTALLFOLDER/../etc/update-cli/`; an executable at `/usr/local/bin/update-cli` therefore uses `/usr/local/etc/update-cli/`.
- `config.json` is now loaded in two layers: global `config.json` first, then the project-local `.update-cli/config.json` as an override. Nested JSON objects are merged recursively.
- `sync.preserve` is cumulative: global preserve/exclude defaults form the minimum policy and local project entries are appended without duplicates. New projects can therefore inherit the centrally maintained rsync exclude list.
- `templates.json` is also layered global -> local. A project-local template with the same name replaces the global template; additional local templates are appended.
- `update-cli config --list` now shows both global and local `config.json`/`templates.json` paths plus the local `history.jsonl`.
- `just install` creates global `config.json` and `templates.json` defaults when missing, but never overwrites existing global administrator files.
- added regression tests for installation-path resolution, global/local config merging, cumulative preserve rules, template overlays, and `config --list`.

## What changed in 2.4.3

- fixed release creation so `.env`, `.env.example`, and other `.env.*` files contained in the source artifact are copied into `release/` instead of being dropped unconditionally.
- fixed `sync.preserve` semantics for first installation: a protected file or directory that does not yet exist in `current/` is seeded once from the release; an existing local copy remains protected from overwrite and deletion.
- this means a project ZIP containing `.env` and `.env.example` now installs both files on an empty `current/`, while subsequent updates continue to honor the configured preserve rules.
- backup snapshots continue to exclude `.env` secrets; the change applies to release/current deployment, not ordinary backup export.
- added regression coverage for release dotfiles, missing protected files/directories, dry-run reporting, and the supplied GrapesJS artifact flow.

## What changed in 2.4.2

- fixed release metadata consistency so `VERSION`, README current release, and the release-notes heading are checked together.
- added a regression test that fails the build when those release version markers diverge.
- release packaging now verifies that the ZIP filename version equals the packaged `VERSION` value before delivery.

## What changed in 2.4.1

- added regression coverage for the two supported local bootstrap commands exactly as users invoke them.
- `update-cli init --project <project>` is end-to-end tested against the configured/default download folder and selects the newest matching `<project>-v<MAJOR>.<MINOR>.<PATCH>.zip` or `<project>-<MAJOR>.<MINOR>.<PATCH>.zip`.
- `update-cli init --project <project> --from-repository <repository>` is end-to-end tested without requiring the legacy `--repository` option.
- the repository argument continues to accept a full GitHub HTTPS URL, `USER/REPO`, or `REPO`; the one-part form uses `source.defaultUser`.
- both bootstrap forms install into the current working directory unless `--root` is explicitly provided.

## What changed in 2.4.0

- `--from-repository` now takes the repository directly: `--from-repository REPOSITORY`.
- accepted forms are a full GitHub URL (`https://github.com/r14r/git-cli`), `USER/REPO` (`r14r/ollama-cli`), or only `REPO` (`ollama-cli`).
- one-part repository names use `source.defaultUser` from `.update-cli/config.json`; the default is `r1r`.
- repository shorthand values are normalized to canonical HTTPS GitHub clone URLs ending in `.git`.
- `.update-cli/config.json` schemaVersion is now **8** and persists `source.defaultUser`.
- the previous `--from-repository --repository URL` spelling remains accepted as a compatibility alias, but is no longer the documented form.
- README and tests were updated for all three repository forms.

## What changed in 2.3.1

- `update-cli init --project <project>` now always uses the **current working directory** as the project root when `--root` is not supplied.
- the project argument only defines the project name used in `.update-cli/config.json` and release matching; it no longer creates a nested `<project>/` directory.
- `--root <path>` remains the explicit override when a different project root is required.
- both download and repository bootstrap integration tests now verify initialization directly in the current directory.

## What changed in 2.3.0

- added `update-cli schema --view` to print the canonical JSON Schema for `update-cli.yaml` schemaVersion 2 to stdout.
- added `update-cli schema --save <file.json>` to save exactly the same schema to a JSON file; missing parent directories are created automatically.
- the exported schema describes project automation plus the project-versioned `update:` settings. Host/user policy such as `security.*` and `source.defaultUser` remains in `.update-cli/config.json`.
- `schema --view`, `schema --save`, and `schema --version` are mutually exclusive and the `schema` command requires exactly one of them.
- CLI discovery/help now exposes the new `schema` command.
- Go **1.26.5** remains the release toolchain.

The local bootstrap behavior remains unchanged: `--init <project>` uses the newest matching download ZIP, while `--init <project> --from-repository <repository>` bootstraps from GitHub.

## Debug output

`--debug` may be added to every command, before or after the command name. It disables the fullscreen TUI path and prints detailed execution/progress steps directly.

```bash
update-cli help --debug
update-cli update --debug
update-cli config --list --debug
update-cli config --list --debug
update-cli init --project demo-app --debug
```

When `--debug` is used without an explicit command, Update CLI executes the configured no-parameter action (for example `check`) with debug output.

## Project layout

```text
/usr/local/etc/update-cli/             # for /usr/local/bin/update-cli
├── config.json                        # global updater defaults
└── templates.json                     # global templates

project/
├── .update-cli/
│   ├── config.json        # project overrides/source configuration
│   ├── history.jsonl      # project-local runtime history
│   ├── templates.json     # optional local template overrides/additions
│   └── repository/        # persistent Git checkout for pull mode
├── update-cli.yaml        # preferred setup/run automation
├── setup.yaml             # optional legacy setup manifest
├── release/
├── current/
└── backup/
```

Older projects may still use an earlier schema inside `.update-cli/config.json`. `update-cli config --migrate` upgrades that file in place and keeps its timestamped backup in the same `.update-cli/` directory.

## INSTALL

### Prerequisites

For a source installation you need Go, `just`, `rsync`, `zip` and `unzip`. The release source keeps its intended Go toolchain in `go.mod`; use that version for production builds.

### Build and install from source

The canonical installation workflow is deliberately short:

```bash
just build
just install
```

`just build` runs formatting checks, `go vet`, the complete Go test suite and race tests before producing `dist/update-cli`. `just install` depends on that build and then installs the binary plus the global Update CLI support files.

With the default build configuration:

```text
binary:       /usr/local/bin/update-cli
global config:/usr/local/etc/update-cli/
```

Existing global `config.json` and `templates.json` are preserved. Default files are created only when no corresponding global file exists yet. Verify the installation with:

```bash
update-cli version
update-cli config --list
```

`update-cli install` is the CLI wrapper for the same project operation and executes exactly `just install` in the active project/source directory:

```bash
update-cli install
```

For local development, CI fixtures and recordings, the Just recipe accepts a non-system binary directory:

```bash
UPDATE_CLI_INSTALL_BIN_DIR="$PWD/.demo-install/bin" just install
.demo-install/bin/update-cli version
```

The matching VHS recording is defined in [`docs/tapes/install.tape`](docs/tapes/install.tape). Render it with:

```bash
vhs docs/tapes/install.tape
# or
just tape-install
```

The generated recording is written to `docs/videos/install.gif`.

## QUICKSTART

The current working directory is always the project root during `init`. The value after `init` is the project name/slug; it never causes Update CLI to create another nested project directory.

### Bootstrap from a downloaded release

Place release ZIPs in the configured download directory, normally `$HOME/Downloads`. File names must use valid semantic versions:

```text
<project>-v<MAJOR>.<MINOR>.<PATCH>.zip
<project>-<MAJOR>.<MINOR>.<PATCH>.zip
```

For example:

```text
demo-app-v1.2.0.zip
demo-app-v1.3.0.zip
demo-app-v2.0.0.zip
```

Then create or enter the desired project root and initialize it:

```bash
mkdir -p ~/projects/demo-app-DEV
cd ~/projects/demo-app-DEV
update-cli init --project demo-app
```

When several matching archives exist, Update CLI parses their semantic versions and selects the highest valid version. The bootstrap:

1. creates `.update-cli/config.json`,
2. resolves the download source,
3. selects the newest matching release,
4. creates the immutable version under `release/<VERSION>/`,
5. synchronizes the release to `current/`,
6. runs `migrate.sh` and project setup only when setup is selected/enabled.

Example resulting layout:

```text
demo-app-DEV/
├── .update-cli/
│   ├── config.json
│   └── history.jsonl
├── release/
│   └── 2.0.0/
└── current/
    └── VERSION
```

A different download folder can be supplied explicitly:

```bash
update-cli init --project demo-app --downloads /srv/releases
```

The downloadable quickstart recording is reproducibly generated from [`docs/tapes/quickstart.tape`](docs/tapes/quickstart.tape). It creates its own local demo ZIP and project tree, so it does not depend on external services:

```bash
vhs docs/tapes/quickstart.tape
# or
just tape-quickstart
```

The recording is written to `docs/videos/quickstart.gif`.

### Bootstrap from a Git repository

The explicit form is:

```bash
update-cli init --project git-cli \
  --from-repository \
  --repository https://github.com/r14r/git-cli
```

The compact form remains supported:

```bash
update-cli init --project git-cli --from-repository https://github.com/r14r/git-cli
update-cli init --project ollama-cli --from-repository r14r/ollama-cli
update-cli init --project ollama-cli --from-repository ollama-cli
```

Repository normalization rules:

```text
https://github.com/r14r/git-cli  -> https://github.com/r14r/git-cli.git
r14r/ollama-cli                 -> https://github.com/r14r/ollama-cli.git
ollama-cli                       -> https://github.com/<source.defaultUser>/ollama-cli.git
```

Absolute local Git repository paths are also accepted. Repository state is maintained below `.update-cli/repository/`; a clean snapshot is deployed to `current/`.

### Normal operation after init

Typical commands are:

```bash
update-cli check
update-cli update --plan
update-cli update
update-cli status
update-cli doctor
```

With the default runtime setting

```json
"no parameter": ["update", "no-setup"]
```

a plain `update-cli` performs an update without opening the setup confirmation prompt.

### Render all README CLI recordings

Install [VHS](https://github.com/charmbracelet/vhs), then run:

```bash
just tapes
# equivalent:
./scripts/render-tapes.sh
```

The `.tape` files are source artifacts; generated GIFs are optional documentation build output.

## Source modes

| `mode` | `source.type` | Required field |
|---|---|---|
| `update` | `download` | `source.folder` |
| `update` | `url` | `source.url` |
| `pull` | `repository` | `source.repository` |

Explicit CLI source flags are one-shot overrides. Persistent values are written to `.update-cli/config.json`. `source.defaultUser` is used only when `--from-repository` receives a repository name without a user.

Examples:

```bash
update-cli config --set mode=pull
update-cli config --set source.type=repository
update-cli config --set source.defaultUser=r1r
update-cli config --set source.repository=https://github.com/acme/demo-app.git
update-cli config --set source.ref=main
```

Switch back to Downloads:

```bash
update-cli config --set mode=update
update-cli config --set source.type=download
update-cli config --set source.folder="$HOME/Downloads"
```

## Global configuration layer

The installation-wide file is resolved from the executable path:

```text
<INSTALLFOLDER>/../etc/update-cli/config.json
```

Example for `/usr/local/bin/update-cli`:

```text
/usr/local/etc/update-cli/config.json
```

The shipped global default focuses on source defaults and the minimum rsync preserve/exclude policy:

```json
{
  "source": {
    "defaultUser": "r1r"
  },
  "sync": {
    "preserve": [
      ".git/",
      ".gitignore",
      ".venv/",
      ".env",
      ".env.*",
      "data/",
      "storage/",
      "uploads/",
      "media/",
      "logs/",
      "var/"
    ]
  },
  "setup": {
    "keepRsyncOnError": false
  }
}
```

The project-local `.update-cli/config.json` is loaded second. Nested objects are merged recursively; local scalar values replace global values. `sync.preserve` is intentionally additive rather than replacing the global list, so centrally required exclusions cannot disappear merely because a project adds its own paths.

### Project-versioned updater settings in `update-cli.yaml`

After global and local JSON are merged, Update CLI reads the preferred project manifest. `./update-cli.yaml` has priority; if it is absent, `./current/update-cli.yaml` can supply the project settings. Values present under `project.slug` and `update:` override the corresponding JSON values. CLI flags such as `--repository`, `--url`, or `--folder` are applied afterwards.

Example:

```yaml
schemaVersion: 2
project:
  name: Demo App
  slug: demo-app

update:
  mode: update
  source:
    type: download
    folder: $HOME/Downloads
  releaseDir: release
  currentDir: current
  backup:
    directory: backup
    keep: 3
  retention:
    releases: 5
  sync:
    preserve:
      - .env
      - .env.*
      - data/
      - uploads/
    keepOnSetupError: true
  docker:
    lifecycle: auto
  healthcheck:
    type: none
```

`update.sync.preserve` is the project-authoritative preserve list when present in YAML; if it is omitted, the merged global/local JSON list is used. `.gitignore` remains protected as an Update CLI invariant. `security.*` is intentionally rejected inside `update:` and must stay in `config.json`.

Templates use the same two-level model with `/usr/local/etc/update-cli/templates.json` and `.update-cli/templates.json`. Templates are merged by case-insensitive name: a local template replaces the global definition with the same name; otherwise it is added.

## Complete updater configuration

A typical `.update-cli/config.json` is:

```json
{
  "schemaVersion": 9,
  "projectName": "demo-app",
  "mode": "update",
  "source": {
    "type": "download",
    "defaultUser": "r1r",
    "folder": "$HOME/Downloads"
  },
  "releaseDir": "release",
  "currentDir": "current",
  "no parameter": ["update", "no-setup"],
  "setup": {
    "commands": [],
    "keepRsyncOnError": false
  },
  "backup": {
    "directory": "backup",
    "keep": 3
  },
  "retention": {
    "releases": 5
  },
  "sync": {
    "preserve": [
      ".git/",
      ".gitignore",
      ".venv/",
      ".env",
      ".env.*",
      "data/",
      "storage/",
      "uploads/",
      "media/",
      "logs/",
      "var/"
    ]
  },
  "security": {
    "allowHttp": false,
    "maxArchiveBytes": 2147483648,
    "maxUncompressedBytes": 8589934592,
    "maxFileBytes": 2147483648,
    "maxEntries": 100000,
    "maxCompressionRatio": 200
  },
  "docker": {
    "lifecycle": "auto"
  },
  "healthcheck": {}
}
```

### `setup.keepRsyncOnError` behavior

`setup.keepRsyncOnError` controls only recovery from a failure in the update's project-setup phase. The default is `false`.

```json
"setup": {
  "keepRsyncOnError": true
}
```

With the option enabled, Update CLI keeps the already verified rsync deployment in `current/` and promotes the matching staged release to `release/<version>/` even though setup failed. The command still exits with an error and records a failed history entry. This is useful when the new source tree should remain available so setup can be diagnosed or rerun separately.

The option does **not** suppress recovery for failures in extraction, rsync, current verification, Docker/service restart, healthcheck, release activation, or metadata handling. Existing `sync.preserve` paths remain protected as usual.

The setting can be changed locally in JSON with:

```bash
update-cli config --set setup.keepRsyncOnError=true
update-cli config --set setup.keepRsyncOnError=false
```

For a project-versioned policy, use `update.sync.keepOnSetupError` in `update-cli.yaml`; that value overrides the JSON fallback. The transitional `update.setup.keepRsyncOnError` form is migrated by `update-cli fix` or `update-cli doctor --migrate`.

### `sync.preserve` behavior

Without a YAML override, the effective `sync.preserve` list is the union of the global list and the project-local JSON list. When `update.sync.preserve` is present in the preferred `update-cli.yaml`, that project list takes precedence. In either case, preserve means **keep the local copy when it already exists**. It is not a permanent source exclusion. During an initial install or when a protected path is missing from `current/`, Update CLI seeds that path once from the selected release. Future updates preserve the local copy.

For example, with:

```json
"sync": {
  "preserve": [".env", ".env.*", "data/"]
}
```

a release containing `.env`, `.env.example`, and `data/` installs those paths when they are absent. If they already exist in `current/`, their local contents are retained. Note that the wildcard `.env.*` also protects `.env.example`; remove or narrow that wildcard if template files should always track the release.

The release snapshot itself contains source `.env` files. Ordinary backup snapshots still omit `.env` and `.env.*` to avoid exporting secrets.

## Configuration commands

Show effective configuration:

```bash
update-cli config
```

List the configuration layers:

```bash
update-cli config --list
```

Typical output for `/usr/local/bin/update-cli`:

```text
Konfigurationsdateien
───────────────────────────────────────
  config.json:        /usr/local/etc/update-cli/config.json
                      /path/to/project/.update-cli/config.json
  templates.json:     /usr/local/etc/update-cli/templates.json
                      /path/to/project/.update-cli/templates.json
  history.jsonl       /path/to/project/.update-cli/history.jsonl
```

Other commands:

```bash
update-cli config --list
update-cli config --edit
update-cli config --check
update-cli install
update-cli config --migrate
update-cli config --set KEY=VALUE
```

Examples:

```bash
update-cli config --set backup.keep=5
update-cli config --set retention.releases=10
update-cli config --set docker.lifecycle=disabled
update-cli config --set no-parameter=update,no-setup
```

`config --set` writes transactionally: all requested changes are validated before the JSON file replaces the previous configuration.

### Legacy config migration

For an older project:

```text
.update-cli/config.json
```

run:

```bash
update-cli config --check
update-cli config --migrate
```

Migration creates a timestamped backup and moves the active file to:

```text
.update-cli/config.json
```

The remaining `.update-cli/` directory is not required to be deleted automatically because it may still contain historical runtime data.

## JSON Schema for `update-cli.yaml`

Show the canonical schema version:

```bash
update-cli schema --version
# 2
```

Standalone `update-cli version` reports the application release version.

Show the canonical schema on stdout:

```bash
update-cli schema --view
```

Save it to a file:

```bash
update-cli schema --save update-cli.schema.json
```

Nested output paths are supported:

```bash
update-cli schema --save schemas/update-cli.schema.json
```

`--view` and `--save` produce the same JSON Schema. `--version` reports the schema version represented by that canonical schema. The schema describes the canonical `update-cli.yaml` **schemaVersion 2** automation format, including project metadata, variables, requirements, workflows, tasks, run configuration, conditions, and typed step operations.

Project-dependent update source settings may be stored under `update:` in `update-cli.yaml` and override their global/local `config.json` fallback values. Host/user-only values such as `source.defaultUser` and `security.*` remain outside the project-controlled manifest.

## Pre-setup migrations

When a release contains `migrate.sh`, Update CLI checks its version marker immediately before any setup execution:

```bash
update-cli setup
```

For version `1.4.0`, a successful run creates:

```text
<project-root>/.update-cli/.migration.done.1.4.0
```

A subsequent `update-cli setup` sees that marker and goes directly to setup. If `migrate.sh` exits non-zero, setup is not started and the marker is not created.

The migration script runs from the active release directory and can use:

```text
UPDATE_CLI_MIGRATION=1
UPDATE_CLI_PROJECT_ROOT=<stable project root>
UPDATE_CLI_CURRENT_DIR=<active current/source directory>
UPDATE_CLI_VERSION=<installed semantic version>
```

The script must have a valid `VERSION` file beside it because the semantic version is part of the persistent completion marker.

## Setup manifests

Preferred filename:

```text
update-cli.yaml
```

Legacy fallback:

```text
setup.yaml
```

Discovery order is always:

```text
1. update-cli.yaml
2. setup.yaml
3. setup.sh
4. just build + just install fallback
```

The modern schemaVersion-2 project manifest can contain project update overrides plus setup/run automation. Example:

```yaml
schemaVersion: 2

project:
  name: Demo Application
  slug: demo-app

run:
  command: .venv/bin/streamlit run app/app.py

workflows:
  setup:
    tasks: [deploy]

tasks:
  build:
    steps:
      - shell: go build ./...

  deploy:
    requires: [build]
    steps:
      - deploy:
          source: ./demo-app
          target: /usr/local/bin/demo-app
          mode: "0755"
```

Project-specific update source settings may be placed under `update.source`; global/host defaults remain in `config.json` and are overridden by the project manifest.

### Legacy `setup.yaml`

Existing projects do not have to rename immediately. `setup.yaml` is accepted by:

```bash
update-cli setup
update-cli setup
update-cli run
```

When `setup.yaml` has no explicit `schemaVersion`/legacy `version`, Update CLI infers:

- schema 2 for structures containing `tasks`, `workflows`, `run`, `defaults`, `variables`, or `requirements`;
- schema 1 otherwise.

`update-cli.yaml` always wins if both files are present.

### Just fallback

If an explicit setup finds no `update-cli.yaml`, `setup.yaml`, or `setup.sh`, but finds `justfile` or `Justfile`, Update CLI executes:

```bash
just build
just install
```

If `just` itself is missing, setup returns an explicit error instead of silently skipping the project build.

## Setup commands

Run the default setup workflow:

```bash
update-cli setup
```

Inspect every setup workflow, task, and step. Steps with an `id` can be addressed individually:

```bash
update-cli setup --list
update-cli setup --list --json
```

Run exactly one step by its manifest `id`:

```bash
update-cli setup --run STEP_ID
update-cli setup --run STEP_ID --details
```

Run a complete task or workflow:

```bash
update-cli setup --task NAME
update-cli setup --workflow NAME
```

Command-first compatibility forms are also accepted:

```bash
update-cli setup --list
update-cli setup --run STEP_ID
```

`setup --run` deliberately runs only the selected step. It does not execute the task's `requires` dependencies or other steps in the same task. The selected step's own condition and execution policy remain active. IDs should be unique across setup tasks; an ambiguous duplicate ID is rejected.

### Detailed command help

The standard help remains concise:

```bash
update-cli help
update-cli help --command setup
```

Add `--details` to get a command-specific description, relevant options, and examples:

```bash
update-cli help --command setup --details
update-cli help --command setup --details
update-cli help --command update --details
```

Use `update-cli help --details` for a detailed overview of all commands.

## Run application

Compact form in either supported manifest filename:

```yaml
run:
  command: .venv/bin/streamlit run app/app.py
```

Structured form:

```yaml
run:
  description: Start Streamlit app
  steps:
    - name: Start Streamlit
      command:
        exec: .venv/bin/streamlit
        args:
          - run
          - app/app.py
```

Execute:

```bash
update-cli run
# compatibility alias
update-cli run
```

## Command syntax and legacy aliases

Commands use a command-first syntax. Options still use `--`:

```text
update-cli check --no-ask
update-cli update --archive app-v1.2.3.zip --backup --setup
update-cli releases --list --json
update-cli doctor --migrate
update-cli config --check
```

Legacy command flags and bare secondary subcommands are rejected. Use only `update-cli <command> --<parameter> ...`.

## ZIP update mode

With:

```json
"mode": "update",
"source": {
  "type": "download",
  "folder": "$HOME/Downloads"
}
```

Update CLI discovers versioned ZIP releases and installs them through the transactional release/current pipeline.

## Git pull mode

With:

```json
"mode": "pull",
"source": {
  "type": "repository",
  "repository": "https://github.com/acme/demo-app.git",
  "ref": "main"
}
```

Update CLI maintains a persistent checkout below the active configuration directory:

```text
.update-cli/repository/
```

The deployed `current/` tree is never the Git working tree. A `.release-commit` marker records the deployed commit so new commits can be detected even if `VERSION` is unchanged.

## Project repair

When `config --check`, `upgrade`, update operations or setup commands fail because project metadata contains legacy/unknown fields, repair the project first:

```bash
update-cli fix
```

`fix` is intentionally more tolerant than the normal loaders. It parses legacy structure, removes unsupported fields, migrates known values to the current schema, writes backups, then verifies the result with the regular strict loaders. Use `--root <folder>` for another project directory and `--json` for machine-readable repair details.

Typical recovery for the historical `defaultUser` error is simply:

```bash
cd /path/to/project
update-cli fix
update-cli upgrade
```

## Project manifest doctor

Run doctor from the project root that contains `update-cli.yaml`:

```bash
cd /path/to/project
update-cli doctor
```

`doctor` determines the project layout and checks all relevant configuration files. From the project root it checks:

```text
./.update-cli/config.json
./update-cli.yaml
./current/update-cli.yaml
```

If the command is started inside `current/`, the parent is recognized as the project root and `../.update-cli/config.json` is used. A runtime config is optional for a pure source project, but when present it is schema-checked together with both manifests. Doctor also compares the root/current `update:` sections and reports which manifest supplies the effective project overrides.

To migrate an older manifest to the newest supported schema:

```bash
update-cli doctor --migrate
```

For an existing `update-cli.yaml`, migration creates a timestamped backup before replacing the file. If a checked location only contains legacy `setup.yaml`, it remains untouched and a new canonical `update-cli.yaml` is created. `doctor --migrate` checks/migrates the project runtime config plus both root/current manifest locations when they exist. Structural errors that cannot be safely migrated are reported with a recommendation to run `update-cli fix`.

`update-cli config --migrate` remains available when only the runtime JSON should be migrated.

`--root <folder>` can be used to check another project folder explicitly. `--json` returns the same doctor result as structured JSON.

## Recovery and safety

Useful commands:

```bash
update-cli backup
update-cli rollback --version VERSION
update-cli restore --snapshot latest
update-cli status
update-cli history
update-cli doctor
update-cli doctor --migrate
update-cli clean
update-cli cleanup
```

Mutating operations prepare transactional recovery. A valid previous `release/<VERSION>/` is used as the fast rollback basis; an exact `current/` snapshot is created only as fallback when no usable previous release exists. Activation/setup/health failures restore the previous release while preserving configured persistent paths.

Docker lifecycle values in JSON config:

```text
auto
disabled
required
```

## macOS path compatibility

macOS can expose the same temporary directory as both:

```text
/var/folders/...
/private/var/folders/...
```

Root-discovery and install-command tests canonicalize symlinks before comparison so this alias does not produce false failures.

The integration suite deliberately executes setup/update/migration workflows multiple times with different fixtures. These are separate test cases, not repeated execution of one production update. On a passing package, standard `go test` output does not print those scenario logs.

## Development

Required release toolchain:

```text
Go 1.26.5
```

Checks:

```bash
just fmt-check
just vet
just test
just test-race
just check
```

Build/install:

```bash
just build
just install
```

Cross-builds:

```bash
just build-all
```

## CLI discovery

```bash
update-cli help
update-cli howto
update-cli help --json
```

## Repository

Source: https://github.com/r14r/update-cli

GitHub Pages: https://r14r.github.io/update-cli/
