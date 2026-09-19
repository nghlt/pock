package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Session represents a pock session
type Session struct {
	Name       string    `json:"name"`
	PID        int       `json:"pid"`
	Command    []string  `json:"command"`
	LastActive time.Time `json:"last_active"`
	BootID     string    `json:"boot_id,omitempty"`
}

// DataDir returns the directory for storing session data
func DataDir() (string, error) {
	if dir := os.Getenv("POCK_DATA_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "pock"), nil
}

// EnsureDataDir creates the data directory if it doesn't exist
func EnsureDataDir() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create data directory: %w", err)
	}
	return dir, nil
}

// SocketPath returns the socket path for a session
func SocketPath(name string) (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".sock"), nil
}

// InfoPath returns the info file path for a session
func InfoPath(name string) (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".json"), nil
}

// ErrorPath returns the error file path for a session
func ErrorPath(name string) (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".err"), nil
}

// Save saves session info to disk
func (s *Session) Save() error {
	path, err := InfoPath(s.Name)
	if err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal session info: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write session info: %w", err)
	}
	return nil
}

// Load loads session info from disk
func Load(name string) (*Session, error) {
	path, err := InfoPath(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read session info: %w", err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session info: %w", err)
	}
	return &s, nil
}

// CurrentBootID returns the system boot ID if available (Linux).
// Returns an empty string if not supported or unavailable.
func CurrentBootID() string {
	data, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// verifyProcessCmdline checks if the process with the given PID corresponds to pock
func verifyProcessCmdline(pid int, sessionName string) bool {
	if pid == os.Getpid() {
		return true
	}
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		// Non-Linux or /proc unavailable, skip cmdline check
		return true
	}
	cmdStr := string(cmdline)
	return strings.Contains(cmdStr, sessionName) || strings.Contains(cmdStr, "pock") || strings.Contains(cmdStr, "tuck")
}

// IsSessionAlive checks whether a session's server process is genuinely running and alive.
func IsSessionAlive(sess *Session) bool {
	if sess == nil || sess.PID <= 0 {
		return false
	}

	// 1. Check Boot ID: if recorded and boot ID changed, it is from a previous boot
	if curBoot := CurrentBootID(); curBoot != "" && sess.BootID != "" && sess.BootID != curBoot {
		return false
	}

	// 2. Check if process exists
	if !isProcessRunning(sess.PID) {
		return false
	}

	// 3. Verify process cmdline (guards against PID reuse across reboot or wrap-around)
	if !verifyProcessCmdline(sess.PID, sess.Name) {
		return false
	}

	// 4. Verify socket file exists
	sockPath, err := SocketPath(sess.Name)
	if err != nil {
		return false
	}
	if _, err := os.Stat(sockPath); err != nil {
		return false
	}

	return true
}

// CleanupStale scans the data directory and removes all stale sessions and orphaned files
// left behind by system reboots, process crashes, or unclean shutdowns.
func CleanupStale() error {
	dir, err := DataDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read data directory: %w", err)
	}

	activeSessions := make(map[string]bool)

	// Phase 1: Check all .json metadata files
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := entry.Name()[:len(entry.Name())-5] // remove .json
		s, err := Load(name)
		if err != nil {
			// Corrupted json, remove stale files
			_ = Remove(name)
			continue
		}
		if !IsSessionAlive(s) {
			// Dead process, previous boot, or missing socket
			_ = Remove(name)
			continue
		}
		activeSessions[name] = true
	}

	// Phase 2: Check for orphaned .sock and .err files with no active session
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext == ".sock" || ext == ".err" {
			name := entry.Name()[:len(entry.Name())-len(ext)]
			if !activeSessions[name] {
				_ = Remove(name)
			}
		}
	}

	return nil
}

// Exists checks if a session exists and is alive.
// If the session is stale (from previous boot or dead process) or an orphaned socket exists,
// it automatically cleans up the stale files and returns false.
func Exists(name string) bool {
	sockPath, err := SocketPath(name)
	if err != nil {
		return false
	}
	if _, err := os.Stat(sockPath); err != nil {
		return false
	}

	// Socket file exists, check if session is actually alive
	sess, err := Load(name)
	if err != nil {
		// Socket exists but no valid json metadata -> stale/orphaned
		_ = Remove(name)
		return false
	}

	if !IsSessionAlive(sess) {
		// Stale session (dead process or previous boot)
		_ = Remove(name)
		return false
	}

	return true
}

// List returns all active sessions, cleaning up any stale ones
func List() ([]*Session, error) {
	if err := CleanupStale(); err != nil {
		return nil, err
	}

	dir, err := DataDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read data directory: %w", err)
	}

	var sessions []*Session
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := entry.Name()[:len(entry.Name())-5] // remove .json
		s, err := Load(name)
		if err != nil {
			continue
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// MostRecent returns the most recently active session
func MostRecent() (*Session, error) {
	sessions, err := List()
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, nil
	}
	most := sessions[0]
	for _, s := range sessions[1:] {
		if s.LastActive.After(most.LastActive) {
			most = s
		}
	}
	return most, nil
}

// Remove removes a session's files
func Remove(name string) error {
	sockPath, _ := SocketPath(name)
	infoPath, _ := InfoPath(name)
	errPath, _ := ErrorPath(name)
	_ = os.Remove(sockPath)
	_ = os.Remove(infoPath)
	_ = os.Remove(errPath)
	return nil
}

// ValidateName checks if a session name is valid
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("session name cannot be empty")
	}
	if strings.ContainsAny(name, "/\\:*?\"<>|\x00 \t\n\r") || strings.HasPrefix(name, ".") {
		return fmt.Errorf("invalid session name %q: must not contain path separators, spaces, or start with '.'", name)
	}
	return nil
}

// isProcessRunning checks if a process with the given PID is running
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}
