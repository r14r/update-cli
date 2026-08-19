---
layout: page
title: Build Configuration
---

# Build configuration

`build-config.json` defines distribution-specific defaults used when building/installing `update-cli` itself, including the default binary deployment path and global configuration path.

The project Justfile reads these values through `cmd/buildconfig`.

Typical development flow:

```bash
just check
just build
just install
```

`just install` is the current installation recipe. It replaces the former `just deploy` recipe.

The application projects managed by `update-cli` do not normally need to modify `build-config.json`; their update source is configured in `.updater-cli/config.json` and their automation in `update-cli.yaml`.
