---
layout: page
title: Terminal demos
permalink: /terminal-demos/
---

# Reproducible terminal recordings

The project includes [VHS](https://github.com/charmbracelet/vhs) terminal recording scripts under [doc/tapes](https://github.com/r14r/update-cli/tree/main/doc/tapes).

```bash
# Install VHS separately; Docker or ttyd may be required by the local setup.
vhs doc/tapes/quickstart.tape
vhs doc/tapes/install.tape
```

**These scripts are recording sources, not proof of a successful run in your environment.** Generated GIF/MP4 assets should only be published after running and checking them.

## Recording proposal

1. Record a simulated walkthrough using a deterministic terminal fixture and clearly label it as a demo.
2. Record a real `update-cli check`, `doctor`, `setup --list` session in an isolated test project.
3. Save `.gif` or `.mp4` under `doc/assets/terminal/` and link them here after verification.
4. Avoid secrets, absolute home paths and real credentials in recordings.

See the [walkthrough with actual screenshots]({{ '/tutorials/' | relative_url }}).
