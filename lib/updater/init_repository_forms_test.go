package updater

import (
	"testing"

	"github.com/r14r/update-cli/lib/config"
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
