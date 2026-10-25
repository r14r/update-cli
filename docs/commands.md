---
layout: page
title: Commands
permalink: /commands/
---

# CLI command reference

Run `update-cli help` for the authoritative command and parameter list for your installed binary. The commands below reflect the project lifecycle; they are not all justfile aliases.

| Command | Purpose |
|---|---|
| `version`, `whatsnew`, `help` | Version and built-in help |
| `init --repository OWNER/REPO` | Initialize and install a managed project |
| `check`, `update --plan`, `update` | Check and update releases |
| `bootstrap` | Prepare project prerequisites |
| `setup --list`, `setup` | Inspect and execute project setup tasks |
| `precommit`, `test`, `ci` | Run manifest-defined checks |
| `build`, `install`, `run`, `clean` | Build and application lifecycle |
| `doctor`, `fix`, `migrate` | Diagnose and repair configuration |
| `status`, `releases`, `history` | Inspect project and release state |
| `backup`, `recover` | Recovery tooling |
| `config`, `schema`, `templates` | Configuration and manifest utilities |
| `generate-config` | Print a starter `update-cli.yaml` to stdout |
| `generate-alias` | Print a Bash function `u` to stdout |
| `show-requirements` | Print a project integration checklist |
| `git status`, `git add`, `git commit`, `git push` | Git helper commands |

## Typical noninteractive use

```bash
update-cli doctor
update-cli precommit --noui
update-cli build --noui
update-cli install --noui
```

`--noui` disables interactive UI and clipboard confirmation. It does not suppress errors or failure logs.

## Shell shortcut

```bash
eval "$(update-cli generate-alias)"
u version
u doctor
u update --plan
```

Explicit subcommands are forwarded; bare `u` invokes `update-cli update`.
