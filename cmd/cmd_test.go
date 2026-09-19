package cmd

import (
	"os"
	"testing"

	"github.com/nghlt/pock/session"
)

func TestRemoveCmdAliases(t *testing.T) {
	aliases := make(map[string]bool)
	for _, a := range removeCmd.Aliases {
		aliases[a] = true
	}
	if !aliases["rm"] {
		t.Errorf("expected removeCmd to have %q alias, got: %v", "rm", removeCmd.Aliases)
	}
	for _, notExpected := range []string{"delete", "kill"} {
		if aliases[notExpected] {
			t.Errorf("expected removeCmd NOT to have %q alias, got: %v", notExpected, removeCmd.Aliases)
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

func TestRootCmdPersistentPreRun(t *testing.T) {
	if rootCmd.PersistentPreRun == nil {
		t.Fatal("expected rootCmd to have PersistentPreRun hook configured")
	}

	tempDir := t.TempDir()
	t.Setenv("POCK_DATA_DIR", tempDir)

	// Create a dead session
	sess := &session.Session{
		Name: "dead-root-test",
		PID:  99999999,
	}
	_ = sess.Save()
	sockPath, _ := session.SocketPath("dead-root-test")
	_ = os.WriteFile(sockPath, []byte(""), 0600)

	// Run PersistentPreRun
	rootCmd.PersistentPreRun(rootCmd, nil)

	// Verify dead session was cleaned up
	if _, err := os.Stat(sockPath); err == nil {
		t.Errorf("expected dead socket to be cleaned up by PersistentPreRun")
	}
	if _, err := session.Load("dead-root-test"); err == nil {
		t.Errorf("expected dead json to be cleaned up by PersistentPreRun")
	}
}
