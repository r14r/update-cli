---
layout: page
title: Requirements
---

# Requirements

## Supported environment

`update-cli` is built for macOS and Linux. Release builds include macOS Intel, macOS Apple Silicon and Linux amd64 binaries.

## Base requirements

- a POSIX-style filesystem and shell environment;
- write access to the managed project directories;
- `bash` where available (the runner can fall back to `sh` for application commands);
- standard filesystem utilities used by the platform.

## Git pull mode

Git is required when the project uses `mode: pull`:

```bash
git --version
```

For private repositories, configure authentication in the normal system Git client (SSH keys, credential helper, token-backed HTTPS credentials, etc.). `update-cli` does not implement a separate GitHub credential store.

## Docker lifecycle

Docker is only required when Docker lifecycle management is needed. `docker.lifecycle` can be `auto`, `disabled` or `required`.

## Building update-cli itself

Building from source requires Go and optionally `just`:

```bash
go version
just --version
```
