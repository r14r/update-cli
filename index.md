---
layout: page
title: update-cli Documentation
---

# update-cli

**Transactional project updates from versioned ZIP releases or Git repositories.**

**Current release: 1.3.0**

`update-cli` keeps application deployment separate from release acquisition. New source content is validated, stored as a release snapshot, synchronized into a stable `current/` directory, and protected by rollback/recovery state.

## Two update sources

| Source | Mode | Best for | Update identity |
|---|---|---|---|
| Download Folder / HTTPS ZIP | `update` | packaged versioned releases | semantic `VERSION` |
| GitHub / Git repository | `pull` | source-controlled deployments | `VERSION` + Git commit |

Both sources continue through the same transaction pipeline:

```text
source
  ↓
validate / acquire
  ↓
release/<version>/
  ↓
backup + transaction snapshot
  ↓
current/
  ↓
optional update-cli.yaml setup
  ↓
health check
```

## Start here

- **[Why update-cli?](getting-started/why)** — problem, architecture and safety model
- **[Installation](getting-started/installation)** — install or build the CLI
- **[Quickstart](usage/quickstart)** — ZIP and GitHub workflows
- **[GitHub Pull](usage/github)** — detailed Git repository setup and updates
- **[Command Reference](usage/commands)** — CLI commands
- **[Project Configuration](configuration/configuration)** — `.updater-cli/config.json`, validation and migration
- **[`update-cli.yaml`](configuration/update-cli-yaml)** — setup/build/run automation
- **[Run Application](usage/run)** — compact `run.command` and structured `run.steps`
- **[Recovery](advanced/recovery)** — backups, rollback, restore and cleanup
- **[Security](advanced/security)** — archive and path safety

## New in 1.3.0

- Structured application launch through **`run.steps`** in `update-cli.yaml`.
- `run.steps[].command.exec` plus explicit `args`, including Streamlit-style launch definitions.
- Structured run steps reuse the schemaVersion-2 step engine and support `cwd`, `env`, `timeout`, `retries`, `when`, and `allowFailure`.
- New read-only **`update-cli config --check`** / `update-cli config check` validation.
- New safe **`update-cli config --migrate`** / `update-cli config migrate` schema migration with backup.
- Compact `run.command` remains supported.

## 1.2.x foundation

- Explicit `mode: update` and `mode: pull` acquisition modes.
- Persistent Git checkout in `.updater-cli/repository/` with `git pull --ff-only`.
- Git commit tracking via `.release-commit`, including updates where `VERSION` is unchanged.
- Project automation filename renamed from `setup.yaml` to **`update-cli.yaml`**.
- Application launch with **`update-cli --run`** / `update-cli run`.
- Home-directory paths are displayed as `$HOME/...` in human-readable UI output.
- Development install recipe is **`just install`**.

Repository: [github.com/r14r/update-cli](https://github.com/r14r/update-cli)
