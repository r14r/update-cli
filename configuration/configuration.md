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

## Download Folder example

```json
{
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
  setup/build/deploy tasks and application run command
```
