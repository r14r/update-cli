package updater

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/r14r/update-cli/src/lib/config"
)

func TestInitRepositoryForms(t *testing.T) {
	cases := []string{
		"travelguide",
		"r14r/travelguide",
		"https://github.com/r14r/travelguide",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			o, err := parseOptions([]string{"init", "--repository", input})
			if err != nil {
				t.Fatal(err)
			}
			if o.projectName != "travelguide" {
				t.Fatalf("project name = %q", o.projectName)
			}
			normalized, err := config.NormalizeRepositorySpec(o.repository, config.DefaultRepositoryUser)
			if err != nil {
				t.Fatal(err)
			}
			if normalized != "https://github.com/r14r/travelguide.git" {
				t.Fatalf("repository = %q", normalized)
			}
		})
	}
}

func TestInitRepositoryDefaultUserFromGlobalConfig(t *testing.T) {
	root := t.TempDir()
	global := t.TempDir()
	t.Setenv("UPDATE_CLI_GLOBAL_CONFIG_DIR", global)
	data := []byte(`{"source":{"defaultUser":"customowner"}}`)
	if err := os.WriteFile(filepath.Join(global, "config.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	user := initRepositoryDefaultUser(root, false)
	if user != "customowner" {
		t.Fatalf("default user = %q", user)
	}
	repo, err := config.NormalizeRepositorySpec("travelguide", user)
	if err != nil {
		t.Fatal(err)
	}
	if repo != "https://github.com/customowner/travelguide.git" {
		t.Fatalf("normalized repository = %q", repo)
	}
	for _, explicit := range []string{"r14r/travelguide", "https://github.com/r14r/travelguide"} {
		normalized, err := config.NormalizeRepositorySpec(explicit, user)
		if err != nil {
			t.Fatal(err)
		}
		if normalized != "https://github.com/r14r/travelguide.git" {
			t.Fatalf("explicit repository %q normalized to %q", explicit, normalized)
		}
	}
}
