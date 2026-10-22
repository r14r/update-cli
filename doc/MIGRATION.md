# 2.5.0: global + project-local configuration

Existing `.update-cli/config.json` files remain valid. After installing 2.5.0, Update CLI additionally reads `INSTALLFOLDER/../etc/update-cli/config.json` first and overlays the project configuration. For the default installation this is `/usr/local/etc/update-cli/config.json`. Existing project `sync.preserve` entries are combined with the global preserve list. No mandatory project-file migration is required.

`templates.json` follows the same global/local precedence; same-name local templates override global definitions.

# Update CLI 2.1.0 migration

Version 2.1 restores JSON as the persistent updater configuration and keeps YAML focused on setup/run automation.

## Active files

```text
.update-cli/config.json   active update source and update policy
update-cli.yaml           preferred setup/run/tasks/workflows
setup.yaml                legacy setup-manifest fallback
```

## From 2.0.x YAML-only configuration

2.0.x allowed updater settings under top-level `update:` and `cli:` in `update-cli.yaml`. In 2.1.0 those blocks are no longer required and should be moved to `.update-cli/config.json`.

Example old YAML:

```yaml
update:
  mode: pull
  source:
    type: repository
    repository: https://github.com/acme/demo.git
    ref: main

cli:
  noParameter: [check]
```

Equivalent JSON:

```json
{
  "schemaVersion": 7,
  "projectName": "demo",
  "mode": "pull",
  "source": {
    "type": "repository",
    "repository": "https://github.com/acme/demo.git",
    "ref": "main"
  },
  "no parameter": ["check"]
}
```

The YAML setup parser remains tolerant of transitional `update:`/`cli:` blocks so existing 2.0 manifests can still be read by Update CLI 2.1, but the values are not the canonical persistent source configuration.

## Runtime config migration

Older projects can use earlier schema versions in `.update-cli/config.json`.

2.1.0 continues to load that file. To move it to the canonical location:

```bash
update-cli config --check
update-cli config --migrate
```

Migration creates a timestamped backup, writes:

```text
.update-cli/config.json
```

and upgrades `.update-cli/config.json` in place. The timestamped backup remains in `.update-cli/`.

## `setup.yaml`

`setup.yaml` is again recognized as a compatibility filename. Discovery order:

```text
update-cli.yaml
setup.yaml
setup.sh
just build + just install
```

If a legacy `setup.yaml` has no schema declaration, Update CLI infers schema 2 for task/workflow/run-style manifests and schema 1 for older step-oriented structures.
