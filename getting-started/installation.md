---
layout: page
title: Installation
---

# Installation

## Quick Start

### Using `just`

If you have `just` installed:

```bash
just build
```

This creates:
```
dist/update-cli
```

### Using Go Directly

```bash
go build -trimpath -o update-cli .
```

Verify the build:

```bash
./update-cli --version
```

## Installation Methods

### 1. Local Project Installation

```bash
just install
```

Installs `update-cli` into your project directory.

### 2. System-Wide Installation

```bash
just deploy
```

Installs to system location (default: `/usr/local/bin/update-cli`).

The installation target is read from `build-config.json`:

```json
{
  "schemaVersion": 1,
  "defaultDownloadFolder": "$HOME/Downloads",
  "defaultDeploymentPath": "/usr/local/bin",
  "defaultConfigPath": "/usr/local/etc/update-cli"
}
```

### 3. Platform-Specific Builds

Build for specific platforms:

```bash
just build-macos-amd64    # Intel Mac
just build-macos-arm64    # Apple Silicon
just build-linux-amd64    # Linux
just build-all            # All platforms
```

Results:
```
dist/update-cli-darwin-amd64
dist/update-cli-darwin-arm64
dist/update-cli-linux-amd64
```

## Build Configuration

### Customizing Defaults

Edit `build-config.json` before building:

```json
{
  "schemaVersion": 1,
  "defaultDownloadFolder": "$HOME/Downloads",
  "defaultDeploymentPath": "/usr/local/bin",
  "defaultConfigPath": "/usr/local/etc/update-cli"
}
```

### Example: Custom Distribution

```json
{
  "schemaVersion": 1,
  "defaultDownloadFolder": "$HOME/Downloads",
  "defaultDeploymentPath": "/Users/Shared/CLOUD/DeveloperTools/bin",
  "defaultConfigPath": "/Users/Shared/CLOUD/DeveloperTools/etc/update-cli"
}
```

**Note**: Rebuild after changing `build-config.json`

## Global Templates

### Setup Global Templates

For system-wide template availability:

```bash
sudo mkdir -p /usr/local/etc/update-cli
sudo cp docs/examples/global-templates.json \
  /usr/local/etc/update-cli/templates.json
```

update-cli will search for:
```
<defaultConfigPath>/templates.json
```

### Template Priority

1. Built-in templates (in binary)
2. Global templates (`/usr/local/etc/update-cli/templates.json`)
3. Local templates (`.updater-cli/templates.json`)

Local templates take precedence and are preserved during `--upgrade`.

## Verification

### Check Installation

```bash
update-cli --version
# Output: update-cli v2.4.1

update-cli --help
# Show command overview

update-cli --doctor
# Validate environment setup
```

### Verify Binary Location

```bash
which update-cli
# /usr/local/bin/update-cli
```

## Troubleshooting

### Go Build Fails

Ensure Go 1.22 or newer:

```bash
go version
```

Update Go: https://golang.org/doc/install

### rsync Not Found During Build

runtime dependency, not build dependency. Install rsync:

```bash
# macOS
brew install rsync

# Ubuntu/Debian
sudo apt-get install rsync
```

### Permission Denied During Deploy

If `/usr/local/bin` is not writable:

```bash
# Use sudo
sudo just deploy

# Or deploy to user-writable location
# Edit build-config.json first
```

### Binary Not in PATH

Verify installation location is in PATH:

```bash
echo $PATH

# If not present, add to shell profile:
echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.bash_profile
source ~/.bash_profile
```

Next: [Quickstart →](/usage/quickstart)
