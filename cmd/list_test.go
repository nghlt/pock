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
