---
layout: page
title: Quickstart
---

# Quickstart

Get up and running with update-cli in 5 minutes.

## Step 1: Initialize Project

In your project directory:

```bash
update-cli --init myproject
```

This creates:
```
.updater-cli/
├── config.json
├── templates.json
└── history.jsonl
```

## Step 2: Prepare Release

update-cli supports three sources. Choose one:

### Option A: Local Folder (Default)

Place a ZIP in your downloads:

```bash
ls -la ~/Downloads/myproject-v1.0.0.zip
```

### Option B: Direct URL

```bash
update-cli --init myproject --from url \
  --url https://releases.example.com/myproject-v1.0.0.zip
```

### Option C: Git Repository

```bash
update-cli --init myproject --from repository \
  --repository https://github.com/myorg/myproject.git
```

## Step 3: Check Version Availability

```bash
update-cli --check
```

Example output:
```
Installed version: none
Available version: v1.0.0
Action required:  --update
```

## Step 4: Preview Update

See exactly what will happen:

```bash
update-cli --update --plan
```

Example output:
```
Update Plan
===========
Source:    download (~/Downloads)
Archive:   myproject-v1.0.0.zip
Version:   1.0.0
Backup:    No
Setup:     No

Steps:
  1. Validate archive and version
  2. Extract to release/1.0.0/
  3. Sync to current/
```

## Step 5: Execute Update

### Basic Update

```bash
update-cli --update
```

### Update with Backup

```bash
update-cli --update --backup
```

### Update with Setup

```bash
update-cli --update --setup
```

### Update with Both

```bash
update-cli --update --backup --setup
```

## Step 6: Verify Success

```bash
update-cli --status
```

Example output:
```
Project Status
==============
Project name:     myproject
Current version:  1.0.0
Release dir:      release/
Current dir:      current/
Backup dir:       backup/
```

## Complete Directory Structure

After successful update:

```
myproject/
├── .updater-cli/
│   ├── config.json
│   ├── templates.json
│   └── history.jsonl
├── release/
│   ├── .last-version
│   └── 1.0.0/
│       ├── app.py
│       ├── requirements.txt
│       └── ...
├── current/
│   ├── app.py              (symlink to ../release/1.0.0/app.py)
│   ├── requirements.txt
│   ├── .env                (protected, not overwritten)
│   └── ...
└── backup/                 (if --backup was used)
```

## Common Next Steps

### Add Setup Commands

Edit configuration:

```bash
update-cli --config --edit
```

Add setup commands:

```json
"setup": {
  "commands": [
    "pip install -r requirements.txt",
    "python manage.py migrate"
  ]
}
```

### Use a Template

For Django:

```bash
update-cli --init myproject --use-template Django
```

Available templates:
- `Laravel` - PHP framework
- `Django` - Python framework
- `FastAPI` - Python async API
- `Vue` - JavaScript frontend
- `Go` - Go applications

### Check Version Updates

Regularly:

```bash
update-cli --check
```

In scripts:

```bash
update-cli --check --json
```

### Rollback if Needed

Return to previous version:

```bash
update-cli --rollback
```

Return to specific version:

```bash
update-cli --rollback 1.0.0
```

## Troubleshooting

### "Version X.Y.Z is already installed"

Force reinstall:

```bash
update-cli --update --force
```

### Docker containers didn't stop

The update was safely aborted. Check Docker:

```bash
docker compose ps
```

Stop manually and retry:

```bash
docker compose down
update-cli --update
```

### Archive verification failed

Verify the ZIP:

```bash
update-cli --verify ~/Downloads/myproject-v1.0.0.zip
```

### Need more information

Run doctor command:

```bash
update-cli --doctor
```

Shows environment status, paths, and configuration.

## Next Steps

- **[Command Reference](../usage/commands)** - Full command documentation
- **[Workflows](../usage/workflows)** - Common use cases
- **[Templates](../configuration/templates)** - Setup automation
- **[Troubleshooting](../development/troubleshooting)** - Solving problems

**Version**: v2.4.1 | **Docs**: [GitHub Pages](https://r14r.github.io/update-cli)
