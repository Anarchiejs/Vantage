package main

import "testing"

func TestVersionDefault(t *testing.T) {
	if version != "dev" {
		t.Fatalf("default version = %q", version)
	}
}
