package cmd

import (
	"fmt"
	"os"

	"github.com/nghlt/pock/session"
	"github.com/spf13/cobra"
)

// getDetachKeys returns the detach keys from flags or environment variables
func getDetachKeys() ([]session.DetachKey, error) {
	var keyStrs []string

	// Collect from flags
	keyStrs = append(keyStrs, detachKeyFlags...)

	// Collect from environment variables (POCK_DETACH_KEY, POCK_DETACH_KEY_1, POCK_DETACH_KEY_2, ...)
	if envKey := os.Getenv("POCK_DETACH_KEY"); envKey != "" {
		keyStrs = append(keyStrs, envKey)
	}
	for i := 1; ; i++ {
		envKey := os.Getenv(fmt.Sprintf("POCK_DETACH_KEY_%d", i))
		if envKey == "" {
			break
		}
		keyStrs = append(keyStrs, envKey)
	}

	// Use defaults if none specified
	if len(keyStrs) == 0 {
		return session.DefaultDetachKeys, nil
	}

	// Parse all keys
	var keys []session.DetachKey
	for _, s := range keyStrs {
		key, err := session.ParseDetachKey(s)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

// mustGetDetachKeys returns the detach keys or exits on error
func mustGetDetachKeys() []session.DetachKey {
	keys, err := getDetachKeys()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	return keys
}

// checkNotNested exits if already inside a pock session
func checkNotNested() {
	if s := os.Getenv("POCK_SESSION"); s != "" {
		fmt.Fprintf(os.Stderr, "Error: already inside pock session %q\n", s)
		os.Exit(1)
	}
}

// defaultTitleFormat is used when --title/-T or POCK_TITLE is enabled
// without an explicit format.
const defaultTitleFormat = "🎒 {name}"

// getTitleFormat returns the terminal title format to use, or "" if the
// terminal title indicator is disabled. Precedence: --title-format flag >
// POCK_TITLE_FORMAT env var > (--title flag or POCK_TITLE env var, using
// the default format).
func getTitleFormat() string {
	if titleFormatFlag != "" {
		return titleFormatFlag
	}
	if envFormat := os.Getenv("POCK_TITLE_FORMAT"); envFormat != "" {
		return envFormat
	}
	if titleFlag || os.Getenv("POCK_TITLE") != "" {
		return defaultTitleFormat
	}
	return ""
}

var (
	quietFlag       bool
	detachKeyFlags  []string
	titleFlag       bool
	titleFormatFlag string
)

var rootCmd = &cobra.Command{
	Use:   "pock",
	Short: "A lightweight terminal session manager",
	Long: `pock is a lightweight terminal session manager that allows you to
detach and reattach terminal sessions without screen splitting.

Unlike tmux or screen, pock does not use the alternate screen buffer,
so your terminal's scrollback buffer remains fully functional.`,
	Example: `  # Start a new session in current directory
  pock

  # Create a named session running bash
  pock new myproject bash

  # Attach to an existing session
  pock attach myproject

  # Attach if exists, otherwise create it
  pock new -A myproject

  # List all active sessions
  pock list`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default to "pock new" behavior
		newCmd.Run(cmd, args)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	setupHelpConfig(rootCmd)
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.PersistentFlags().BoolVarP(&quietFlag, "quiet", "q", false, "Suppress status messages")
	rootCmd.PersistentFlags().StringArrayVarP(&detachKeyFlags, "detach-key", "d", nil, "Detach key (e.g., ?., ~., ctrl-a). Can be specified multiple times")
	rootCmd.PersistentFlags().BoolVarP(&titleFlag, "title", "T", false, "Show session name in terminal title while attached (also: POCK_TITLE)")
	rootCmd.PersistentFlags().StringVar(&titleFormatFlag, "title-format", "", "Terminal title format, use {name} as placeholder (implies --title, also: POCK_TITLE_FORMAT)")

	// Allow command arguments with dashes (e.g., "claude --continue")
	newCmd.Flags().SetInterspersed(false)
	rootCmd.Flags().SetInterspersed(false)

	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(attachCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(clearCmd)
}
