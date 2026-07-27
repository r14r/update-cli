---
layout: page
title: Project Configuration
---

# Project Configuration

## Overview

Each project managed by update-cli has a configuration file:

```
.updater-cli/config.json
```

This file is created with `--init` and can be edited with `--config --edit`.

## Current Schema

```json
{
  "schemaVersion": 5,
  "projectName": "mediastudio",
  "source": {
    "type": "download",
    "folder": "$HOME/Downloads"
  },
  "releaseDir": "release",
  "currentDir": "current",
  "no parameter": [
    "help"
  ],
  "setup": {
    "commands": []
  },
  "backup": {
    "directory": "backup",
    "keep": 3
  },
  "retention": {
    "releases": 5
  }
}
```

## Fields Reference

### Core Fields

| Field | Type | Required | Example |
|-------|------|----------|----------|
| `schemaVersion` | number | Yes | `5` |
| `projectName` | string | Yes | `"mediastudio"` |
| `releaseDir` | string | No | `"release"` |
| `currentDir` | string | No | `"current"` |

### schemaVersion

Version of configuration schema. Used by `--upgrade` to migrate old formats.

Current: `5`

Supported: Versions 1-5 (older versions auto-migrated)

### projectName

Required. Used as prefix for archive filenames:

```
{projectName}-v{VERSION}.zip
```

Example values:
```
"mediastudio"
"release-updater-go"
"myapp-backend"
```

### source

Release source configuration. Three types:

#### Local Folder

```json
"source": {
  "type": "download",
  "folder": "$HOME/Downloads"
}
```

update-cli selects the latest SemVer release from this folder.

Supports environment variables: `$HOME`, `$USER`, etc.

#### Direct URL

```json
"source": {
  "type": "url",
  "url": "https://releases.example.test/mediastudio-v3.25.0.zip"
}
```

Downloads from specific URL. Version extracted from URL and validated against archive.

#### Git Repository

```json
"source": {
  "type": "repository",
  "repository": "https://github.com/example/mediastudio.git"
}
```

Clones repository shallowly. Version read from `VERSION` file in repository root.

### releaseDir

Directory where versioned releases are stored:

```
release/
├── 3.24.0/
├── 3.25.0/
└── .last-version
```

Default: `"release"`

### currentDir

Active deployment directory. Contains current version and is synced from `releaseDir`.

Default: `"current"`

Protected paths within `currentDir`:
- `.git/` - Never deleted
- `.venv/` - Never deleted  
- `.env` - Never deleted

### no parameter

Actions executed when update-cli runs without arguments.

Allowed values:
```json
["help"]                    // Show help (default)
["update"]                  // Run update
["setup"]                   // Run setup
["update", "setup"]        // Update then setup
```

`"help"` cannot be combined with other actions.

Examples:

```json
// Default - show help
"no parameter": ["help"]

// Auto-update on every run
"no parameter": ["update"]

// Update and setup
"no parameter": ["update", "setup"]
```

### setup

Setup commands to run after update.

```json
"setup": {
  "commands": [
    "composer install",
    "npm install",
    "just assets-build"
  ]
}
```

Execution:
1. First: `current/setup.sh` (if exists)
2. Then: Each command in `setup.commands`
3. Working directory: `currentDir`
4. Stops on first error

### backup

Backup configuration.

```json
"backup": {
  "directory": "backup",
  "keep": 3
}
```

- `directory`: Where backups are stored
- `keep`: How many backups to retain (oldest deleted automatically)

### retention

Release retention configuration.

```json
"retention": {
  "releases": 5
}
```

- `releases`: How many old releases to keep before cleanup

## Initialization

### Basic

```bash
update-cli --init myproject
```

Creates:
```
.updater-cli/config.json
.updater-cli/templates.json
.updater-cli/history.jsonl
```

### With Source

```bash
update-cli --init myproject --from url \
  --url https://releases.example.com/myproject-v1.0.0.zip
```

### With Template

```bash
update-cli --init myproject --use-template Laravel
```

## Editing Configuration

### View Current Config

```bash
update-cli --config
```

Outputs prettified JSON.

### Edit in Editor

```bash
update-cli --config --edit
```

Editor selection (in order):
1. `VISUAL` environment variable
2. `EDITOR` environment variable
3. `code --wait` (VS Code)
4. `cursor --wait` (Cursor)
5. `nano`
6. `vim`
7. `vi`

Example:

```bash
EDITOR="nano" update-cli --config --edit
```

## Migration

### Upgrading Configuration

```bash
update-cli --upgrade
```

The upgrade process:
1. Reads current schema (supports v1-5)
2. Preserves all project-specific values
3. Adds new default fields
4. Backs up old config
5. Writes new config
6. Validates result

Old config backed up as:
```
.updater-cli/config.v{old-version}.backup.json
```

### JSON Output

```bash
update-cli --upgrade --json
```

Machine-readable migration details.

## Environment Variables

Supported in string fields:

```json
{
  "source": {
    "folder": "$HOME/releases"  // ✅ Supported
  }
}
```

Expanded at runtime:
```
/Users/username/releases
/home/username/releases
```

Common variables:
- `$HOME` - User home directory
- `$USER` - Username
- `$PWD` - Current directory
- `$TMPDIR` - Temporary directory

## Examples

### Minimal Local Setup

```json
{
  "schemaVersion": 5,
  "projectName": "myapp",
  "source": {
    "type": "download",
    "folder": "$HOME/Downloads"
  },
  "releaseDir": "release",
  "currentDir": "current",
  "no parameter": ["help"],
  "setup": {"commands": []},
  "backup": {"directory": "backup", "keep": 3},
  "retention": {"releases": 5}
}
```

### Production Docker Setup

```json
{
  "schemaVersion": 5,
  "projectName": "api-service",
  "source": {
    "type": "url",
    "url": "https://releases.internal.company.com/api-service-latest.zip"
  },
  "releaseDir": "/srv/api-service/releases",
  "currentDir": "/srv/api-service/current",
  "no parameter": ["update", "setup"],
  "setup": {
    "commands": [
      "docker compose pull",
      "docker compose up -d"
    ]
  },
  "backup": {"directory": "/srv/api-service/backups", "keep": 5},
  "retention": {"releases": 10}
}
```

Next: [Archive Format →](/configuration/archive-format)
