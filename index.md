---
layout: page
title: update-cli Documentation
---

# update-cli

**Sicherer Release-Updater für lokale ZIPs, direkte URLs und Git-Repositories**

[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Release](https://img.shields.io/badge/release-v2.4.1-2ea44f)](#version)
[![Platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-6e7681)](#requirements)
[![Tests](https://img.shields.io/badge/tests-go%20test%20.%2F...-2ea44f)](#development)

`update-cli` bezieht Releases aus einem lokalen Ordner, einer direkten HTTP(S)-URL oder einem Git-Repository, legt sie versioniert ab und synchronisiert den Inhalt kontrolliert nach `current`.

## Quick Navigation

- **[Why update-cli?](getting-started/why)** - Problem and solution overview
- **[Features](getting-started/features)** - Complete feature list
- **[Installation](getting-started/installation)** - Setup and deployment
- **[Quickstart](usage/quickstart)** - Get up and running in 5 minutes
- **[Command Reference](usage/commands)** - Complete CLI documentation
- **[Workflows](usage/workflows)** - Common use cases and patterns

## What is update-cli?

`update-cli` handles secure, reproducible updates for projects distributed as versioned ZIP files, direct URLs, or Git repositories. It manages:

✅ Multiple release sources (local folder, HTTP(S) URL, Git repo)
✅ Secure ZIP validation and extraction
✅ Atomic version management
✅ Docker Compose integration
✅ Automatic backups and rollback
✅ Template-based setup automation
✅ Comprehensive JSON output for CI/CD

## Key Features

- **Three Release Sources**: Local ZIP folder, direct HTTP(S) URL, or Git repository
- **Version Safety**: SemVer validation, downgrade protection, atomic updates
- **Docker Support**: Automatic `docker compose down` before updates
- **Backup & Rollback**: Versioned releases with automatic backups
- **Setup Templates**: Built-in templates for Laravel, Django, FastAPI, Vue, Go
- **Protected Paths**: Automatic protection of `.git`, `.venv`, `.env`
- **Dry-Run Support**: Plan updates without filesystem changes
- **JSON Output**: Machine-readable output for scripting and CI
- **Status Monitoring**: Health checks, version comparison, inventory listing

## Documentation Sections

### Getting Started
- [Why update-cli?](getting-started/why) - Problem statement and benefits
- [Features](getting-started/features) - Detailed feature overview
- [Requirements](getting-started/requirements) - System prerequisites
- [Installation](getting-started/installation) - Build and deployment

### Configuration
- [Build Configuration](configuration/build-config) - Distribution-specific settings
- [Project Configuration](configuration/configuration) - config.json schema
- [Archive Format](configuration/archive-format) - ZIP naming and structure
- [Templates](configuration/templates) - Setup templates

### Usage
- [Quickstart](usage/quickstart) - 5-minute quick start
- [Command Reference](usage/commands) - Complete CLI reference
- [Typical Workflows](usage/workflows) - Common scenarios
- [Setup & Templates](usage/setup) - Project initialization and setup

### Advanced Topics
- [Backups & Rollback](advanced/backups) - Backup management and recovery
- [JSON Output](advanced/json-output) - Machine-readable output
- [Security Model](advanced/security) - Security considerations
- [Exit Codes](advanced/exit-codes) - Error codes and meanings

### Development
- [Project Structure](development/structure) - Directory layout
- [Development & Tests](development/development) - Build and test
- [Troubleshooting](development/troubleshooting) - Common issues

---

**Version**: v2.4.1 | **License**: MIT | **Repository**: [github.com/r14r/update-cli](https://github.com/r14r/update-cli)
