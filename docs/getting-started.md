---
layout: page
title: Getting started
permalink: /getting-started/
---

# Install and inspect

Follow the [repository README](https://github.com/r14r/update-cli#readme) for platform-specific installation prerequisites. Verify which executable is active:

```bash
command -v update-cli
update-cli version
update-cli help
```

## Initialize a project

In the intended project root:

```bash
update-cli init --repository OWNER/REPO
```

`REPO` alone requires `source.defaultUser` in the local or global `config.json`. GitHub HTTPS URLs are accepted too. `init` installs the initial release and runs configured setup; use a disposable environment first.

![Example initialization output]({{ '/images/quickstart/01-init.png' | relative_url }})

## Generate and review a manifest

```bash
update-cli generate-config > update-cli.yaml
update-cli doctor
update-cli setup --list
```

**Warning:** Redirection overwrites any existing manifest. Use a temporary file when reviewing.

![Example manifest creation]({{ '/images/quickstart/02-create-yaml.png' | relative_url }})

## Update the application

```bash
update-cli check
update-cli update --plan
update-cli update
```

![Check output]({{ '/images/quickstart/03-check.png' | relative_url }})
![Update output]({{ '/images/quickstart/04-update.png' | relative_url }})

## Inspect and run setup

```bash
update-cli setup --list
update-cli bootstrap --noui
update-cli setup --noui
```

![Setup output]({{ '/images/quickstart/05-setup.png' | relative_url }})

## Inspect the result

```bash
update-cli status
update-cli history
```

![Status output]({{ '/images/quickstart/06-status.png' | relative_url }})

Continue with the [complete walkthrough]({{ '/tutorials/' | relative_url }}).
