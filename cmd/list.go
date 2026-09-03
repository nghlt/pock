package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nghlt/pock/session"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all active sessions",
	Example: `  # List all active sessions
  pock list
  pock ls`,
	Run: func(cmd *cobra.Command, args []string) {
		sessions, err := session.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(sessions) == 0 {
			fmt.Println("No sessions")
			return
		}

		currentSession := os.Getenv("POCK_SESSION")
		isTTY := session.IsColorEnabled(os.Stdout)

		renderSessionsTable(os.Stdout, sessions, currentSession, isTTY)
	},
}

type tableCell struct {
	plain   string
	display string
}

func renderSessionsTable(w io.Writer, sessions []*session.Session, currentSession string, isTTY bool) {
	headers := []tableCell{
		{plain: "NAME", display: session.ColorBold("NAME", isTTY)},
		{plain: "STATUS", display: session.ColorBold("STATUS", isTTY)},
		{plain: "LAST ACTIVE", display: session.ColorBold("LAST ACTIVE", isTTY)},
		{plain: "COMMAND", display: session.ColorBold("COMMAND", isTTY)},
	}

	rows := make([][]tableCell, len(sessions))
	for i, s := range sessions {
		namePlain := s.Name
		var nameDisplay string
		if s.Name == currentSession {
			namePlain = "* " + s.Name
			nameDisplay = session.ColorGreen(namePlain, isTTY)
		} else {
			nameDisplay = session.ColorCyan(namePlain, isTTY)
		}

		statusPlain := "running"
		statusDisplay := session.ColorGreen(statusPlain, isTTY)

		timePlain := formatRelativeTime(s.LastActive)
		timeDisplay := session.ColorDim(timePlain, isTTY)

		cmdStr := strings.Join(s.Command, " ")
		if cmdStr == "" {
			cmdStr = "(default shell)"
		}
		cmdDisplay := session.ColorDim(cmdStr, isTTY)

		rows[i] = []tableCell{
			{plain: namePlain, display: nameDisplay},
			{plain: statusPlain, display: statusDisplay},
			{plain: timePlain, display: timeDisplay},
			{plain: cmdStr, display: cmdDisplay},
		}
	}

	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = utf8.RuneCountInString(h.plain)
	}
	for _, row := range rows {
		for i, c := range row {
			if width := utf8.RuneCountInString(c.plain); width > colWidths[i] {
				colWidths[i] = width
			}
		}
	}

	printRow := func(row []tableCell) {
		var sb strings.Builder
		for i, c := range row {
			sb.WriteString(c.display)
			if i < len(row)-1 {
				padding := colWidths[i] - utf8.RuneCountInString(c.plain) + 2
				sb.WriteString(strings.Repeat(" ", padding))
			}
		}
		fmt.Fprintln(w, sb.String())
	}

	printRow(headers)
	for _, row := range rows {
		printRow(row)
	}
}

func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
