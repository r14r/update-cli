---
layout: page
title: Run Application
---

# Run the active application

Version 1.2 adds an application runner driven by `update-cli.yaml`.

Use either form:

```bash
update-cli --run
update-cli run
```

For a managed project, the command is read from:

```text
current/update-cli.yaml
```

and runs against the active `current/` release.

## Configuration

```yaml
schemaVersion: 2

project:
  name: Example

run:
  command: docker compose up
  cwd: .
  env:
    APP_ENV: production
```

`command` is executed through a shell, so compound shell syntax is supported. `cwd` is relative to the active project directory and may not escape it. `env` is optional.

Examples:

```yaml
# Docker Compose
run:
  command: docker compose up

# Just
run:
  command: just start

# Node.js
run:
  command: npm run dev
  cwd: frontend

# Python
run:
  command: .venv/bin/python -m myapp

# Compiled binary
run:
  command: ./dist/myapp
```

A run-only manifest is valid; setup tasks are not required:

```yaml
schemaVersion: 2
project:
  name: Example
run:
  command: ./example
```

The child process receives normal stdin/stdout/stderr, and its exit code is propagated by `update-cli`.
