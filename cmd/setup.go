package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/nghlt/pock/session"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	markerStart = "# >>> pock indicator >>>"
	markerEnd   = "# <<< pock indicator <<<"
)

var (
	uninstallFlag bool
	noReloadFlag  bool
)

var setupCmd = &cobra.Command{
	Use:   "setup [bash|zsh|fish]",
	Short: "Configure shell prompt indicator for pock sessions",
	Long: `Configure your shell prompt to show a visual indicator when inside a pock session.
Automatically detects your current shell if not specified and refreshes configuration.
Use --uninstall to remove the configuration.`,
	Example: `  # Auto-detect current shell and configure prompt
  pock setup

  # Configure specifically for zsh
  pock setup zsh

  # Remove pock prompt configuration
  pock setup --uninstall`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		shellType := ""
		if len(args) > 0 {
			shellType = strings.ToLower(args[0])
		} else {
			shellType = detectShell()
		}

		isTTY := session.IsColorEnabled(os.Stdout)
		errTTY := session.IsColorEnabled(os.Stderr)

		if shellType == "" {
			fmt.Fprintf(os.Stderr, "%s unable to detect shell. Please specify one of: bash, zsh, fish\n", session.ColorRed("Error:", errTTY))
			os.Exit(1)
		}

		rcFile, snippet, err := getShellConfig(shellType)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s %v\n", session.ColorRed("Error:", errTTY), err)
			os.Exit(1)
		}

		if uninstallFlag {
			if err := removeConfig(rcFile); err != nil {
				fmt.Fprintf(os.Stderr, "%s removing configuration: %v\n", session.ColorRed("Error", errTTY), err)
				os.Exit(1)
			}
			fmt.Printf("✅ %s pock indicator from %s\n", session.ColorGreen("Removed", isTTY), session.ColorCyan(rcFile, isTTY))
			if !noReloadFlag {
				reloadShell()
			}
			fmt.Printf("Please reload your shell: %s\n", session.ColorCyan("source "+rcFile, isTTY))
			return
		}

		if err := addConfig(rcFile, snippet); err != nil {
			fmt.Fprintf(os.Stderr, "%s configuring shell: %v\n", session.ColorRed("Error", errTTY), err)
			os.Exit(1)
		}

		fmt.Printf("✅ %s pock indicator in %s\n", session.ColorGreen("Configured", isTTY), session.ColorCyan(rcFile, isTTY))
		if !noReloadFlag {
			reloadShell()
		}
		fmt.Printf("To apply changes immediately, run:\n  %s\n", session.ColorCyan("source "+rcFile, isTTY))
	},
}

func detectShell() string {
	shellEnv := os.Getenv("SHELL")
	if shellEnv != "" {
		base := filepath.Base(shellEnv)
		switch base {
		case "bash":
			return "bash"
		case "zsh":
			return "zsh"
		case "fish":
			return "fish"
		}
	}

	home, err := os.UserHomeDir()
	if err == nil {
		if _, err := os.Stat(filepath.Join(home, ".zshrc")); err == nil {
			return "zsh"
		}
		if _, err := os.Stat(filepath.Join(home, ".bashrc")); err == nil {
			return "bash"
		}
	}
	return "bash"
}

func getShellConfig(shellType string) (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to get home directory: %w", err)
	}

	switch shellType {
	case "bash":
		rcPath := filepath.Join(home, ".bashrc")
		snippet := fmt.Sprintf(`%s
if [ -n "$POCK_SESSION" ]; then
  PS1="\[\e[1;34m\][🎒 $POCK_SESSION]\[\e[0m\] $PS1"
fi
%s`, markerStart, markerEnd)
		return rcPath, snippet, nil

	case "zsh":
		rcPath := filepath.Join(home, ".zshrc")
		snippet := fmt.Sprintf(`%s
if [ -n "$POCK_SESSION" ]; then
  PS1="%%F{blue}[🎒 $POCK_SESSION]%%f $PS1"
fi
%s`, markerStart, markerEnd)
		return rcPath, snippet, nil

	case "fish":
		confDir := filepath.Join(home, ".config", "fish", "conf.d")
		_ = os.MkdirAll(confDir, 0755)
		rcPath := filepath.Join(confDir, "pock.fish")
		snippet := fmt.Sprintf(`%s
if set -q POCK_SESSION
  functions -c fish_prompt __original_fish_prompt 2>/dev/null
  function fish_prompt
    set_color blue
    echo -n "[🎒 $POCK_SESSION] "
    set_color normal
    if functions -q __original_fish_prompt
      __original_fish_prompt
    end
  end
end
%s`, markerStart, markerEnd)
		return rcPath, snippet, nil

	default:
		return "", "", fmt.Errorf("unsupported shell: %q (supported: bash, zsh, fish)", shellType)
	}
}

func addConfig(rcFile, snippet string) error {
	var content string
	data, err := os.ReadFile(rcFile)
	if err == nil {
		content = string(data)
	} else if !os.IsNotExist(err) {
		return err
	}

	// Check if already present
	startIdx := strings.Index(content, markerStart)
	endIdx := strings.Index(content, markerEnd)

	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		// Replace existing block
		before := content[:startIdx]
		after := content[endIdx+len(markerEnd):]
		newContent := strings.TrimRight(before, "\n") + "\n\n" + snippet + "\n" + strings.TrimLeft(after, "\n")
		return os.WriteFile(rcFile, []byte(strings.TrimSpace(newContent)+"\n"), 0644)
	}

	// Append to file
	newContent := content
	if len(newContent) > 0 && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += "\n" + snippet + "\n"
	return os.WriteFile(rcFile, []byte(newContent), 0644)
}

func removeConfig(rcFile string) error {
	data, err := os.ReadFile(rcFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	content := string(data)
	startIdx := strings.Index(content, markerStart)
	endIdx := strings.Index(content, markerEnd)

	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		return nil // Not found
	}

	before := content[:startIdx]
	after := content[endIdx+len(markerEnd):]
	newContent := strings.TrimRight(before, "\n") + "\n" + strings.TrimLeft(after, "\n")
	return os.WriteFile(rcFile, []byte(strings.TrimSpace(newContent)+"\n"), 0644)
}

func reloadShell() {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	bin, err := exec.LookPath(shell)
	if err != nil {
		return
	}
	fmt.Println("🔄 Refreshing shell configuration...")
	_ = syscall.Exec(bin, []string{shell}, os.Environ())
}

func init() {
	setupCmd.Flags().BoolVarP(&uninstallFlag, "uninstall", "u", false, "Remove pock shell prompt configuration")
	setupCmd.Flags().BoolVar(&noReloadFlag, "no-reload", false, "Do not automatically reload shell")
	rootCmd.AddCommand(setupCmd)
}
