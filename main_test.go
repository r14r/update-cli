package main

import "testing"

func TestResolvedVersionFallsBackToEmbeddedVersion(t *testing.T) {
	originalVersion := version
	originalEmbedded := embeddedVersion
	defer func() {
		version = originalVersion
		embeddedVersion = originalEmbedded
	}()

	version = "{{version}}"
	embeddedVersion = "1.6.0\n"
	if got := resolvedVersion(); got != "1.6.0" {
		t.Fatalf("resolvedVersion() = %q, want 1.6.0", got)
	}
}
