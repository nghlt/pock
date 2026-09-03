package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/nghlt/pock/session"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove a session and terminate its process",
	Long:    `Remove a session by name. This will terminate the running process.`,
	Example: `  # Remove a session by name
  pock remove myproject
  pock rm myproject`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		isTTY := session.IsColorEnabled(os.Stderr)
		outTTY := session.IsColorEnabled(os.Stdout)

		sess, err := session.Load(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s session %q does not exist\n", session.ColorRed("Error:", isTTY), name)
			os.Exit(1)
		}

		// Kill the server process
		if sess.PID > 0 {
			_ = syscall.Kill(sess.PID, syscall.SIGTERM)
		}

		// Remove session files
		_ = session.Remove(name)
		fmt.Printf("Session %s removed\n", session.ColorBold(fmt.Sprintf("%q", name), outTTY))
	},
}
