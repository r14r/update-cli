# Release Notes

## v2.4.1

- Docker-Compose-Projekte werden vor jedem echten Update automatisch erkannt
- vorhandene Container werden vor Backup, Release-Aktivierung und rsync mit `docker compose down --remove-orphans` gestoppt
- Legacy-Fallback auf `docker-compose` ergänzt
- schlägt der Docker-Stopp fehl oder fehlt Docker bei erkannter Compose-Datei, wird das Update vor Projektänderungen abgebrochen
- `--plan` und `--dry-run` zeigen die Docker-Erkennung, verändern den Containerzustand aber nicht
- Doctor, Update-Ausgabe, README und Tests um die Docker-Sicherheitsprüfung erweitert

## v2.4.0

- `build-config.json` für distributionsspezifische Build-Standards ergänzt
- Standard-Downloadordner, Deployment-Pfad und globaler Konfigurationspfad werden beim Build eingebettet
- `just deploy` verwendet `defaultDeploymentPath` statt eines fest codierten Pfads
- globale Zusatztemplates aus `<defaultConfigPath>/templates.json` werden bei `--init` und `--upgrade` berücksichtigt
- `--config --list` und `--doctor` zeigen beziehungsweise prüfen den globalen Template-Katalog
- `just build-config` und `just build-config-show` ergänzt

## v2.3.0

- neue Release-Quellen `download`, `url` und `repository`
- neue CLI-Parameter `--from`, `--folder`, `--url` und `--repository`
- lokale Download-Quelle verwendet standardmäßig `~/Downloads`
- direkte HTTP(S)-ZIPs werden in einen temporären Arbeitsordner geladen und vollständig validiert
- Git-Repositories werden flach geklont, über die Root-Datei `VERSION` versioniert, von `.git` bereinigt und atomar nach `release/<version>` verschoben
- `config.json` auf Schema 5 mit dem neuen Bereich `source` migriert
- `--init`, `--check`, `--status`, `--list` und `--doctor` unterstützen Quellenparameter
- Doctor, JSON-Ausgaben, README und Tests um Quellinformationen erweitert

## v2.2.3

- alle mitgelieferten Setup-Templates stoppen vor den Setup-Schritten vorhandene Docker-Compose-Stacks mit `docker compose down --remove-orphans`
- Compose-Dateien `compose.yml`, `compose.yaml`, `docker-compose.yml` und `docker-compose.yaml` werden erkannt
- fehlt Docker oder eine Compose-Datei, wird der Docker-Stopp übersprungen
- `--upgrade` ergänzt den Docker-Stopp in unveränderten älteren Basistemplates, erhält aber angepasste und eigene Templates
- Template-Beschreibungen, README und Tests aktualisiert

## v2.2.2

- `--templates --list` zeigt ausschließlich Template-Name und Beschreibung in zwei Spalten
- `--templates --list --details` ergänzt parameterlose Aktionen und alle Setup-Kommandos
- Detailhilfe und README um `--details` erweitert
- bereits installierte Versionen werden ohne `FEHLER`-Präfix gemeldet
- Versionsmeldung wird im Terminal vollbreit mit rotem Hintergrund und weißer Schrift hervorgehoben; der Handlungshinweis bleibt normaler Text

## v2.2.1

- Updates derselben bereits installierten Version werden standardmäßig blockiert
- `update-cli --update --force` erlaubt eine ausdrückliche Reinstallation
- `--force` umgeht den Downgrade-Schutz nicht; ältere Versionen benötigen weiterhin `--allow-downgrade`
- Update-Hilfe und README um die Reinstallationsregeln erweitert

## v2.2.0

- `--init PROJECTNAME --use-template NAME` wendet ein Basistemplate direkt bei der Initialisierung an
- `.updater-cli/templates.json` wird bei `--init` und bei Bedarf durch `--upgrade` erzeugt
- neue Template-Verwaltung: `--templates --list`, `--templates --use NAME`, `--templates --edit NAME`
- neues Template `update-and-setup` setzt den parameterlosen Ablauf auf `update` und `setup`
- `--config --list` zeigt config.json, templates.json und history.jsonl
- kontextspezifische Hilfe, z. B. `update-cli --config --help` und `update-cli --templates --help`
- `--help` zeigt alle Befehle, Parameter und Unterparameter
- Doctor validiert den lokalen Template-Katalog
- Konfigurationsschema auf Version 4 angehoben

## v2.1.5

- Update-Header zeigt den Versionswechsel als `Release Update     from <alt> to <neu>`
- Erstinstallationen verwenden `from none to <neu>`
- Update-Header wird terminalbreit mit blauem Hintergrund und weißer Schrift hervorgehoben
- README-Screenshot für `--update` aktualisiert

## v2.1.4

- README um zusätzliche GitHub-Screenshots für `--help`, `--howto`, `--init` und `--update` erweitert
- neue PNG-Dateien unter `docs/images/` hinzugefügt
- Release-Badge und Versionsdatei auf 2.1.4 aktualisiert

## v2.1.3

## Documentation

- Replaced the existing README with a comprehensive GitHub-oriented project guide.
- Added a local terminal screenshot under `docs/images/update-cli-check.png`.
- Added quickstart, architecture, configuration, full command reference, workflows, setup templates, backup/rollback/restore, JSON automation, security model, development, and troubleshooting sections.
- Added repository-friendly badges, navigation links, and an indexed table of contents.

## Validation

- Verified all README commands against the current CLI help and implementation.
- Verified the screenshot path and PNG format.
- Re-ran formatting, static analysis, unit tests, setup script validation, and native build.

# Release Notes 2.1.2

## Changed

- `update-cli --check` now renders fields in the order project, source, archive, installed version, available version, and status.
- The status line no longer uses `WARN` or `OK` diagnostic prefixes.
- In interactive terminals, the complete status line uses a blue background and bright white foreground through the right edge of the terminal line.
- Non-interactive and no-color output retains the same aligned plain-text structure.

## Validation

- Output-order regression test.
- Plain-text status formatting test.
- ANSI full-width background formatting test.

# Release Notes 2.1.1

## Changed

- `update-cli --help` now shows only the available commands and a pointer to `--howto`.
- `update-cli --howto` contains the former detailed help with usage forms, options, examples, configuration guidance, setup templates, archive naming, and protected paths.
- The configured no-parameter action `"help"` now uses the concise command overview.

## Added

- New `--howto` parameter.
- New `just help` and `just howto` recipes.
- Tests that keep the short and detailed help outputs separate.

# Release Notes 2.1.0

## Added

- `"no parameter"` now accepts a list of actions.
- `"no parameter": ["update", "setup"]` performs the same workflow as `update-cli --update --setup`.
- Legacy string values remain readable and are migrated by `update-cli --upgrade`.

## Changed

- The configuration schema is now version 3.
- New configurations use `"no parameter": ["help"]`.
- Update output renders project-local paths as `./...` while paths outside the project remain absolute.
- Update output now includes the absolute project root and a compact protected-path summary.

## Validation

- Supported actions are `help`, `update`, and `setup`.
- `help` cannot be combined with another action.
- Duplicate actions and unknown actions are rejected.
- `update` and `setup` are normalized to the safe execution order: update first, setup second.

# Release Notes 2.0.0

## Added

- `update-cli --upgrade` migrates `.updater-cli/config.json` to the current schema.
- `update-cli --upgrade --json` provides a machine-readable migration result.
- Configuration upgrades create a timestamped backup before writing changes.
- Missing current settings are added with safe defaults while existing project-specific values are preserved.
- New `just upgrade` recipe.

## Changed

- Project initialization now takes the project name directly: `update-cli --init release-updater-go`.
- The former `--project-name` option has been removed.
- The configuration schema is now version 2.
- Older schema-version-1 configurations remain readable and can be persisted in the current form with `--upgrade`.

## Upgrade behavior

The migration preserves:

```text
projectName
downloadDir
releaseDir
currentDir
no parameter
setup.commands
backup settings
retention settings
```

It adds missing current defaults and rejects unknown fields or configuration schemas newer than the installed CLI.

# Release Notes 1.9.1

## Added

- New `"no parameter"` setting in `.updater-cli/config.json`.
- `"no parameter": "help"` keeps the existing behavior and shows CLI help.
- `"no parameter": "setup"` executes the same project setup workflow as `update-cli --setup`.
- New configurations created by `--init` contain `"no parameter": "help"`.

## Compatibility

- Existing configuration files without the new field remain valid and default to `help`.
- Invalid values are rejected during configuration validation and by `--doctor`.

# Release Notes 1.9.0

## Added

- `update-cli --backup` for a standalone snapshot of `current`.
- `update-cli --update --backup` for an automatic snapshot before installation.
- `update-cli --rollback [VERSION]` to activate the previous or an explicitly selected validated release.
- `update-cli --rollback VERSION --setup` to run the project setup after rollback.
- `update-cli --restore BACKUP` to restore a named backup or `latest`.
- `update-cli --history [--limit N]` backed by `.updater-cli/history.jsonl`.
- `update-cli --cleanup [--keep N] [--plan]` for release and backup retention.
- JSON output for backup, rollback, restore, history, and cleanup operations.
- Backup and retention sections in newly generated `config.json` files.
- Just recipes: `backup`, `rollback`, `restore`, `history`, `cleanup`, and `cleanup-plan`.

## Changed

- `--list` now includes backup inventory and validation state.
- `--status` now includes backup and history counts.
- `--doctor` checks the backup destination and backup metadata.
- CLI argument parsing accepts options before or after a rollback version.
- Existing v1.8 configuration files remain valid and receive runtime defaults.

## Backup behavior

Backups exclude generated dependency and build directories:

```text
.git/
.venv/
node_modules/
vendor/
dist/
build/
__pycache__/
```

Restore and rollback preserve local state in:

```text
current/.git/
current/.venv/
current/.env
```

## Retention safety

Cleanup never removes the active release or the immediately preceding validated release, even when `--keep 0` is used.
