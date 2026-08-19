---
layout: page
title: Installation
---

# Installation

Current release: **1.2.1**

## Default installation locations

```text
Binary             /usr/local/bin/update-cli
Global config      /usr/local/etc/update-cli
Download folder    $HOME/Downloads
```

## Build from source

```bash
git clone https://github.com/r14r/update-cli.git
cd update-cli
just build
```

Without `just`:

```bash
go vet ./...
go test ./...
go build -trimpath \
  -ldflags "-s -w -X main.version=$(cat VERSION)" \
  -o dist/update-cli .
```

## Install the local build

```bash
just install
```

`just install` runs the validated build first, then installs the binary and global support files using `build-config.json`.

> The old recipe name `just deploy` was removed. Use `just install`.

## Verify

```bash
update-cli --version
update-cli --help
```
