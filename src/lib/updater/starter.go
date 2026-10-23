package updater

// StarterManifest is a complete schemaVersion 2 example that forwards
// supported project lifecycle operations to matching justfile recipes.
func StarterManifest() string {
	return `# Starter manifest for update-cli. Save as update-cli.yaml.
# Customize the project name and slug; supply the matching just recipes.
# Host-specific repository settings belong in .update-cli/config.json.
schemaVersion: 2

project:
  name: Example Application
  slug: example-app
  type: generic

defaults:
  timeout: 15m
  failFast: true

workflows:
  bootstrap:
    tasks: [bootstrap]
  setup:
    tasks: [setup]
  precommit:
    tasks: [precommit]
  test:
    tasks: [test]
  build:
    tasks: [build]
  ci:
    tasks: [ci]
  clean:
    tasks: [clean]

tasks:
  bootstrap:
    steps:
      - id: bootstrap
        shell: just bootstrap
  setup:
    steps:
      - id: setup
        shell: just setup
  precommit:
    steps:
      - id: precommit
        shell: just precommit
  test:
    steps:
      - id: test
        shell: just test
  build:
    steps:
      - id: build
        shell: just build
  ci:
    steps:
      - id: ci
        shell: just ci
  clean:
    steps:
      - id: clean
        shell: just clean

# update-cli install invokes just install directly from project root/current.
# update-cli run executes this configured command.
run:
  command: just run

# Other built-in CLI commands (update, doctor, init, version, etc.) retain
# their native behavior and are not overridden by manifest workflows.
`
}
