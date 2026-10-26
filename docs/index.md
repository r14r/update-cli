---
layout: single
title: Update CLI
---

# Update CLI

**Update, configure, validate, build, install and run projects from a consistent project root.**

Update CLI works with Git repositories and ZIP releases and uses a versioned `update-cli.yaml` manifest plus a host-local `.update-cli/config.json`.

[Start here]({{ '/getting-started/' | relative_url }}) · [CLI reference]({{ '/commands/' | relative_url }}) · [Project configuration]({{ '/configuration/' | relative_url }}) · [Guided walkthrough]({{ '/tutorials/' | relative_url }}) · [Terminal recordings]({{ '/terminal-demos/' | relative_url }})

## Lifecycle

```text
update → bootstrap → install → precommit → test → build → run
```

The default seven-stage workflow runs without confirmation. Configure `no parameter: [workflow-ask]` to confirm each stage. Individual commands remain available. Keep production migrations and destructive operations explicit.

## Example

```bash
update-cli version
update-cli doctor
update-cli bootstrap --noui
update-cli test --noui
update-cli build --noui
update-cli install --noui
update-cli run --noui
```

## Documentation

- [Manifest schema and examples]({{ '/configuration/' | relative_url }})
- [Precommit checks]({{ '/precommit/' | relative_url }})
- [Lifecycle details]({{ '/lifecycle/' | relative_url }})
- [Full source and release notes](https://github.com/r14r/update-cli)
