---
layout: page
title: Quickstart
---

# Quickstart

Choose one acquisition model and keep it explicit.

## Option A — Download Folder (`mode: update`)

### 1. Create and initialize the project

```bash
mkdir DigitalProductsPlatform
cd DigitalProductsPlatform

update-cli init DigitalProductsPlatform \
  --mode update \
  --folder "$HOME/Downloads"
```

Equivalent source configuration:

```json
{
  "mode": "update",
  "source": {
    "type": "download",
    "folder": "$HOME/Downloads"
  }
}
```

### 2. Add a versioned release ZIP

```text
$HOME/Downloads/DigitalProductsPlatform-v4.5.0.zip
```

The archive should contain a `VERSION` file matching the release version.

### 3. Check, plan and update

```bash
update-cli check
update-cli update --plan
update-cli update
update-cli status
```

To install a specific ZIP directly:

```bash
update-cli update \
  "$HOME/Downloads/DigitalProductsPlatform-v4.5.0.zip"
```

## Option B — GitHub Repository (`mode: pull`)

### 1. Initialize

```bash
mkdir DigitalProductsPlatform
cd DigitalProductsPlatform

update-cli init DigitalProductsPlatform \
  --mode pull \
  --repository https://github.com/acme/DigitalProductsPlatform.git
```

### 2. Select a branch/ref

```bash
update-cli config --set source.ref=main
update-cli config list
```

### 3. Check, plan and pull

```bash
update-cli check
update-cli update --plan
update-cli update
update-cli status
```

The repository is cached under `.updater-cli/repository/`; the active `current/` directory is not a Git checkout.

## Project automation

Source configuration belongs in `.updater-cli/config.json`. Build/setup/run behavior belongs in `current/update-cli.yaml`.

Minimal example:

```yaml
schemaVersion: 2

project:
  name: DigitalProductsPlatform

run:
  command: docker compose up

workflows:
  setup:
    tasks: [build]

tasks:
  build:
    steps:
      - name: Build
        shell: just build
```

Run setup or launch the application:

```bash
update-cli setup
update-cli --run
```

## Change an existing project from ZIP to GitHub

```bash
update-cli config \
  --set mode=pull \
  --set source.type=repository \
  --set source.repository=https://github.com/acme/DigitalProductsPlatform.git \
  --set source.ref=main

update-cli doctor
update-cli check
update-cli update --plan
update-cli update
```

See [GitHub Pull](github) for the detailed repository workflow.
