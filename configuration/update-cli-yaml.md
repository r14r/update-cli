---
layout: page
title: update-cli.yaml
---

# `update-cli.yaml`

`update-cli.yaml` is the canonical project automation manifest. The old filename `setup.yaml` was replaced in version 1.2.

It controls actions that happen **after** project content has been acquired. The Download Folder or Git repository URL remains in `.updater-cli/config.json`.

## Minimal run-only manifest

```yaml
schemaVersion: 2

project:
  name: Example

run:
  command: ./example
```

## Workflow/task example

```yaml
schemaVersion: 2

project:
  name: Example CLI
  type: go
  description: Build, test and install Example CLI

variables:
  binary: example
  distDir: dist

defaults:
  failFast: true
  timeout: 10m

requirements:
  commands: [go]

run:
  command: ./dist/example
  cwd: .

workflows:
  setup:
    tasks: [install]
  ci:
    tasks: [verify]

tasks:
  check:
    steps:
      - id: vet
        name: Static analysis
        go:
          action: vet

  build:
    requires: [check]
    steps:
      - id: build
        name: Build
        shell: |
          mkdir -p "{{ distDir }}"
          go build -o "{{ distDir }}/{{ binary }}" .

  verify:
    requires: [build]
    steps:
      - id: verify
        name: Verify binary
        assert:
          executable: "{{ distDir }}/{{ binary }}"

  install:
    requires: [verify]
    steps:
      - id: install
        name: Install binary
        deploy:
          source: "{{ distDir }}/{{ binary }}"
          target: "/usr/local/bin/{{ binary }}"
          mode: "0755"
```

## Run configuration

```yaml
run:
  command: npm run dev
  cwd: frontend
  env:
    NODE_ENV: development
```

Run with:

```bash
update-cli --run
```

## Setup execution

```bash
update-cli setup
update-cli setup list
update-cli setup task TASK
update-cli setup workflow WORKFLOW
update-cli setup manifest ./other-update-cli.yaml
```

Historical flag forms such as `--setup-task` and `--setup-workflow` remain supported.

## File lifecycle

```bash
update-cli convert-yaml
update-cli create-yaml --from project
update-cli create-yaml --from setup-script
update-cli create-setup-script
```

The current schema is `schemaVersion: 2`.
