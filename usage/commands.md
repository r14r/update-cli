---
layout: page
title: Command Reference
---

# Command Reference

## Quick Help

```bash
update-cli --help       # Compact overview
update-cli --howto      # Comprehensive guide
update-cli --version    # Show version
```

## Core Commands

### --update

Install or update the project to the latest release.

```bash
update-cli --update
update-cli --update --backup
update-cli --update --setup
update-cli --update --backup --setup
```

**Options**:
- `--backup` - Create backup before update
- `--setup` - Run setup commands after update
- `--dry-run` - Simulate without changes
- `--plan` - Show detailed update plan
- `--force` - Force reinstall of same version
- `--allow-downgrade` - Allow installing older version

### --check

Compare installed version with available version.

```bash
update-cli --check
update-cli --check --json
```

**Output**: Installed vs. available versions and recommended action.

### --status

Show detailed project status.

```bash
update-cli --status
update-cli --status --json
```

**Shows**: Current version, directories, configuration paths.

### --list

List releases, backups, and available sources.

```bash
update-cli --list
update-cli --list --json
```

**Shows**: Available releases in source, installed versions, backups.

## Project Setup

### --init

Initialize new project configuration.

```bash
update-cli --init projectname
update-cli --init projectname --from url \
  --url https://example.com/release.zip
update-cli --init projectname --use-template Laravel
update-cli --init projectname --force
```

**Creates**: `.updater-cli/config.json` and `templates.json`

**Options**:
- `--from TYPE` - Source: `download`, `url`, `repository`
- `--folder DIR` - Download folder (for `download` source)
- `--url URL` - Direct URL (for `url` source)
- `--repository REPO` - Git repo (for `repository` source)
- `--use-template NAME` - Apply template during init
- `--force` - Overwrite existing config

### --setup

Run project setup without updating.

```bash
update-cli --setup
update-cli --setup --json
```

**Executes**:
1. `current/setup.sh` (if exists)
2. All `setup.commands` from config

## Configuration

### --config

Manage project configuration.

```bash
update-cli --config              # Show config
update-cli --config --edit       # Edit in $EDITOR
update-cli --config --list       # List config paths
update-cli --config --json       # Show as JSON
```

**Options**:
- `--edit` - Open in editor
- `--list` - Show configuration file locations

### --upgrade

Migrate configuration to latest schema.

```bash
update-cli --upgrade
update-cli --upgrade --json
```

**Process**:
1. Reads old schema
2. Migrates to current
3. Preserves project-specific values
4. Backs up old config

### --templates

Manage setup templates.

```bash
update-cli --templates --list                 # List templates
update-cli --templates --list --details       # With details
update-cli --templates --use TemplateName     # Apply template
update-cli --templates --edit TemplateName    # Edit template
```

## Backup & Recovery

### --backup

Create backup of current version.

```bash
update-cli --backup
update-cli --backup --json
```

**Format**: `backup/<YYYYMMDD-HHMMSS>-v<VERSION>/`

**Excludes**: `.git`, `.venv`, `node_modules`, `vendor`, etc.

### --rollback

Restore to previous or specific version.

```bash
update-cli --rollback              # Previous version
update-cli --rollback 1.5.0        # Specific version
update-cli --rollback --setup      # With setup
```

**Restores**: `release/<VERSION>/` to `current/`

### --restore

Restore from backup snapshot.

```bash
update-cli --restore backup/20260725-103000-v1.4.0/
```

**Replaces**: `current/` with backup contents.

## Validation & Diagnostics

### --verify

Validate archive integrity and version.

```bash
update-cli --verify /path/to/archive.zip
update-cli --verify ~/Downloads/myapp-v1.0.0.zip --json
```

**Checks**: ZIP validity, version match, path security.

### --doctor

Validate environment and configuration.

```bash
update-cli --doctor
update-cli --doctor --json
```

**Validates**:
- System prerequisites (rsync, bash, git, docker)
- Configuration files
- Directory permissions
- Template validity

## Inspection

### --history

Show operation history.

```bash
update-cli --history
update-cli --history --limit 10
update-cli --history --json
```

**Format**: JSON Lines (one entry per line)

**Includes**: Updates, setups, rollbacks, backups

### --cleanup

Remove old releases and backups.

```bash
update-cli --cleanup
update-cli --cleanup --plan
update-cli --cleanup --dry-run
update-cli --cleanup --keep 10     # Override retention
```

**Respects**: `retention.releases` and `backup.keep` settings

## Global Options

### Working Directory

```bash
update-cli --root /srv/myapp --update
```

Manage project outside current directory.

### Source Override

```bash
update-cli --from download --folder /mnt/releases --update
update-cli --from url --url https://... --update
update-cli --from repository --repository https://... --update
```

Temporarily override configured source.

### Archive Override

```bash
update-cli --archive ~/Downloads/custom-name.zip --update
```

Use specific archive regardless of naming.

### Output Format

```bash
update-cli --json              # Machine-readable output
update-cli --no-color          # Disable ANSI colors
```

### Dry Run

```bash
update-cli --dry-run           # Simulate update
update-cli --plan              # Show detailed plan
```

## Version Information

| Command | Purpose |
|---------|----------|
| `update-cli --version` | Show program version |
| `update-cli --help` | Compact help |
| `update-cli --howto` | Detailed guide |

Next: [Workflows →](/usage/workflows)
