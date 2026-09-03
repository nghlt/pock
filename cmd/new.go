package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nghlt/pock/session"
	"github.com/spf13/cobra"
)

var attachFlag bool

var newCmd = &cobra.Command{
	Use:     "new [name] [command...]",
	Aliases: []string{"n", "create"},
	Short:   "Create a new session (auto-named if omitted)",
	Long: `Create a new session with an optional name and command.
If no name is specified, an auto-generated name based on current directory is used.
If no command is specified, the default shell is used.

Use -A / --attach to attach if the session already exists instead of returning an error.
Use ` + "`?.`" + ` (default) or configured detach key to detach.`,
	Example: `  # Auto-named session from current directory
  tuck new

  # Named session with default shell
  tuck new myproject

  # Named session running a specific command
  tuck new myproject python app.py

  # Attach to existing session or create it if not found
  tuck new -A myproject`,
	Run: func(cmd *cobra.Command, args []string) {
		checkNotNested()

		var name string
		var command []string

		if len(args) == 0 {
			name = generateSessionName()
		} else {
			name = args[0]
			command = args[1:]
		}

		createAndAttachSession(name, command, attachFlag)
	},
}

func createAndAttachSession(name string, command []string, attachIfExists bool) {
	isTTY := session.IsColorEnabled(os.Stderr)

	if err := session.ValidateName(name); err != nil {
		fmt.Fprintf(os.Stderr, "%s %v\n", session.ColorRed("Error:", isTTY), err)
		os.Exit(1)
	}

	if session.Exists(name) {
		if attachIfExists {
			detachKeys := mustGetDetachKeys()
			if !quietFlag {
				fmt.Fprintln(os.Stderr, session.FormatStatusMsg("🔗", "attaching to existing", name, session.FormatDetachKeys(detachKeys)+" to detach"))
			}
			if err := session.Attach(name, session.AttachOptions{
				Quiet:            quietFlag,
				SuppressAttached: true,
				DetachKeys:       detachKeys,
				TitleFormat:      getTitleFormat(),
			}); err != nil {
				fmt.Fprintf(os.Stderr, "%s %v\n", session.ColorRed("Error:", isTTY), err)
				os.Exit(1)
			}
			return
		}

		fmt.Fprintf(os.Stderr, "%s session %q already exists\n", session.ColorRed("Error:", isTTY), name)
		fmt.Fprintf(os.Stderr, "%s Use '%s' or '%s' to connect to it.\n",
			session.ColorYellow("Hint:", isTTY),
			session.ColorCyan("pock attach "+name, isTTY),
			session.ColorCyan("pock new -A "+name, isTTY),
		)
		os.Exit(1)
	}

	// Fork to create server process
	if os.Getenv("POCK_SERVER") == "1" {
		// We are the server process
		runServer(name, command)
		return
	}

	// Start server process in background
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	serverArgs := append([]string{"new", name}, command...)
	serverCmd := exec.Command(exe, serverArgs...)
	serverCmd.Env = append(os.Environ(), "POCK_SERVER=1")
	serverCmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	// Capture server stderr for error reporting
	errPath, _ := session.ErrorPath(name)
	if errPath != "" {
		_ = os.Remove(errPath) // Clean up any previous error
	}

	if err := serverCmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		os.Exit(1)
	}

	// Wait for server to start with fast adaptive backoff
	delays := []time.Duration{
		1 * time.Millisecond,
		2 * time.Millisecond,
		5 * time.Millisecond,
		10 * time.Millisecond,
		20 * time.Millisecond,
	}
	startTime := time.Now()
	timeout := 5 * time.Second

	for time.Since(startTime) < timeout {
		if session.Exists(name) {
			break
		}
		// Check if server wrote an error
		if errPath != "" {
			if errData, err := os.ReadFile(errPath); err == nil && len(errData) > 0 {
				_ = os.Remove(errPath)
				fmt.Fprintf(os.Stderr, "Error: %s\n", string(errData))
				os.Exit(1)
			}
		}
		delay := 50 * time.Millisecond
		if len(delays) > 0 {
			delay = delays[0]
			delays = delays[1:]
		}
		time.Sleep(delay)
	}

	if !session.Exists(name) {
		// Check for error file one more time
		if errPath != "" {
			if errData, err := os.ReadFile(errPath); err == nil && len(errData) > 0 {
				_ = os.Remove(errPath)
				fmt.Fprintf(os.Stderr, "Error: %s\n", string(errData))
				os.Exit(1)
			}
		}
		fmt.Fprintf(os.Stderr, "Error: failed to create session (server did not start)\n")
		os.Exit(1)
	}

	// Show created message
	detachKeys := mustGetDetachKeys()
	if !quietFlag {
		fmt.Fprintln(os.Stderr, session.FormatStatusMsg("✨", "created", name, session.FormatDetachKeys(detachKeys)+" to detach"))
	}

	// Attach to the session
	if err := session.Attach(name, session.AttachOptions{
		Quiet:            quietFlag,
		SuppressAttached: true,
		DetachKeys:       detachKeys,
		TitleFormat:      getTitleFormat(),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runServer(name string, command []string) {
	server, err := session.NewServer(name, command)
	if err != nil {
		// Write error to file for client to read
		if errPath, pathErr := session.ErrorPath(name); pathErr == nil {
			_ = os.WriteFile(errPath, []byte(err.Error()), 0600)
		}
		os.Exit(1)
	}
	_ = server.Run()
}

// generateSessionName creates a session name from current directory
func generateSessionName() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "session"
	}
	base := filepath.Base(cwd)
	if base == "" || base == "/" || base == "." {
		base = "session"
	}

	// If base name is available, use it
	if !session.Exists(base) {
		return base
	}

	// Otherwise, append a number
	for i := 1; i < 1000; i++ {
		name := fmt.Sprintf("%s-%d", base, i)
		if !session.Exists(name) {
			return name
		}
	}
	return base
}

func init() {
	newCmd.Flags().BoolVarP(&attachFlag, "attach", "A", false, "Attach to session if it already exists")
}
