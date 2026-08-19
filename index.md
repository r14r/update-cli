---
layout: page
title: update-cli Documentation
---

# update-cli

**Transactional project updates from versioned ZIP releases or Git repositories.**

**Current release: 1.2.1**

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
- **[Command Reference](usage/commands)** — current 1.2.1 CLI commands
- **[Project Configuration](configuration/configuration)** — `.updater-cli/config.json`
- **[`update-cli.yaml`](configuration/update-cli-yaml)** — setup/build/run automation
- **[Run Application](usage/run)** — `update-cli --run`
- **[Recovery](advanced/recovery)** — backups, rollback, restore and cleanup
- **[Security](advanced/security)** — archive and path safety

## Important 1.2.x changes

- Explicit `mode: update` and `mode: pull` acquisition modes.
- Persistent Git checkout in `.updater-cli/repository/` with `git pull --ff-only`.
- Git commit tracking via `.release-commit`, including updates where `VERSION` is unchanged.
- Project automation filename renamed from `setup.yaml` to **`update-cli.yaml`**.
- Application launch with **`update-cli --run`** / `update-cli run`.
- Home-directory paths are displayed as `$HOME/...` in human-readable UI output.
- Development install recipe is **`just install`**.

Repository: [github.com/r14r/update-cli](https://github.com/r14r/update-cli)
