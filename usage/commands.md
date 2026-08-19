---
layout: page
title: Command Reference
---

# Command Reference

This page is generated from the **1.2.1** machine-readable CLI contract (`update-cli --help --json`).

Preferred syntax uses command tokens (`update-cli check`). Historical flag forms (`update-cli --check`) remain supported where documented by the CLI.

## Help and discovery

```bash
update-cli --help
update-cli --help --json
update-cli --howto
update-cli --version
```

## Commands

### `update-cli check`

Check for an available project update

Key options:

- `--root`, `-r` — Project root directory
- `--json` — Write structured JSON output
- `--no-ask` — Do not ask to install an available update
- `--wait` — Wait before leaving interactive output
- `--no-wait` — Do not wait before leaving interactive output
- `--no-ui`, `--noui` — Disable fullscreen TUI and stream output directly
- `--no-color` — Disable ANSI colors
- `--mode` — Update mode override Choices: `update`, `pull`.
- `--downloads`, `-d` — Downloads/source directory override
- `--from` — Release source type override Choices: `download`, `url`, `repository`.
- `--folder` — Release source folder override
- `--url` — Release source URL override
- `--repository` — Release repository override

### `update-cli update [archive]`

Install a new project release

Key options:

- `--root`, `-r` — Project root directory
- `--archive`, `-a` — Release ZIP archive
- `--dry-run`, `-n` — Preview update without applying it
- `--plan` — Create an update plan without applying changes
- `--allow-downgrade` — Allow installing an older project version
- `--json` — Write structured JSON output
- `--backup` — Create a backup before updating
- `--setup` — Run project setup after update without asking
- `--no-setup` — Do not run project setup after update
- `--force`, `-f` — Force update where supported
- `--wait` — Wait before leaving interactive output
- `--no-wait` — Do not wait before leaving interactive output
- `--no-ui`, `--noui` — Disable fullscreen TUI and stream output directly
- `--no-color` — Disable ANSI colors
- `--mode` — Update mode override Choices: `update`, `pull`.
- `--downloads`, `-d` — Downloads/source directory override
- `--from` — Release source type override Choices: `download`, `url`, `repository`.
- `--folder` — Release source folder override
- `--url` — Release source URL override
- `--repository` — Release repository override

### `update-cli backup`

Create a project backup

### `update-cli rollback [version]`

Restore a previous validated release

### `update-cli restore <backup>`

Restore a project backup

### `update-cli status`

Show project and release status

### `update-cli list`

List releases and backups

### `update-cli verify <archive>`

Verify a release ZIP archive

### `update-cli doctor`

Run project diagnostics

### `update-cli run`

Run the application command from update-cli.yaml

### `update-cli clean`

Remove obsolete release directory entries only

### `update-cli cleanup`

Apply configured release and backup retention

### `update-cli history`

Show update history

### `update-cli init`

Initialize Update CLI configuration for a project

### `update-cli upgrade`

Upgrade project configuration to the current schema

### `update-cli unlock`

Remove a stale update lock

### `update-cli setup`

Run project setup automation

Subcommands:

- `update-cli setup list` — List available setup workflows and tasks
- `update-cli setup task` — Run one setup task
- `update-cli setup workflow` — Run one setup workflow
- `update-cli setup manifest` — Run setup from an explicit manifest file

### `update-cli convert-yaml`

Upgrade update-cli.yaml to the latest supported schema

### `update-cli create-yaml`

Generate schemaVersion 2 update-cli.yaml

### `update-cli create-setup-script`

Generate a setup.sh bootstrap

### `update-cli config`

Show or change project configuration

Subcommands:

- `update-cli config list` — List project configuration files
- `update-cli config edit` — Open config.json in the configured editor
- `update-cli config use-template` — Apply a configuration template

### `update-cli templates`

Manage Update CLI configuration templates

Subcommands:

- `update-cli templates list` — List configuration templates
- `update-cli templates edit` — Open templates.json in the configured editor
- `update-cli templates use` — Apply a configuration template

## Common workflows

### ZIP update

```bash
update-cli check --mode update
update-cli update --plan
update-cli update
```

### GitHub pull

```bash
update-cli check --mode pull
update-cli update --plan
update-cli update
```

### Application runner

```bash
update-cli --run
# or
update-cli run
```

### Configuration

```bash
update-cli config list
update-cli config edit
update-cli config --set source.ref=main
```

### Machine-readable output

Commands that publish `--json` return structured output suitable for scripts and CI. The complete command/option contract is always available through:

```bash
update-cli --help --json
```
