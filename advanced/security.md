---
layout: page
title: Security Model
---

# Security Model

## Overview

update-cli implements multiple security layers to ensure safe, reproducible deployments.

## Archive Validation

### ZIP Integrity

- **CRC Checksums**: Verifies all files using CRC validation
- **Completeness**: Ensures ZIP not truncated or corrupted
- **Path Validation**: Only allows expected directory structures

### Version Verification

```bash
update-cli --verify archive.zip
```

**Checks**:
1. ZIP file integrity
2. Internal `VERSION` file exists and is readable
3. Internal version matches filename version
4. No version mismatches between archive name and content

### Symlink Protection

- **Rejects symlinks** - Archives containing symlinks are rejected
- **Prevents traversal** - No `../` sequences in paths
- **Safe extraction** - Ensures files only go to expected locations

## Update Safety

### Atomic Operations

- **Versioned Storage**: Each release under `release/<VERSION>/`
- **Clean Sync**: `rsync --delete --checksum` for consistency
- **No Partial States**: Either fully updated or unchanged

### Protected Paths

The following are **never deleted** during sync:

```
current/.git/      # Version control
current/.venv/     # Python environment
current/.env       # Configuration
```

These paths are explicitly excluded from `rsync --delete`.

### Downgrade Protection

- **Version Comparison**: SemVer parsing prevents unintended downgrades
- **Explicit Approval**: `--allow-downgrade` required for older versions
- **Clear Warnings**: Highlighted when downgrading

```bash
# Blocked by default
update-cli --update ~/project-v1.0.0.zip
# Error: Version 1.0.0 < installed 2.0.0

# Explicitly allowed
update-cli --update --allow-downgrade ~/project-v1.0.0.zip
```

## Docker Integration Safety

### Strict Enforcement

If `current/` contains Compose file:

1. Must support either `docker compose` or `docker-compose`
2. `docker compose down --remove-orphans` **must succeed**
3. Update aborts if Docker stop fails
4. Running containers never exposed to partial updates

### Supported Compose Files

```
compose.yml
compose.yaml
docker-compose.yml
docker-compose.yaml
```

## File Permissions

### Preservation

- **Executable bits**: Preserved from archive
- **Owner/Group**: Set by extraction process
- **Permissions**: Respects umask

### Sensitive Files

`.env` protection:
```bash
rsync --exclude '.env' ...
```

## Backup Security

### Snapshot Isolation

- **Separate Directory**: `backup/<timestamp>/`
- **Independent Copies**: Not symlinked, standalone files
- **Retention**: Configurable automatic cleanup

### Backup Contents

**Excluded (regenerable)**:
```
.git/
.venv/
node_modules/
vendor/
dist/
build/
__pycache__/
```

**Included**:
- Source code
- Configuration (except `.env`)
- Application data
- Assets

## Configuration Security

### Embedded Defaults

`build-config.json` embedded in binary - cannot be modified at runtime.

### Sensitive Data

- **No Secrets in Config**: `.env` protected separately
- **Path Expansion**: Only `$HOME`, `$USER` supported (no `$()`)
- **No Eval**: Configuration values never evaluated as code

## Audit Trail

### History Tracking

All operations logged to:
```
.updater-cli/history.jsonl
```

**Recorded**:
- Timestamp
- Operation type
- Version
- Success/failure
- User info (when available)

```bash
update-cli --history --json
```

### JSON Output

Machine-readable output for audit logging:

```bash
update-cli --update --json |
  tee -a deployment.log | jq '.success'
```

## Network Security

### HTTPS Validation

For `--from url` source:
- **HTTPS Recommended**: Plain HTTP accepted but not recommended
- **Certificate Validation**: Standard Go TLS validation
- **No MITM Protection**: Consider VPN for sensitive environments

### Git Source Security

For `--from repository`:
- **SSH Support**: Git over SSH with key authentication
- **HTTPS Support**: Standard Git HTTPS
- **Shallow Clone**: Efficient, minimal data transfer
- **No Submodules**: Only main repository cloned

## Principle of Least Privilege

### Required Permissions

- **Read**: Project directory for checks
- **Write**: `release/`, `current/`, `backup/` directories
- **Execute**: `rsync`, `bash`, `git` (when needed)

### Running as Unprivileged User

```bash
# OK - unprivileged user manages /srv/myapp/
sudo -u deploy update-cli --update --root /srv/myapp

# Not recommended - system-wide deployment with --deploy
# Use CI/CD system for privilege escalation
```

## Validation Checklist

### Before Deployment

```bash
# 1. Verify archive
update-cli --verify ~/Downloads/myapp-v2.0.0.zip

# 2. Check environment
update-cli --doctor

# 3. Preview update
update-cli --update --plan

# 4. Test with dry-run
update-cli --update --dry-run

# 5. Execute with backup
update-cli --update --backup
```

## Known Limitations

- **No Cryptographic Signatures**: Archives not signed
- **No Checksum Verification**: Version-based validation only
- **No Role-Based Access**: No per-user permissions
- **Local Repository Support**: Git over SSH requires key setup

## Security Best Practices

1. **Always backup** before production updates
2. **Test in staging** first
3. **Use `--dry-run`** to preview
4. **Monitor `--history`** for audit trails
5. **Keep `.env` separate** from version control
6. **Use `--doctor`** to validate setup
7. **Verify archives** before deployment
8. **Use HTTPS** for remote sources
9. **Automate via CI/CD** for consistency
10. **Retain backups** for compliance

Next: [Exit Codes →](/advanced/exit-codes)
