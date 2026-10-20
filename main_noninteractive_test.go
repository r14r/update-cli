package main

import "testing"

func TestNonInteractiveOutput(t *testing.T) {
	for _, flag := range []string{"--noui", "--no-ui", "--no-ask", "--json"} {
		if !nonInteractiveOutput([]string{"install", flag}) {
			t.Fatalf("%s must disable clipboard prompt", flag)
		}
	}
	if nonInteractiveOutput([]string{"install"}) {
		t.Fatal("interactive install must remain eligible for prompt")
	}
}
