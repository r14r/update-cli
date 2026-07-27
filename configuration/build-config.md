---
layout: page
title: Build Configuration
---

# Build Configuration

## Overview

Distribution-specific defaults are configured in `build-config.json`. This file is embedded in the binary during build, making it impossible to modify after compilation.

## File Location

```
build-config.json
```

## Schema

```json
{
  "schemaVersion": 1,
  "defaultDownloadFolder": "$HOME/Downloads",
  "defaultDeploymentPath": "/usr/local/bin",
  "defaultConfigPath": "/usr/local/etc/update-cli"
}
```

## Fields

| Field | Purpose | Default | Environment Variable Support |
|-------|---------|---------|------------------------------|
| `schemaVersion` | Configuration schema version | `1` | No |
| `defaultDownloadFolder` | Standard folder for local ZIP releases | `$HOME/Downloads` | Yes (`$HOME` expanded) |
| `defaultDeploymentPath` | Target directory for `just deploy` | `/usr/local/bin` | No |
| `defaultConfigPath` | System-wide configuration directory | `/usr/local/etc/update-cli` | No |

## Field Descriptions

### defaultDownloadFolder

Used when:
- `--init` creates new project configuration
- No `--from` source specified
- Default release source is `download`

Supports `$HOME` expansion:
```json
{
  "defaultDownloadFolder": "$HOME/Downloads"
}
```

Becomes:
```
/Users/username/Downloads  (on macOS)
/home/username/Downloads   (on Linux)
```

### defaultDeploymentPath

Target location for:
```bash
just deploy
```

Must be:
- Writable by deployment user
- In system PATH (for shell command availability)
- Typically `/usr/local/bin` on Unix systems

### defaultConfigPath

Directory for global configuration:
- System-wide templates (`templates.json`)
- Shared project setups
- Global defaults

Checked by:
- `--config --list` (reports location)
- `--doctor` (validates if exists)
- `--init` (offers as fallback)

## Customization

### Before Build

Edit `build-config.json` **before** any of:
- `just build`
- `just build-all`
- `just install`
- `just deploy`
- `go build ...`

### Example: Custom Setup

Organization-specific deployment:

```json
{
  "schemaVersion": 1,
  "defaultDownloadFolder": "/mnt/releases",
  "defaultDeploymentPath": "/usr/local/bin",
  "defaultConfigPath": "/etc/update-cli"
}
```

### Example: Development Distribution

Custom user directories:

```json
{
  "schemaVersion": 1,
  "defaultDownloadFolder": "$HOME/projects/releases",
  "defaultDeploymentPath": "$HOME/.local/bin",
  "defaultConfigPath": "$HOME/.config/update-cli"
}
```

## Verification

### Check Current Configuration

```bash
just build-config
```

This shows what will be embedded in the next build.

### Display Configuration

```bash
just build-config-show
```

Pretty-prints the current `build-config.json`.

### After Build

View embedded configuration:

```bash
update-cli --doctor
```

Reports where update-cli will look for configurations.

## Build Process

### Step 1: Edit Configuration

```bash
cat build-config.json
```

### Step 2: Build Binary

```bash
go build -trimpath -o update-cli .
```

The build process:
1. Reads `build-config.json`
2. Embeds contents in binary
3. Cannot be modified afterward

### Step 3: Verify

```bash
./update-cli --doctor
```

## Global Templates Path

update-cli looks for optional global templates at:

```
<defaultConfigPath>/templates.json
```

Example with default:
```
/usr/local/etc/update-cli/templates.json
```

Setup:

```bash
sudo mkdir -p /usr/local/etc/update-cli
sudo cp docs/examples/global-templates.json \
  /usr/local/etc/update-cli/templates.json
```

## Migration Between Distributions

If deploying to new system or environment:

1. Adjust `build-config.json` for new defaults
2. Rebuild binary: `just build`
3. Reinstall: `just deploy`
4. Old project configurations continue working (`.updater-cli/config.json` is independent)

## Platform-Specific Configurations

### macOS

```json
{
  "schemaVersion": 1,
  "defaultDownloadFolder": "$HOME/Downloads",
  "defaultDeploymentPath": "/usr/local/bin",
  "defaultConfigPath": "/usr/local/etc/update-cli"
}
```

### Linux (Ubuntu/Debian)

```json
{
  "schemaVersion": 1,
  "defaultDownloadFolder": "$HOME/Downloads",
  "defaultDeploymentPath": "/usr/local/bin",
  "defaultConfigPath": "/etc/update-cli"
}
```

### Docker Container

```json
{
  "schemaVersion": 1,
  "defaultDownloadFolder": "/app/releases",
  "defaultDeploymentPath": "/usr/local/bin",
  "defaultConfigPath": "/app/etc/update-cli"
}
```

Next: [Project Configuration →](/configuration/configuration)
