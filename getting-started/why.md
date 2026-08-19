---
layout: page
title: Why update-cli?
---

# Why update-cli?

Application updates are easy to make unsafe: unzip over a live directory, copy files manually, or run `git pull` directly inside the production tree. Those approaches mix source acquisition with deployment state and make rollback, protected files and interrupted updates difficult to reason about.

`update-cli` introduces a stable deployment model:

```text
.updater-cli/       updater metadata, config, history, transactions
release/            validated immutable release snapshots
current/            active application tree
backup/             persistent user backups
```

The application always runs from `current/`. New content comes either from a ZIP release (`mode: update`) or a Git repository (`mode: pull`) and is only promoted to `current/` after validation and transaction preparation.

## Why not just unzip into the application directory?

A direct overwrite can leave a half-installed state when extraction, setup or health checking fails. `update-cli` stages the release first, preserves configured persistent paths, creates recovery state, and only then synchronizes the new tree.

## Why not run `git pull` directly in production?

In pull mode, Git lives in an internal cache:

```text
.updater-cli/repository/
```

`current/` is deliberately **not** a Git working tree. The repository cache is updated using fast-forward-only behavior, then a clean snapshot without `.git` is passed through the normal release transaction. This gives Git-based projects the same rollback semantics as ZIP-based projects.

## What the tool adds

- explicit source/update modes;
- release version validation and downgrade protection;
- transaction snapshots and persistent backups;
- rollback and restore;
- protected paths such as `.env`, data directories and uploads;
- optional Docker Compose lifecycle handling;
- post-update `update-cli.yaml` automation and health checks;
- application launch through `update-cli --run`;
- JSON output for automation and CI;
- status, history, doctor, verification and retention commands.
