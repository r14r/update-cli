package updater

import "testing"

func TestRunCommandAndFlag(t *testing.T) {
	for _, args := range [][]string{{"--run"}, {"run"}} {
		o, err := parseOptions(args)
		if err != nil {
			t.Fatalf("parseOptions(%v): %v", args, err)
		}
		if !o.run {
			t.Fatalf("parseOptions(%v) did not select run", args)
		}
	}
}
