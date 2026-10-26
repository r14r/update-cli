---
layout: single
title: CLI reference
permalink: /commands/
---

# Commands and parameters

**Source of truth:** the installed binary provides `update-cli help --details`, `update-cli help --command NAME --details` and `update-cli help --json`. The JSON output exposes command options for integrations and tools. Run `update-cli version` to verify the binary actually installed.

## Core lifecycle

| Command | Purpose |
|---|---|
| `help`, `howto`, `version`, `whatsnew` | Help, guide, version and release notes |
| `init` | Initialize a project |
| `check`, `update` | Check and install a release |
| `workflow` | Seven-phase default lifecycle |
| `bootstrap`, `setup` | Prepare prerequisites and apply setup tasks |
| `precommit`, `test`, `ci` | Project validation |
| `build`, `install`, `run` | Build, prepare installation and start |
| `status`, `releases`, `history`, `verify` | Inspect state, releases and history |
| `backup`, `recover`, `clean`, `cleanup` | Backup, interrupted-transaction recovery and retention |
| `doctor`, `fix`, `migrate`, `upgrade`, `unlock` | Diagnose, repair and manage runtime state |
| `config`, `schema`, `templates` | Configurations and manifest schemas |
| `generate-alias`, `generate-config`, `show-requirements` | Generate project integration output |
| `convert-yaml`, `create-yaml`, `create-setup-script` | Manifest and bootstrap authoring |
| `git` | Git helper |

**Removed:** `rollback` and `restore --snapshot` are not public commands. `recover` refers specifically to an interrupted update transaction; it is not an arbitrary release rollback.

## Parameter reference

To list the available commands and parameters:

```bash
update-cli help
update-cli help --details
update-cli help --command update --details
update-cli help --command config --details
update-cli help --json
```

The general help contains the command overview. Command-specific `--details` combines the explanatory examples with the registered flags from CLI discovery. Use `--json` for tooling.

## Workflow modes

```json
{ "no parameter": ["workflow"] }
```

Bare `update-cli` (`u`) executes the seven workflow phases **without confirmation prompts**. To request confirmation before each phase, use:

```json
{ "no parameter": ["workflow-ask"] }
```

The configured value lives in the runtime `.update-cli/config.json`; it is separate from the repository's `update-cli.yaml` (manifest schema 2). For a terminal-friendly output, use `update-cli workflow --noui`.

## Common commands

```bash
update-cli doctor
update-cli doctor --migrate
update-cli config --show
update-cli config --check
update-cli update --plan
update-cli workflow --view --details
update-cli setup --list --json
update-cli ci --noui
update-cli install --noui
update-cli run --noui
```

**Important:** `tasks.install` in `update-cli.yaml` is a manifest task identifier, **not** automatically the same as a `just install` recipe. A manifest workflow referring to `install` must define that task.

## Local quality gate

```bash
just ci           # tests the checked-out working tree, without checkout or pull
just ci-branch    # explicitly fetches/fast-forwards the current branch before testing
```
