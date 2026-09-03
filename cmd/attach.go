package cmd

import (
	"fmt"
	"os"

	"github.com/nghlt/pock/session"
	"github.com/spf13/cobra"
)

var attachCmd = &cobra.Command{
	Use:     "attach [name]",
	Aliases: []string{"a"},
	Short:   "Attach to an existing session",
	Long: `Attach to an existing session with the given name.
If no name is specified, attaches to the most recently active session.

Use ` + "`?.`" + ` (default) or configured detach key to detach.`,
	Example: `  # Attach to the most recently active session
  pock attach

  # Attach to a specific session by name
  pock attach myproject`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		checkNotNested()

		isTTY := session.IsColorEnabled(os.Stderr)

		var name string
		if len(args) == 0 {
			// Attach to most recent session
			s, err := session.MostRecent()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s %v\n", session.ColorRed("Error:", isTTY), err)
				os.Exit(1)
			}
			if s == nil {
				fmt.Fprintf(os.Stderr, "%s No active sessions found.\n", session.ColorYellow("Info:", isTTY))
				fmt.Fprintf(os.Stderr, "%s Run '%s' to start a new session.\n",
					session.ColorYellow("Hint:", isTTY),
					session.ColorCyan("pock new", isTTY),
				)
				os.Exit(1)
			}
			name = s.Name
		} else {
			name = args[0]
			if err := session.ValidateName(name); err != nil {
				fmt.Fprintf(os.Stderr, "%s %v\n", session.ColorRed("Error:", isTTY), err)
				os.Exit(1)
			}
			if !session.Exists(name) {
				fmt.Fprintf(os.Stderr, "%s session %q does not exist\n", session.ColorRed("Error:", isTTY), name)
				fmt.Fprintf(os.Stderr, "%s Run '%s' to create it, or '%s' to view active sessions.\n",
					session.ColorYellow("Hint:", isTTY),
					session.ColorCyan("pock new "+name, isTTY),
					session.ColorCyan("pock list", isTTY),
				)
				os.Exit(1)
			}
		}

		if err := session.Attach(name, session.AttachOptions{
			Quiet:       quietFlag,
			DetachKeys:  mustGetDetachKeys(),
			TitleFormat: getTitleFormat(),
		}); err != nil {
			fmt.Fprintf(os.Stderr, "%s %v\n", session.ColorRed("Error:", isTTY), err)
			os.Exit(1)
		}
	},
}
