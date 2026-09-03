package cmd

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/nghlt/pock/session"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var clearForceFlag bool

var clearCmd = &cobra.Command{
	Use:     "clear",
	Aliases: []string{"c"},
	Short:   "Delete all sessions (terminates all processes)",
	Long:    `Delete all sessions. This will terminate all running processes.`,
	Example: `  # Prompts for confirmation before terminating
  pock clear

  # Force termination without prompt
  pock clear --force
  pock clear -f`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		isTTY := session.IsColorEnabled(os.Stdout)
		errTTY := session.IsColorEnabled(os.Stderr)

		sessions, err := session.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s %v\n", session.ColorRed("Error:", errTTY), err)
			os.Exit(1)
		}

		if len(sessions) == 0 {
			fmt.Println("No sessions to clear")
			return
		}

		// Protect against accidental clearance without --force flag
		if !clearForceFlag {
			if term.IsTerminal(int(os.Stdin.Fd())) {
				fmt.Printf("%s Are you sure you want to terminate all %s active session(s)? [y/N]: ",
					session.ColorYellow("⚠️ ", isTTY),
					session.ColorBold(fmt.Sprintf("%d", len(sessions)), isTTY),
				)
				var response string
				_, _ = fmt.Scanln(&response)
				response = strings.TrimSpace(strings.ToLower(response))
				if response != "y" && response != "yes" {
					fmt.Println("Aborted.")
					return
				}
			} else {
				fmt.Fprintf(os.Stderr, "%s 'pock clear' requires --force (-f) flag in non-interactive mode\n", session.ColorRed("Error:", errTTY))
				os.Exit(1)
			}
		}

		for _, sess := range sessions {
			// Kill the server process
			if sess.PID > 0 {
				_ = syscall.Kill(sess.PID, syscall.SIGTERM)
			}

			// Remove session files
			_ = session.Remove(sess.Name)
			fmt.Printf("Session %s deleted\n", session.ColorBold(fmt.Sprintf("%q", sess.Name), isTTY))
		}

		fmt.Printf("%s %d session(s)\n", session.ColorGreen("Cleared", isTTY), len(sessions))
	},
}

func init() {
	clearCmd.Flags().BoolVarP(&clearForceFlag, "force", "f", false, "Force delete all sessions without confirmation prompt")
}
