package session

import (
	"bytes"
	"os"
	"testing"
)

func TestRingBuffer(t *testing.T) {
	rb := newRingBuffer(10)

	// Test partial write
	rb.Write([]byte("hello"))
	if got := string(rb.Bytes()); got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}

	// Test write filling buffer
	rb.Write([]byte("12345"))
	if got := string(rb.Bytes()); got != "hello12345" {
		t.Fatalf("expected 'hello12345', got %q", got)
	}

	// Test wrap around
	rb.Write([]byte("abc"))
	if got := string(rb.Bytes()); got != "lo12345abc" {
		t.Fatalf("expected 'lo12345abc', got %q", got)
	}

	// Test write larger than buffer
	rb.Write([]byte("1234567890EXTRA"))
	if got := string(rb.Bytes()); got != "67890EXTRA" {
		t.Fatalf("expected '67890EXTRA', got %q", got)
	}
}

func TestWriteReadMessage(t *testing.T) {
	buf := new(bytes.Buffer)
	payload := []byte("hello world terminal output")

	if err := writeMessage(buf, MsgOutput, payload); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}

	msgType, data, err := readMessage(buf)
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	if msgType != MsgOutput {
		t.Fatalf("expected MsgOutput (%d), got %d", MsgOutput, msgType)
	}
	if !bytes.Equal(data, payload) {
		t.Fatalf("expected payload %q, got %q", payload, data)
	}
}

func TestValidateName(t *testing.T) {
	valid := []string{"mysession", "session-1", "dev_test", "work123"}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("expected %q to be valid, got err: %v", name, err)
		}
	}

	invalid := []string{"", "foo/bar", "foo\\bar", ".hidden", "my session", "foo\x00bar", "a*b"}
	for _, name := range invalid {
		if err := ValidateName(name); err == nil {
			t.Errorf("expected %q to be invalid, but got nil", name)
		}
	}
}

func TestDefaultDetachKey(t *testing.T) {
	if len(DefaultDetachKeys) == 0 {
		t.Fatalf("DefaultDetachKeys should not be empty")
	}
	if DefaultDetachKeys[0].EscapeChar != '?' {
		t.Fatalf("expected primary default escape char to be '?', got %c", DefaultDetachKeys[0].EscapeChar)
	}
	if got := FormatDetachKeys(DefaultDetachKeys); got != "?. or ~." {
		t.Fatalf("expected '?. or ~.', got %q", got)
	}
}

func TestIsProcessRunning(t *testing.T) {
	// Current process PID is running
	if !isProcessRunning(os.Getpid()) {
		t.Errorf("expected current pid %d to be running", os.Getpid())
	}
	// PID <= 0 is not running
	if isProcessRunning(-1) {
		t.Errorf("expected negative pid to not be running")
	}
}

func TestColorHelpers(t *testing.T) {
	if got := ColorBold("text", false); got != "text" {
		t.Errorf("expected 'text', got %q", got)
	}
	if got := ColorBold("text", true); got != "\033[1mtext\033[0m" {
		t.Errorf("expected colored bold, got %q", got)
	}
	if got := ColorCyan("text", true); got != "\033[36mtext\033[0m" {
		t.Errorf("expected colored cyan, got %q", got)
	}
	if got := ColorGreen("text", true); got != "\033[32mtext\033[0m" {
		t.Errorf("expected colored green, got %q", got)
	}
	if got := ColorYellow("text", true); got != "\033[33mtext\033[0m" {
		t.Errorf("expected colored yellow, got %q", got)
	}
	if got := ColorRed("text", true); got != "\033[31mtext\033[0m" {
		t.Errorf("expected colored red, got %q", got)
	}
	if got := ColorDim("text", true); got != "\033[2mtext\033[0m" {
		t.Errorf("expected colored dim, got %q", got)
	}
}

func TestFormatStatusMsg(t *testing.T) {
	msg := FormatStatusMsg("✨", "created", "test", "?. to detach")
	if !bytes.Contains([]byte(msg), []byte("created")) || !bytes.Contains([]byte(msg), []byte("test")) {
		t.Errorf("FormatStatusMsg output missing expected parts: %s", msg)
	}
}

func TestCleanupStale(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("POCK_DATA_DIR", tempDir)

	curBoot := CurrentBootID()

	// 1. Live session
	liveSess := &Session{
		Name:    "live",
		PID:     os.Getpid(),
		BootID:  curBoot,
		Command: []string{"bash"},
	}
	if err := liveSess.Save(); err != nil {
		t.Fatalf("failed to save live session: %v", err)
	}
	liveSock, _ := SocketPath("live")
	if err := os.WriteFile(liveSock, []byte(""), 0600); err != nil {
		t.Fatalf("failed to create live socket: %v", err)
	}

	// 2. Stale session with dead PID
	staleDead := &Session{
		Name:    "stale-dead",
		PID:     99999999,
		BootID:  curBoot,
		Command: []string{"bash"},
	}
	if err := staleDead.Save(); err != nil {
		t.Fatalf("failed to save stale-dead session: %v", err)
	}
	staleDeadSock, _ := SocketPath("stale-dead")
	_ = os.WriteFile(staleDeadSock, []byte(""), 0600)

	// 3. Stale session from previous boot (if system supports BootID)
	if curBoot != "" {
		staleBoot := &Session{
			Name:    "stale-boot",
			PID:     os.Getpid(),
			BootID:  "old-boot-uuid-from-prev-boot",
			Command: []string{"bash"},
		}
		if err := staleBoot.Save(); err != nil {
			t.Fatalf("failed to save stale-boot session: %v", err)
		}
		staleBootSock, _ := SocketPath("stale-boot")
		_ = os.WriteFile(staleBootSock, []byte(""), 0600)
	}

	// 4. Corrupted json session
	corruptPath, _ := InfoPath("corrupt")
	_ = os.WriteFile(corruptPath, []byte("not valid json {"), 0600)
	corruptSock, _ := SocketPath("corrupt")
	_ = os.WriteFile(corruptSock, []byte(""), 0600)

	// 5. Orphaned .sock and .err files (no json)
	orphanedSock, _ := SocketPath("orphaned")
	_ = os.WriteFile(orphanedSock, []byte(""), 0600)
	orphanedErr, _ := ErrorPath("orphaned")
	_ = os.WriteFile(orphanedErr, []byte("error"), 0600)

	// Run cleanup
	if err := CleanupStale(); err != nil {
		t.Fatalf("CleanupStale failed: %v", err)
	}

	// Verify live session is intact
	if _, err := os.Stat(liveSock); err != nil {
		t.Errorf("expected live session socket to exist, got: %v", err)
	}
	if _, err := Load("live"); err != nil {
		t.Errorf("expected live session json to exist, got: %v", err)
	}

	// Verify stale sessions and orphaned files are cleaned up
	if _, err := os.Stat(staleDeadSock); err == nil {
		t.Errorf("expected stale-dead socket to be removed")
	}
	if _, err := Load("stale-dead"); err == nil {
		t.Errorf("expected stale-dead json to be removed")
	}

	if curBoot != "" {
		staleBootSock, _ := SocketPath("stale-boot")
		if _, err := os.Stat(staleBootSock); err == nil {
			t.Errorf("expected stale-boot socket to be removed")
		}
		if _, err := Load("stale-boot"); err == nil {
			t.Errorf("expected stale-boot json to be removed")
		}
	}

	if _, err := os.Stat(corruptSock); err == nil {
		t.Errorf("expected corrupt socket to be removed")
	}
	if _, err := os.Stat(corruptPath); err == nil {
		t.Errorf("expected corrupt json to be removed")
	}

	if _, err := os.Stat(orphanedSock); err == nil {
		t.Errorf("expected orphaned socket to be removed")
	}
	if _, err := os.Stat(orphanedErr); err == nil {
		t.Errorf("expected orphaned err to be removed")
	}
}

func TestExistsStaleCleanup(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("POCK_DATA_DIR", tempDir)

	// Create stale session with non-existent PID
	sess := &Session{
		Name: "stale-check",
		PID:  99999999,
	}
	if err := sess.Save(); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}
	sockPath, _ := SocketPath("stale-check")
	_ = os.WriteFile(sockPath, []byte(""), 0600)

	// Exists should return false and clean up files
	if Exists("stale-check") {
		t.Errorf("expected Exists to return false for dead session")
	}

	// Verify files were removed
	if _, err := os.Stat(sockPath); err == nil {
		t.Errorf("expected socket to be removed by Exists()")
	}
	if _, err := Load("stale-check"); err == nil {
		t.Errorf("expected json to be removed by Exists()")
	}
}

func TestListCleansStale(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("POCK_DATA_DIR", tempDir)

	// Live session
	liveSess := &Session{
		Name: "live-session",
		PID:  os.Getpid(),
	}
	_ = liveSess.Save()
	liveSock, _ := SocketPath("live-session")
	_ = os.WriteFile(liveSock, []byte(""), 0600)

	// Dead session
	deadSess := &Session{
		Name: "dead-session",
		PID:  99999999,
	}
	_ = deadSess.Save()
	deadSock, _ := SocketPath("dead-session")
	_ = os.WriteFile(deadSock, []byte(""), 0600)

	sessions, err := List()
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected exactly 1 active session, got %d", len(sessions))
	}
	if sessions[0].Name != "live-session" {
		t.Errorf("expected session name 'live-session', got %q", sessions[0].Name)
	}

	// Verify dead session was cleaned up
	if _, err := os.Stat(deadSock); err == nil {
		t.Errorf("expected dead-session socket to be removed")
	}
}
