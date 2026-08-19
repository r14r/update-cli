---
layout: page
title: Project Configuration
---

# Project configuration: `.updater-cli/config.json`

Project acquisition and updater policy are stored in `.updater-cli/config.json`.

## Show and edit configuration

```bash
update-cli config
update-cli config list
update-cli config edit
```

Update values from the command line:

```bash
update-cli config --set KEY=VALUE
```

`--set` is repeatable and changes are validated before the file is written atomically.

## Validate configuration

Version 1.3.0 adds a read-only configuration check:

```bash
update-cli config --check
```

Equivalent command-token form:

```bash
update-cli config check
```

The check:

- parses the JSON strictly;
- validates the complete configuration;
- determines the stored schema version;
- verifies that migration to the current schema is possible;
- reports whether migration is needed;
- does **not** modify `config.json`.

For scripting:

```bash
update-cli config --check --json
```

Example for an older schema:

```text
Schema              6
Aktuelles Schema    7
Modus               pull
Quelle              repository
WARN  Konfiguration ist gültig, aber eine Migration ist verfügbar: update-cli config --migrate
```

## Migrate configuration

Migrate the project configuration to the current schema with:

```bash
update-cli config --migrate
```

or:

```bash
update-cli config migrate
```

Migration uses the same safe migration engine as the historical top-level `update-cli upgrade` command. If a change is required, the old configuration is backed up before the new file is written.

Example:

```text
Schema              6 → 7
Backup              .updater-cli/config.json.backup-v6-20260819-150333
OK    Konfiguration wurde migriert
```

Running `config --migrate` against an already-current schema is safe and results in a no-op report.

Recommended maintenance workflow:

```bash
update-cli config --check
update-cli config --migrate
update-cli config --check
```

## Download Folder example

```json
{
  "schemaVersion": 7,
  "mode": "update",
  "source": {
    "type": "download",
    "folder": "$HOME/Downloads"
  }
}
```

Configure it:

```bash
update-cli config \
  --set mode=update \
  --set source.type=download \
  --set source.folder="$HOME/Downloads"
```

## Git repository example

```json
{
  "schemaVersion": 7,
  "mode": "pull",
  "source": {
    "type": "repository",
    "repository": "https://github.com/acme/demo-app.git",
    "ref": "main"
  }
}
```

Configure it:

```bash
update-cli config \
  --set mode=pull \
  --set source.type=repository \
  --set source.repository=https://github.com/acme/demo-app.git \
  --set source.ref=main
```

## Common dotted keys

```text
projectName
mode
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

Lists may be comma-separated or supplied as JSON arrays. Booleans and numbers retain their JSON types.

## Docker lifecycle

```json
{
  "docker": {
    "lifecycle": "auto"
  }
}
```

Values:

- `auto` — use Compose lifecycle when available; otherwise warn and continue where safe;
- `disabled` — never manage Docker lifecycle;
- `required` — Docker lifecycle failures abort an update when a Compose file exists.

## Config vs. `update-cli.yaml`

Keep the responsibilities separate:

```text
.updater-cli/config.json
  source, mode, retention, security, health checks, Docker policy

update-cli.yaml
  setup/build/deploy tasks and application run definition
```

The Git repository URL and branch/ref belong in `.updater-cli/config.json`; application start commands belong in `update-cli.yaml`.
