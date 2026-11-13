package main

import "testing"

func TestRequireVault(t *testing.T) {
	if err := requireVault(""); err == nil {
		t.Fatal("empty -vault was accepted")
	}
	if err := requireVault("   "); err == nil {
		t.Fatal("blank -vault was accepted")
	}
	if err := requireVault(`C:\dev\my-vault`); err != nil {
		t.Fatal(err)
	}
}
