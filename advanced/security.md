---
layout: page
title: Security Model
---

# Security model

The updater treats release input as untrusted until validation succeeds.

## ZIP safety

ZIP update mode validates archive structure before promotion. The implementation rejects unsafe or ambiguous extraction paths and applies configured limits such as:

```text
security.maxArchiveBytes
security.maxUncompressedBytes
security.maxFileBytes
security.maxEntries
security.maxCompressionRatio
```

HTTPS is expected for remote ZIP URLs. Plain HTTP requires an explicit `security.allowHttp` opt-in.

## Path safety

Deployment and restore operations use canonical path checks. `update-cli` protects configured persistent paths and prevents automation working directories from escaping the active project tree.

Human-readable UI output shortens paths inside the user's home directory to `$HOME/...`; internal filesystem operations continue to use the real absolute paths.

## Git safety

Pull mode keeps Git metadata in `.updater-cli/repository/`, not in `current/`. Branch updates use fast-forward-only pull behavior, preventing the updater from silently creating merge commits when histories diverge.

## Locks

Update transactions use a project lock to prevent overlapping destructive operations. Stale/incomplete locks can be inspected and explicitly removed with:

```bash
update-cli unlock
```

## Verify an archive

```bash
update-cli verify /path/to/project-v1.2.3.zip
```

Use `--json` when the result is consumed by automation.
