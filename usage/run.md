---
layout: page
title: Run Application
---

# Run the active application

Version 1.3.0 supports both a compact run command and structured `run.steps` in `update-cli.yaml`.

Use either CLI form:

```bash
update-cli --run
update-cli run
```

For a managed project, the run definition is read from:

```text
current/update-cli.yaml
```

and executes against the active `current/` release.

## Compact form

Use `run.command` for a single shell command:

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

## Structured steps

Use `run.steps` when the application needs a structured executable/arguments definition, multiple steps, conditions, retries, timeouts, or per-step environment settings.

The Streamlit form below is valid in 1.3.0:

```yaml
schemaVersion: 2

run:
  description: Start Streamlit app
  steps:
    - name: Start Streamlit
      command:
        exec: .venv/bin/streamlit
        args:
          - run
          - app/app.py
```

This executes the equivalent of:

```bash
.venv/bin/streamlit run app/app.py
```

Structured run steps reuse the schemaVersion-2 setup step engine. They therefore support the same step controls, including `cwd`, `env`, `timeout`, `retries`, `when`, and `allowFailure`.

Example with defaults and a step-specific environment value:

```yaml
schemaVersion: 2

run:
  description: Start application
  cwd: app
  env:
    APP_ENV: development
  steps:
    - name: Start server
      timeout: 30m
      env:
        PORT: "8501"
      command:
        exec: ../.venv/bin/streamlit
        args: [run, app.py]
```

`run.command` and `run.steps` are alternatives and must not be configured together.

## More compact examples

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

The child process receives normal stdin/stdout/stderr. Failures from the launched application are returned by `update-cli`; for direct child process failures the application exit code is preserved.
