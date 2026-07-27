---
layout: page
title: Why update-cli?
---

# Why update-cli?

Projects are frequently distributed as versioned ZIP files or directly from Git repositories. Manual updates are error-prone and problematic:

## The Problem

- **Wrong Archive Selected** - Easy to pick the wrong version from a folder full of releases
- **Version Conflicts** - An older version accidentally overwrites a newer installation
- **Lost Local Files** - Local configuration files (`.env`, `.git`, `.venv`) get deleted
- **Inconsistent State** - Old files remain in the deployment directory, causing subtle bugs
- **Forgotten Setup Steps** - Manual setup commands are forgotten or run in wrong order
- **No Audit Trail** - Impossible to track which version was installed when, making debugging hard

## How update-cli Solves It

`update-cli` handles the entire update workflow reproducibly:

```
Download-Ordner ─┐
Direkte ZIP-URL ─┼─► Version ermitteln und Quelle validieren
Git-Repository ──┘                    │
                                     ▼
                     Docker Compose in current erkennen
                                     │
                      ┌──────────┴──────────┐
                      │ Compose vorhanden? │
                      └──────────┬──────────┘
                                 ▼
                     Container sicher herunterfahren
                                 │
                                 ▼
                     optional current sichern
                                 │
                                 ▼
                     release/X.Y.Z versioniert ablegen
                                 │
                                 ▼
                     per rsync nach current synchronisieren
                                 │
                                 ▼
                     optional Projekt-Setup ausführen
```

## Key Benefits

### ✅ Safety First
- **SemVer Validation** - Enforces strict semantic versioning
- **Downgrade Protection** - Prevents accidental downgrades (unless explicitly allowed)
- **Atomic Operations** - Updates are all-or-nothing
- **Dry-Run Support** - Plan before executing

### ✅ Multiple Source Support
- **Local Folders** - Select latest version from download directory
- **Direct URLs** - Pull specific releases from HTTP(S) endpoints
- **Git Repositories** - Clone from any Git repository

### ✅ Smart Deployment
- **Protected Paths** - Never touches `.git`, `.venv`, or `.env`
- **Docker Integration** - Automatically stops containers before update
- **Clean Sync** - Uses `rsync --delete --checksum` for consistency
- **Template-Based Setup** - Automate post-deployment setup

### ✅ Recovery & Audit
- **Automatic Backups** - Snapshots before each update
- **One-Command Rollback** - Return to previous version instantly
- **Version History** - JSON Lines audit trail of all operations
- **Retention Policies** - Automatic cleanup of old releases

### ✅ Scriptable & Observable
- **JSON Output** - Machine-readable results for CI/CD
- **Exit Codes** - Programmatic success/failure detection
- **Status Monitoring** - Check installed vs. available versions
- **Health Checks** - Doctor command validates environment

## Real-World Scenario

### Without update-cli 😞

```bash
# Download new version
cd ~/Downloads
ls -la | grep myapp
# Which one is the latest?

# Extract manually
unzip myapp-v3.2.0.zip

# Copy to deployment
cp -r myapp-3.2.0/* /srv/myapp/
# Oops! Lost the .env file!

# Run setup (which steps again?)
cd /srv/myapp
bash setup.sh
# Forgot to restart Docker!

# Something broke - how do we rollback?
# ...no version history available
```

### With update-cli ✅

```bash
# Check available version
update-cli --check

# Review what will happen
update-cli --update --plan

# Update with backup and setup
update-cli --update --backup --setup

# Something went wrong?
update-cli --rollback
```

## Use Cases

- **Microservices** - Manage multiple versioned deployments safely
- **Docker Stacks** - Automate container orchestration around updates
- **CI/CD Pipelines** - Reliable, scriptable deployment workflow
- **Backup Compliance** - Automatic snapshots for audit trails
- **Multi-Environment** - Same tool works across dev, staging, production
- **Remote Deployments** - Lightweight, pure shell-compatible operations

Next: [Learn the Features →](/getting-started/features)
