package cmd

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/nghlt/pock/session"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func TestRenderSessionsTableAlignment(t *testing.T) {
	sessions := []*session.Session{
		{
			Name:       "main",
			PID:        1234,
			Command:    []string{"bash"},
			LastActive: time.Now().Add(-10 * time.Second),
		},
		{
			Name:       "long-session-name-here",
			PID:        5678,
			Command:    []string{"vim", "file.go"},
			LastActive: time.Now().Add(-2 * time.Hour),
		},
		{
			Name:       "web",
			PID:        9999,
			Command:    nil,
			LastActive: time.Time{},
		},
	}

	for _, isTTY := range []bool{false, true} {
		var buf bytes.Buffer
		renderSessionsTable(&buf, sessions, "main", isTTY)

		output := buf.String()
		lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
		if len(lines) != 4 { // 1 header + 3 sessions
			t.Fatalf("expected 4 lines, got %d:\n%s", len(lines), output)
		}

		// Strip ANSI codes to check visual alignment
		headerPlain := stripANSI(lines[0])
		statusCol := strings.Index(headerPlain, "STATUS")
		lastActiveCol := strings.Index(headerPlain, "LAST ACTIVE")
		commandCol := strings.Index(headerPlain, "COMMAND")

		if statusCol == -1 || lastActiveCol == -1 || commandCol == -1 {
			t.Fatalf("missing header columns in %q", headerPlain)
		}

		for idx, rawLine := range lines[1:] {
			plainLine := stripANSI(rawLine)
			// Check # starts with row index
			expectedNum := strings.TrimSpace(formatNum(idx + 1))
			if !strings.HasPrefix(strings.TrimSpace(plainLine), expectedNum) {
				t.Errorf("[isTTY=%v] row %d: expected # to start with %q, got line %q",
					isTTY, idx, expectedNum, plainLine)
			}
			// Check STATUS starts at statusCol
			if !strings.HasPrefix(plainLine[statusCol:], "running") {
				t.Errorf("[isTTY=%v] row %d: expected 'running' at col %d, got %q\nfull line: %q",
					isTTY, idx, statusCol, plainLine[statusCol:], plainLine)
			}
			// Check LAST ACTIVE starts at lastActiveCol
			timeVal := formatRelativeTime(sessions[idx].LastActive)
			if !strings.HasPrefix(plainLine[lastActiveCol:], timeVal) {
				t.Errorf("[isTTY=%v] row %d: expected %q at col %d, got %q\nfull line: %q",
					isTTY, idx, timeVal, lastActiveCol, plainLine[lastActiveCol:], plainLine)
			}
			// Check COMMAND starts at commandCol
			cmdVal := strings.Join(sessions[idx].Command, " ")
			if cmdVal == "" {
				cmdVal = "(default shell)"
			}
			if !strings.HasPrefix(plainLine[commandCol:], cmdVal) {
				t.Errorf("[isTTY=%v] row %d: expected %q at col %d, got %q\nfull line: %q",
					isTTY, idx, cmdVal, commandCol, plainLine[commandCol:], plainLine)
			}
		}
	}
}

func formatNum(n int) string {
	return strings.TrimSpace(string(rune('0' + n)))
}

func TestRunInteractiveMenu(t *testing.T) {
	sessions := []*session.Session{
		{Name: "session-a"},
		{Name: "session-b"},
	}

	// 1. Select 0 -> Exit (no attach called)
	{
		var attached string
		in := strings.NewReader("0\n")
		var out bytes.Buffer
		runInteractiveMenu(in, &out, sessions, false, func(name string) {
			attached = name
		})
		if attached != "" {
			t.Errorf("expected no attach on '0', got %q", attached)
		}
	}

	// 2. Select valid session number 1 -> attach session-a
	{
		var attached string
		in := strings.NewReader("1\n")
		var out bytes.Buffer
		runInteractiveMenu(in, &out, sessions, false, func(name string) {
			attached = name
		})
		if attached != "session-a" {
			t.Errorf("expected attach 'session-a', got %q", attached)
		}
	}

	// 3. Select valid session number 2 -> attach session-b
	{
		var attached string
		in := strings.NewReader("2\n")
		var out bytes.Buffer
		runInteractiveMenu(in, &out, sessions, false, func(name string) {
			attached = name
		})
		if attached != "session-b" {
			t.Errorf("expected attach 'session-b', got %q", attached)
		}
	}

	// 4. Type a new session name -> attach new name
	{
		var attached string
		in := strings.NewReader("my-new-session\n")
		var out bytes.Buffer
		runInteractiveMenu(in, &out, sessions, false, func(name string) {
			attached = name
		})
		if attached != "my-new-session" {
			t.Errorf("expected attach 'my-new-session', got %q", attached)
		}
	}

	// 5. Invalid number, then invalid name, then valid 0 to exit
	{
		var attached string
		in := strings.NewReader("99\ninvalid/name\n0\n")
		var out bytes.Buffer
		runInteractiveMenu(in, &out, sessions, false, func(name string) {
			attached = name
		})
		if attached != "" {
			t.Errorf("expected no attach, got %q", attached)
		}
		output := out.String()
		if !strings.Contains(output, "invalid selection") {
			t.Errorf("expected error about invalid selection, got: %s", output)
		}
		if !strings.Contains(output, "must not contain path separators") {
			t.Errorf("expected error about invalid name, got: %s", output)
		}
	}

	// 6. Only-digits name rejection (e.g. 500 when len=2)
	{
		var attached string
		in := strings.NewReader("500\n0\n")
		var out bytes.Buffer
		runInteractiveMenu(in, &out, sessions, false, func(name string) {
			attached = name
		})
		if attached != "" {
			t.Errorf("expected no attach, got %q", attached)
		}
		if !strings.Contains(out.String(), "invalid selection \"500\"") {
			t.Errorf("expected rejection of 500, got: %s", out.String())
		}
	}

	// 7. Empty sessions list: enter name to create
	{
		var attached string
		in := strings.NewReader("first-session\n")
		var out bytes.Buffer
		runInteractiveMenu(in, &out, nil, false, func(name string) {
			attached = name
		})
		if attached != "first-session" {
			t.Errorf("expected attach 'first-session', got %q", attached)
		}
	}

	// 8. EOF
	{
		var attached string
		in := strings.NewReader("")
		var out bytes.Buffer
		runInteractiveMenu(in, &out, sessions, false, func(name string) {
			attached = name
		})
		if attached != "" {
			t.Errorf("expected no attach on EOF, got %q", attached)
		}
	}
}
