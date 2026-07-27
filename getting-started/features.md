---
layout: page
title: Features
---

# Features

## Release Sources

- **Three Release Sources**: Local ZIP folder, direct HTTP(S) URL, or Git repository
- **Latest Version Selection**: Automatically finds newest SemVer release in local folder
- **Temporary Downloads**: Transient folder for URL downloads, auto-cleaned
- **Shallow Clone**: Efficient Git repository support with configurable cleanup

## Version Management

- **Semantic Versioning**: Full SemVer 2.0.0 support (MAJOR.MINOR.PATCH)
- **Version Validation**: Ensures archive and internal VERSION file match
- **Version Comparison**: Installed vs. available version status display
- **Downgrade Protection**: Blocks downgrades by default, explicit `--allow-downgrade` required
- **Atomic Activation**: Updates under `release/<version>` with atomic link switching

## Archive Security

- **ZIP Validation**: CRC checksums, path validation, symlink protection
- **Path Whitelisting**: Only expected paths allowed in archives
- **Symlink Detection**: Rejects archives containing symlinks to prevent traversal
- **Wrapper Folder Support**: Handles archives with single top-level directory
- **Archive Verification**: Standalone `--verify` command for pre-deployment checks

## Deployment Safety

- **Protected Paths**: `.git`, `.venv`, and `.env` are never deleted
- **Dry-Run Mode**: `--plan` and `--dry-run` show what would happen
- **Rollback Support**: One command to return to previous version
- **Atomic Sync**: `rsync --delete --checksum` ensures consistency
- **Update Plans**: Detailed breakdown of planned operations

## Docker Integration

- **Automatic Detection**: Recognizes `compose.yml`, `compose.yaml`, `docker-compose.yml`, `docker-compose.yaml`
- **Safe Shutdown**: Automatically runs `docker compose down --remove-orphans` before updates
- **Strict Enforcement**: Fails if Docker stack can't be stopped safely
- **Container Protection**: Running containers never exposed to partial updates

## Backup & Recovery

- **Automatic Backups**: Optional `--backup` before updates
- **Snapshot Format**: `backup/<YYYYMMDD-HHMMSS>-v<VERSION>/`
- **Smart Exclusions**: Skips `.git`, `.venv`, `node_modules`, `vendor`, etc.
- **Rollback Command**: `--rollback` to previous or specific version
- **Restore Command**: Restore from any backup snapshot
- **Retention Policies**: Configurable automatic cleanup of old backups

## Project Setup

- **Multi-Step Setup**: Combines `current/setup.sh` + configured commands
- **Built-in Templates**: Laravel, Django, FastAPI, Vue, Go
- **Docker-Aware**: Templates stop containers before setup
- **Custom Commands**: Add arbitrary post-deployment commands
- **Flexible Triggers**: Manual or automatic via `--setup` flag

## Status & Monitoring

- **Project Status**: `--status` shows current deployment state
- **Version Inventory**: `--list` shows all available releases and backups
- **Configuration Review**: `--config` displays current settings
- **Health Checks**: `--doctor` validates environment and setup
- **Archive Inspection**: `--verify` checks archive integrity

## Output & Integration

- **JSON Mode**: `--json` for machine-readable output
- **Color Output**: Rich terminal formatting (optional with `--no-color`)
- **History Tracking**: `--history` shows JSON Lines audit trail
- **Exit Codes**: Structured codes for script integration
- **Detailed Logging**: Comprehensive operation output

## Configuration

- **Single Config File**: `.updater-cli/config.json`
- **Schema Versioning**: Automatic migration on `--upgrade`
- **Template System**: `.updater-cli/templates.json` for setup customization
- **Environment Variables**: Support for `$HOME` and other variables
- **Multi-Source Override**: Temporary source switching per command

## Development Features

- **Platform Builds**: Separate macOS (Intel/ARM64) and Linux (AMD64) binaries
- **Build Configuration**: `build-config.json` for distribution defaults
- **Global Templates**: System-wide templates in `defaultConfigPath`
- **Just Integration**: Makefile-like commands for development
- **Go 1.22+**: Latest Go features and performance

## CLI Features

- **Compact Help**: `--help` for quick reference
- **Detailed Howto**: `--howto` for comprehensive guide
- **Context Help**: `--update --help` for command-specific details
- **Flag Completion**: Short and long form for all options
- **Root Override**: `--root` for managing external projects
- **Force Mode**: `--force` for reinstalling same version

Next: [Check Requirements →](/getting-started/requirements)
