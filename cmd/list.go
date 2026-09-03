package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

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

		w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
		fmt.Fprintln(w, session.ColorBold("NAME\tSTATUS\tLAST ACTIVE\tCOMMAND", isTTY))
		for _, s := range sessions {
			nameDisplay := s.Name
			if s.Name == currentSession {
				nameDisplay = session.ColorGreen("* "+s.Name, isTTY)
			} else {
				nameDisplay = session.ColorCyan(s.Name, isTTY)
			}
			statusDisplay := session.ColorGreen("running", isTTY)
			timeDisplay := session.ColorDim(formatRelativeTime(s.LastActive), isTTY)
			cmdStr := strings.Join(s.Command, " ")
			if cmdStr == "" {
				cmdStr = "(default shell)"
			}
			cmdDisplay := session.ColorDim(cmdStr, isTTY)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", nameDisplay, statusDisplay, timeDisplay, cmdDisplay)
		}
		_ = w.Flush()
	},
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
