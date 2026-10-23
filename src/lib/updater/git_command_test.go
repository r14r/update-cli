package updater

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateGitArguments(t *testing.T) {
	for _, args := range [][]string{{"status"}, {"add"}, {"push"}, {"commit", "test message"}, {"commit", "multiple", "words"}} {
		if err := validateGitArguments(args); err != nil {
			t.Errorf("%v: %v", args, err)
		}
		if err := ValidatePublicSyntax(append([]string{"git"}, args...)); err != nil {
			t.Errorf("syntax %v: %v", args, err)
		}
	}
	for _, args := range [][]string{{}, {"commit"}, {"commit", ""}, {"commit", "-m"}, {"push", "origin"}, {"add", "."}, {"bad"}} {
		if err := validateGitArguments(args); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}

func TestGitStatusAddCommit(t *testing.T) {
	dir := t.TempDir()
	cmd := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return string(out)
	}
	cmd("init")
	cmd("config", "user.name", "Test User")
	cmd("config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)
	for _, args := range [][]string{{"status"}, {"add"}, {"commit", "example", "message"}} {
		if err := RunGit(context.Background(), args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	if got := cmd("log", "-1", "--pretty=%s"); strings.TrimSpace(got) != "example message" {
		t.Fatalf("commit subject = %q", got)
	}
}
