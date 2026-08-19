---
layout: page
title: Backups and Recovery
---

# Backups and recovery

`update-cli` keeps deployment recovery separate from the source mechanism. ZIP and GitHub updates both use the same release/backup transaction model.

## Create a persistent backup

```bash
update-cli backup
```

## Roll back to a validated release

```bash
update-cli rollback
update-cli rollback 1.2.0
```

Add `--setup` when project setup should run after rollback.

## Restore a backup

```bash
update-cli restore latest
update-cli restore BACKUP_NAME
```

`restore latest` only considers validated backups and applies canonical path checks.

## Transaction snapshots

Update transactions create temporary recovery state before `current/` is replaced. If an update fails during deployment/setup/health checking, the transaction can restore the previous state.

## Retention

```bash
update-cli cleanup --plan
update-cli cleanup
```

`cleanup` applies configured release and backup retention. `clean` is narrower and removes obsolete release-directory entries only.

## Inspect state

```bash
update-cli list
update-cli history
update-cli status
update-cli doctor
```
