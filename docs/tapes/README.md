# CLI demo tapes

The VHS sources in this directory generate the terminal recordings referenced by `README.md`.

```bash
just tapes
```

or render one recording directly:

```bash
vhs doc/tapes/install.tape
vhs doc/tapes/quickstart.tape
```

Outputs are written to `doc/assets/terminal/`. The install recording sets `UPDATE_CLI_INSTALL_BIN_DIR` to `.demo-install/bin`, so it never writes to `/usr/local`. The quickstart recording creates its release ZIP and project below `.demo-quickstart/`. Both directories are disposable.
