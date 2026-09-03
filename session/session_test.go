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
