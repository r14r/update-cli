---
layout: page
title: Features
---

# Features

## Release acquisition

- **Download Folder** — versioned ZIP archives such as `myapp-v1.4.2.zip`.
- **HTTPS URL** — versioned ZIP acquired from a direct URL.
- **GitHub / Git repository** — persistent checkout with `fetch` + `pull --ff-only`.
- Explicit `mode: update` and `mode: pull` to prevent accidental source-semantics changes.

## Transaction and recovery

- validated release snapshots under `release/<version>/`;
- crash-safe transaction staging;
- optional persistent backups;
- rollback to a validated release;
- restore from a backup;
- stale-lock recovery and cleanup/retention support.

## Git pull mode

- persistent checkout under `.updater-cli/repository/`;
- `.git` excluded from deployed releases;
- target branch/ref through `source.ref`;
- commit tracking through `.release-commit`;
- detects a new commit even when the repository's `VERSION` did not change.

## Project automation

- declarative `update-cli.yaml` schemaVersion 2;
- workflows and reusable tasks;
- task dependencies;
- variables and conditions;
- typed operations plus shell steps;
- optional setup after update;
- application launcher through `run.command` and `update-cli --run`.

## Operations

- `check`, `update`, `status`, `list`, `history`, `doctor`;
- `backup`, `rollback`, `restore`, `clean`, `cleanup`;
- archive verification;
- JSON output;
- fullscreen TUI and `--no-ui` streaming mode;
- user-home path shortening to `$HOME/...` in human-readable output.
