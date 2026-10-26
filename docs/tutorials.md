---
layout: single
title: Walkthrough
permalink: /tutorials/
---

# Step-by-step CLI walkthrough

The images below are existing repository screenshots, not new recordings. Actual results depend on the project and release.

## 1 — Initialize

```bash
update-cli init --repository OWNER/REPO
```

![Initialization screenshot]({{ '/images/quickstart/01-init.png' | relative_url }})

## 2 — Prepare manifest

```bash
update-cli generate-config > generated-update-cli.yaml
# Review and adapt before replacing update-cli.yaml
update-cli doctor
```

![Manifest screenshot]({{ '/images/quickstart/02-create-yaml.png' | relative_url }})

## 3 — Check and update

```bash
update-cli check
update-cli update --plan
update-cli update
```

![Check screenshot]({{ '/images/quickstart/03-check.png' | relative_url }})
![Update screenshot]({{ '/images/quickstart/04-update.png' | relative_url }})

## 4 — Bootstrap and setup

```bash
update-cli bootstrap --noui
update-cli setup --list
update-cli setup --noui
```

![Setup screenshot]({{ '/images/quickstart/05-setup.png' | relative_url }})

## 5 — Validate

```bash
update-cli precommit --noui
update-cli test --noui
update-cli ci --noui
```

These commands depend on the recipes and workflows defined by the project.

## 6 — Build and install

```bash
update-cli build --noui
update-cli install --noui
```

Installation is a state-changing operation. Run validation first.

## 7 — Launch and inspect

```bash
update-cli run --noui
update-cli status
update-cli history
```

![Status screenshot]({{ '/images/quickstart/06-status.png' | relative_url }})

## 8 — Diagnose issues

```bash
update-cli doctor
update-cli setup --list
update-cli help
```

Review failure logs under `.update-cli/logs/`. Do not replace the project's actual manifest with an unreviewed example.
