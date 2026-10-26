# Update CLI documentation website

This folder is the Jekyll source for https://r14r.github.io/update-cli/.

The layout and build conventions follow [r14r/github-pages-template](https://github.com/r14r/github-pages-template): Minimal Mistakes theme, left sidebar from `_data/navigation.yml`, sticky right table of contents, search and scrollspy. Update CLI's existing documentation pages remain the content source.

## Build everything from the repository root

```bash
just build-docs
```

This first renders `docs/tapes/install.tape` and `docs/tapes/quickstart.tape` via VHS into `docs/assets/terminal/`, then runs the Jekyll site build using `docs/justfile`. Prerequisites: `vhs` and its terminal recording dependencies, Ruby, Bundler, and the Go toolchain used by the tape recordings. The finished site is written to `_site/` at the repository root.

## Local build

```bash
cd docs
just bootstrap
just ci
just run
```

Alternatively, run `bundle install` and `bundle exec jekyll build --strict_front_matter --destination ../_site`.

The project uses `docs/_config.yml` to preserve the `/update-cli` GitHub Pages base URL. The `docs/justfile` handles website-only tasks; the repository root `justfile` remains for Go CLI development. Do not copy the template's application-level update-cli manifest or VERSION into this folder.

## Publishing

`.github/workflows/pages.yml` validates the Jekyll build on pull requests and publishes it on a push to `main`. GitHub Pages must be configured to use GitHub Actions as the source.

## Template-specific files

- `_config.yml` — theme and website configuration
- `_data/navigation.yml` — sidebar and primary navigation
- `_includes/footer.html` and `_includes/head/custom.html` — shared template includes
- `assets/css/documentation.css` and `assets/js/scrollspy.js` — navigation and table-of-contents presentation
