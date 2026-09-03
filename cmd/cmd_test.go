package cmd

import (
	"testing"
)

func TestRemoveCmdAliases(t *testing.T) {
	aliases := make(map[string]bool)
	for _, a := range removeCmd.Aliases {
		aliases[a] = true
	}
	for _, expected := range []string{"rm", "delete", "kill"} {
		if !aliases[expected] {
			t.Errorf("expected removeCmd to have %q alias, got: %v", expected, removeCmd.Aliases)
		}
	}
}

func TestClearCmdForceFlagAndAliases(t *testing.T) {
	flag := clearCmd.Flags().Lookup("force")
	if flag == nil {
		t.Fatal("expected clearCmd to have --force flag")
	}
	if flag.Shorthand != "f" {
		t.Errorf("expected shorthand 'f', got %q", flag.Shorthand)
	}

	hasC := false
	for _, a := range clearCmd.Aliases {
		if a == "c" {
			hasC = true
		}
	}
	if !hasC {
		t.Errorf("expected clearCmd to have 'c' alias, got: %v", clearCmd.Aliases)
	}
}

func TestNewCmdAliasesAndFlags(t *testing.T) {
	hasCreate := false
	for _, alias := range newCmd.Aliases {
		if alias == "create" {
			hasCreate = true
		}
		if alias == "c" {
			t.Errorf("newCmd should not have 'c' alias (reserved for clear)")
		}
	}
	if !hasCreate {
		t.Errorf("expected newCmd to have 'create' alias, got: %v", newCmd.Aliases)
	}

	attachFlag := newCmd.Flags().Lookup("attach")
	if attachFlag == nil {
		t.Fatal("expected newCmd to have --attach flag")
	}
	if attachFlag.Shorthand != "A" {
		t.Errorf("expected shorthand 'A', got %q", attachFlag.Shorthand)
	}
}
