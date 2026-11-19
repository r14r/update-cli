# Managed development environments

Version 3.6.0 introduces opt-in native environment management. Existing manifests
without a bootstrap section retain their legacy workflows and Just fallback.

## Quick start

```bash
update-cli bootstrap --init --envs python,node
update-cli bootstrap --dry-run
update-cli bootstrap
update-cli bootstrap --check
direnv allow
```

Run from the checkout or use --root. When the supplied directory has no
update-cli.yaml but current/update-cli.yaml exists, current is used.
Without a managed configuration, explicit --env/--envs use built-in defaults.

```yaml
schemaVersion: 2
bootstrap:
  environments:
    python:
      enabled: true
      version: "3.12"
      path: ../.venv/python
      requirements:
        - requirements.txt
    node:
      enabled: true
      version: "24.15.0"
      path: ../.venv/node
      version_manager: nvm
      package_manager: npm
      working_directory: frontend
  direnv:
    enabled: true
    generate: true
```

Paths are relative to the checkout; requirements are relative to working_directory
(default checkout). Python uses an existing compatible python3.x executable; nvm
must already be installed (NVM_DIR/nvm.sh or ~/.nvm/nvm.sh). Node is installed
in the project's NVM_DIR, with npm dependencies in the package directory.
Existing Python environments are preserved; an invalid or mismatched environment
is reported, not removed. Python checks pip consistency and direct requirements;
npm checks the package/lockfile fingerprint and npm ls. npm ci runs for lockfiles.
Complex editable/direct URL Python requirements may trigger repeated installation.
The managed block in .envrc preserves unrelated content. Review changes before
direnv allow; the CLI does not authorize the file automatically.

Options: --env, --envs, --root, --check (nonzero if pending), --status,
--dry-run, --verbose, --force (reinstall dependencies, never delete venv),
--list, --init (append configuration only). --init refuses an existing bootstrap
section. For dry-run/check/status with init, print configuration instead of writing.
Managed configuration replaces the legacy bootstrap workflow for that project;
other workflows remain unchanged.

Example Justfile:

```makefile
python := "../.venv/python/bin/python"
bootstrap:
    update-cli bootstrap --env python
test: bootstrap
    {{python}} -m pytest
```

Native Python and Node runtime installation is not bundled. Windows nvm is not
supported. Go, Ruby, Java, Rust, PHP and Typst are future environment types.
No automatic migration of other repositories is performed.
