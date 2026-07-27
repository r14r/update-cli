---
layout: page
title: Requirements
---

# Requirements

## Runtime Requirements

### Required

- **Operating System**: macOS or Linux (native Windows support not available)
- **rsync**: File synchronization (usually pre-installed on macOS/Linux)
- **bash**: Shell for `setup.sh` and configured setup commands

### Conditional

- **git**: Only required if using `--from repository` source
- **docker compose** or **docker-compose**: Only needed if `current/` contains Compose files

### Verification

Check your environment:

```bash
go version      # (required for building)
rsync --version
bash --version
git --version
docker compose version
```

## Build Requirements

- **Go 1.22+**: Latest version recommended
- **just** (optional): Task runner for development commands

## Platform Support

### Supported

- ✅ **macOS** (Intel and Apple Silicon)
- ✅ **Linux** (x86_64)

### Not Supported

- ❌ **Windows** (native support not available; WSL2 possible)
- ❌ **BSD** (not tested)
- ❌ **Other Unix variants** (not officially supported)

### Why Not Windows?

update-cli relies on:
- `rsync` for atomic file synchronization
- `bash` for setup script execution
- POSIX shell semantics throughout

These are not natively available on Windows, making Windows support difficult. Consider:
- **WSL2** (Windows Subsystem for Linux 2) - Should work
- **Docker** - Run update-cli in container
- **Remote SSH** - Execute on Linux/macOS server

## System Resources

### Disk Space

- **Binary**: ~20-30 MB
- **Working Directory**: Depends on archive sizes
  - Temporary extraction space needed during update
  - Versioned releases stored under `release/`
  - Backups stored under `backup/`
- **Recommendation**: At least 2-3x your largest archive size available

### Memory

- Minimal requirements (< 50 MB)
- Depends more on archive extraction performance

### Network

- For `--from url`: Standard internet connectivity
- For `--from repository`: Git protocol support (HTTP/HTTPS or SSH)

## Common Environment Setups

### Development Machine

```bash
# macOS
brew install go rsync git docker
brew install just  # optional

# Linux (Ubuntu/Debian)
sudo apt-get install golang-go rsync git docker.io
sudo apt-get install just  # optional
```

### CI/CD Pipeline

```bash
# GitHub Actions example
- uses: actions/setup-go@v4
  with:
    go-version: '1.22'

# rsync and bash typically pre-installed
```

### Production Deployment

```bash
# Minimal requirements
# - rsync installed
# - bash available
# - git (if using repository source)
# - docker (only if using Docker Compose)
```

## Troubleshooting

### rsync not found

```bash
# macOS
brew install rsync

# Ubuntu/Debian
sudo apt-get install rsync

# CentOS/RHEL
sudo yum install rsync
```

### bash not found

Unlikely on modern systems, but if needed:

```bash
# Most systems have bash in /bin/bash
# Verify:
which bash

# macOS
brew install bash

# Linux should have bash in base installation
```

### docker compose fails

If you have old Docker with separate `docker-compose`:

```bash
# Check what's available
docker compose version  # or
docker-compose version

# Both formats are supported by update-cli
```

### Go version too old

```bash
# Check current version
go version

# Update Go
# https://golang.org/doc/install
```

Next: [Installation →](/getting-started/installation)
