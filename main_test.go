package main

import "testing"

func TestResolvedVersionUsesEmbeddedVersion(t *testing.T) {
	oldEmbedded := embeddedVersion
	t.Cleanup(func() { embeddedVersion = oldEmbedded })
	embeddedVersion = "2.14.1\n"
	if got := resolvedVersion(); got != "2.14.1" {
		t.Fatalf("resolvedVersion() = %q, want embedded VERSION 2.14.1", got)
	}
}

func TestResolvedVersionFallsBackToDevWithoutEmbeddedVersion(t *testing.T) {
	oldEmbedded := embeddedVersion
	t.Cleanup(func() { embeddedVersion = oldEmbedded })
	embeddedVersion = ""
	if got := resolvedVersion(); got != "dev" {
		t.Fatalf("resolvedVersion() = %q, want dev", got)
	}
}
