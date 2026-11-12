package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nghlt/pock/session"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var nonInteractiveFlag bool

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all active sessions",
	Example: `  # List all active sessions
  pock list
  pock ls

  # List sessions without interactive menu
  pock ls --non-interactive`,
	Run: func(cmd *cobra.Command, args []string) {
		sessions, err := session.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		currentSession := os.Getenv("POCK_SESSION")
		isTTY := session.IsColorEnabled(os.Stdout)
		isInteractive := term.IsTerminal(int(os.Stdin.Fd())) &&
			term.IsTerminal(int(os.Stdout.Fd())) &&
			currentSession == "" &&
			!nonInteractiveFlag

		if len(sessions) == 0 {
			if !isInteractive {
				fmt.Println("No sessions")
				return
			}
			fmt.Println("No active sessions.")
			fmt.Println()
			fmt.Printf("%s  %s\n\n", session.ColorBold("0", isTTY), session.ColorDim("(exit, or press Enter)", isTTY))
			runInteractiveMenu(os.Stdin, os.Stdout, sessions, isTTY, func(name string) {
				createAndAttachSession(name, nil, true)
			})
			return
		}

		renderSessionsTable(os.Stdout, sessions, currentSession, isTTY)

		if !isInteractive {
			return
		}

		col0Width := utf8.RuneCountInString(fmt.Sprintf("%d", len(sessions)))
		if col0Width < 1 {
			col0Width = 1
		}
		leadZero := strings.Repeat(" ", col0Width-1)
		fmt.Println()
		fmt.Printf("%s%s  %s\n\n", leadZero, session.ColorBold("0", isTTY), session.ColorDim("(exit, or press Enter)", isTTY))

		runInteractiveMenu(os.Stdin, os.Stdout, sessions, isTTY, func(name string) {
			createAndAttachSession(name, nil, true)
		})
	},
}

type tableCell struct {
	plain   string
	display string
}

func renderSessionsTable(w io.Writer, sessions []*session.Session, currentSession string, isTTY bool) {
	headers := []tableCell{
		{plain: "#", display: session.ColorBold("#", isTTY)},
		{plain: "NAME", display: session.ColorBold("NAME", isTTY)},
		{plain: "STATUS", display: session.ColorBold("STATUS", isTTY)},
		{plain: "LAST ACTIVE", display: session.ColorBold("LAST ACTIVE", isTTY)},
		{plain: "COMMAND", display: session.ColorBold("COMMAND", isTTY)},
	}

	rows := make([][]tableCell, len(sessions))
	for i, s := range sessions {
		numPlain := fmt.Sprintf("%d", i+1)
		numDisplay := session.ColorBold(numPlain, isTTY)

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
			{plain: numPlain, display: numDisplay},
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

	printRow := func(row []tableCell, isHeader bool) {
		var sb strings.Builder
		for i, c := range row {
			if i == 0 && !isHeader {
				// Right-align numbers in the first column
				leadSpaces := colWidths[0] - utf8.RuneCountInString(c.plain)
				if leadSpaces > 0 {
					sb.WriteString(strings.Repeat(" ", leadSpaces))
				}
				sb.WriteString(c.display)
				sb.WriteString("  ")
			} else {
				sb.WriteString(c.display)
				if i < len(row)-1 {
					padding := colWidths[i] - utf8.RuneCountInString(c.plain) + 2
					sb.WriteString(strings.Repeat(" ", padding))
				}
			}
		}
		fmt.Fprintln(w, sb.String())
	}

	printRow(headers, true)
	for _, row := range rows {
		printRow(row, false)
	}
}

func runInteractiveMenu(r io.Reader, w io.Writer, sessions []*session.Session, isTTY bool, onAttach func(name string)) {
	scanner := bufio.NewScanner(r)
	for {
		if len(sessions) == 0 {
			fmt.Fprint(w, session.ColorBold("Enter name to create (or press Enter to exit): ", isTTY))
		} else {
			fmt.Fprint(w, session.ColorBold(fmt.Sprintf("Select session [1-%d] or enter name (Enter to exit): ", len(sessions)), isTTY))
		}

		if !scanner.Scan() {
			fmt.Fprintln(w)
			return
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" || input == "0" {
			return
		}

		if session.IsAllDigits(input) {
			num, err := strconv.Atoi(input)
			if err != nil || num < 1 || num > len(sessions) {
				if len(sessions) == 0 {
					fmt.Fprintf(w, "%s invalid selection %q. Enter a session name or press Enter to exit.\n", session.ColorRed("Error:", isTTY), input)
				} else {
					fmt.Fprintf(w, "%s invalid selection %q. Choose 1-%d, enter a session name, or press Enter to exit.\n", session.ColorRed("Error:", isTTY), input, len(sessions))
				}
				continue
			}
			onAttach(sessions[num-1].Name)
			return
		}

		// Input is a session name
		if err := session.ValidateName(input); err != nil {
			fmt.Fprintf(w, "%s %v\n", session.ColorRed("Error:", isTTY), err)
			continue
		}

		onAttach(input)
		return
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

func init() {
	listCmd.Flags().BoolVarP(&nonInteractiveFlag, "non-interactive", "n", false, "Do not show interactive selection menu")
}
