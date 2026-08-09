# Migration to Update CLI 3.x

Update CLI 3.0 changes the project configuration schema from version 5 to version 6 and introduces transactional deployment semantics.

## Upgrade the project configuration

Run:

```bash
update-cli --upgrade
```

The command creates a timestamped backup of the existing `.updater-cli/config.json`, migrates it to schema 6 and validates the result before activation.

## New configuration sections

Schema 6 adds:

- `sync.preserve` for project-local mutable paths that must survive release synchronization.
- `security.allowHttp` and `security.maxDownloadBytes` for remote-source policy.
- `healthcheck` for post-deployment verification.
- optional repository `ref`, `commit` and `version` constraints.
- optional source `sha256` verification.

Default preserved paths include `.git/`, `.venv/`, `.env`, `.env.*`, `data/`, `storage/`, `uploads/`, `media/`, `logs/` and `var/`.

## Setup compatibility

Update CLI 3.0.7 restores and extends compatibility with the established 2.14 `setup.yaml` contract. Existing manifests that start with `schemaVersion: 1` and use `project.name`, `project.description`, optional `project.type`, and step fields `id`, `when`, `run`, `cwd`, and `allowFailure` can remain unchanged. `project.type` is descriptive metadata and does not select a setup handler.

The typed `version: 1` handler syntax introduced in 3.x remains available as an extension. Both formats are documented in `doc/setup-schema.md`. Legacy `setup.sh` and `config.setup.commands` are still supported as fallbacks.

The interactive fullscreen TUI is also restored for check, update, and setup. Use `--no-ui` for direct stdout/stderr without TUI rendering, `UPDATE_CLI_TUI=plain` for the older plain renderer, or `--no-color` for uncolored output. `--no-wait` skips the default fullscreen Enter-at-completion wait.

## Update behavior

Every mutating update, rollback and restore now creates an exact temporary transaction snapshot. If synchronization, setup, Docker startup or the health check fails, Update CLI restores the previous `current` tree and the previous Docker Compose running state.

Persistent `--backup` snapshots remain separate from transaction snapshots. Logical backups omit `.env` and `.env.*` as well as regenerated dependency directories.

## Docker Compose

If a Compose project is running before a transaction, Update CLI stops it before activation and starts it again after successful setup. On failure the prior files and prior running state are restored.

## Source behavior

`--check`, `--status` and `--list` perform metadata discovery rather than downloading the complete release when possible. `--rollback` and `--cleanup` are local-only operations and no longer require the configured remote source to be reachable.

HTTP release URLs are rejected by default. Set `security.allowHttp` only for explicitly trusted local/test environments.

## Setup schema 2 (Update CLI 3.1)

No migration is required for existing schema-1 setup manifests. Update CLI 3.1 adds schemaVersion 2 for projects that want reusable project automation rather than a single linear setup list.

Typical migration:

```yaml
schemaVersion: 2
workflows:
  setup:
    tasks: [deploy]
  ci:
    tasks: [verify]
tasks:
  check:
    steps:
      - go:
          action: test
  build:
    requires: [check]
    steps:
      - shell: go build ./...
  verify:
    requires: [build]
    steps:
      - assert:
          fileExists: dist/app
  deploy:
    requires: [verify]
    steps:
      - deploy:
          source: dist/app
          target: /usr/local/bin/app
```

New CLI entry points:

```bash
update-cli --setup-list
update-cli --setup-task build
update-cli --setup-workflow ci
```

The existing `update-cli --setup` command executes the `setup` workflow. See `doc/setup-schema.md` for the complete schema.

## Recovery when the globally installed CLI is still 3.0.x

If `update-cli --version` still reports 3.0.x after the newer release has been copied into `current/`, run from the new `current/` directory:

```bash
./setup-template.sh --no-ui
```

The template detects `schemaVersion: 2` and will not pass it to an incompatible global binary. It prefers the matching packaged `dist/update-cli-<os>-<arch>` binary and can bootstrap from the Go source tree as a final fallback.

## setup.yaml automatisch auf schemaVersion 2 migrieren

Ab 3.2.0 kann ein vorhandenes schemaVersion-1-Manifest automatisch migriert werden:

```bash
update-cli --convert-yaml
```

Vor dem Ersetzen wird ein timestamped `setup.yaml.schema1-YYYYMMDD-HHMMSS.bak` angelegt. Mit `--dry-run` kann das Ergebnis vorher geprüft werden. Für Projekte ohne Manifest kann `update-cli --create-yaml` ein schemaVersion-2-Beispiel aus den vorhandenen Projektdateien erzeugen.

