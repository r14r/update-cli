---
layout: page
title: Command Reference
---

# Command Reference

This page reflects the **1.4.0** CLI contract.

## Equivalent primary commands

For `check`, `update`, and `run`, command-token and flag forms are guaranteed to use the same parser and execution path:

```bash
update-cli check     # identical to update-cli --check
update-cli update    # identical to update-cli --update
update-cli run       # identical to update-cli --run
```

Options and positional arguments are preserved:

```bash
update-cli check --json
update-cli --check --json

update-cli update release.zip --plan
update-cli --update release.zip --plan

update-cli run --root /srv/demo
update-cli --run --root /srv/demo
```

## Help and discovery

```bash
update-cli --help
update-cli --help --json
update-cli --howto
update-cli --version
```

## Core commands

### `update-cli check` / `update-cli --check`

Check for an available project update.

Important options include `--root/-r`, `--json`, `--no-ask`, `--wait`, `--no-wait`, `--no-ui/--noui`, `--no-color`, `--mode update|pull`, `--downloads/-d`, `--from`, `--folder`, `--url`, and `--repository`.

### `update-cli update [archive]` / `update-cli --update [archive]`

Apply the configured ZIP update or Git pull transaction.

Important options include `--archive/-a`, `--dry-run/-n`, `--plan`, `--allow-downgrade`, `--json`, `--backup`, `--setup`, `--no-setup`, `--force/-f`, source overrides, UI options, and `--root/-r`.

### `update-cli run` / `update-cli --run`

Run the application command defined by the active `current/update-cli.yaml`. Both compact `run.command` and structured `run.steps` are supported.

### Other project commands

```text
update-cli backup
update-cli rollback [version]
update-cli restore <backup>
update-cli status
update-cli list
update-cli verify <archive>
update-cli doctor
update-cli clean
update-cli cleanup
update-cli history
update-cli init
update-cli upgrade
update-cli unlock
```

## Setup automation

```text
update-cli setup
update-cli setup list
update-cli setup task NAME
update-cli setup workflow NAME
update-cli setup manifest FILE
update-cli convert-yaml
update-cli create-yaml
update-cli create-setup-script
```

The canonical project automation filename is `update-cli.yaml`.

## Configuration

```bash
update-cli config
update-cli config list
update-cli config edit
update-cli config --set KEY=VALUE
update-cli config --check
update-cli config --migrate
```

Equivalent command-token forms are also available for config operations:

```bash
update-cli config check
update-cli config migrate
```

`config --check` validates the configuration without changing it and reports whether migration is needed. `config --migrate` migrates to the current schema and creates a timestamped backup when a schema change is required.

## Templates

```text
update-cli templates
update-cli templates list
update-cli templates edit
update-cli templates use NAME
```

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
update-cli run
# exactly the same as
update-cli --run
```

### Machine-readable discovery

The complete command/option contract is available through:

```bash
update-cli --help --json
```
