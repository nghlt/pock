package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Set by goreleaser via ldflags for tagged releases; defaults to the
// current development version otherwise.
var (
	Version = "0.2.1"
	Commit  = "unknown"
)

var versionCmd = &cobra.Command{
	Use:    "version",
	Short:  "Show version information",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("pock %s (%s)\n", Version, Commit)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	// Enable `pock --version` / `pock -v` in addition to `pock version`,
	// using the same output format.
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate(fmt.Sprintf("pock %s (%s)\n", Version, Commit))
}
