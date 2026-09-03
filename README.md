# 🎒 pock

> A lightweight, zero-config terminal session manager with native scrollback support.

**pock** allows you to detach and reattach terminal sessions effortlessly without the overhead of heavy terminal multiplexers. It stays out of your way and keeps your native terminal scrollback completely functional.

---

## 💡 Why pock?

Traditional multiplexers like `tmux` or `screen` hijack the alternate screen buffer, breaking native scrolling gestures, corrupting mobile SSH rendering, and adding unnecessary keybinding complexity when all you need is persistent sessions.

| Feature | tmux | screen | abduco | 🎒 pock |
|:---|:---:|:---:|:---:|:---:|
| **Detach & Reattach** | ✅ | ✅ | ✅ | ✅ |
| **Multi-client Sharing** | ✅ | ✅ | ✅ | ✅ |
| **Native Terminal Scrollback** | ❌ | ❌ | ❌ | ✅ |
| **Zero Configuration Required** | ❌ | ❌ | ✅ | ✅ |
| **Clean SSH Detach (`?.`)** | ❌ | ❌ | ❌ | ✅ |
| **Dark & Light Theme Adaptive** | ❌ | ❌ | ❌ | ✅ |
| **Single Static Binary** | ❌ | ❌ | ✅ | ✅ |

---

## ✨ Features

- 📜 **Native Scrollback Intact** — Does not enter alternate screen mode; trackpad and terminal scrollback work natively.
- ⚡ **Ultra-low Latency & Memory Safe** — Powered by a zero-allocation circular ring buffer (1MB fixed per session) and single-syscall vectored I/O (`writev`).
- 👥 **Multi-Client Pairing** — Multiple terminals can attach to the same session simultaneously with non-blocking output broadcast.
- 🚪 **Clean Detach (`?.`)** — Detaches with zero leftover characters on your prompt. Perfect for SSH and mobile terminals.
- 🎨 **Dark & Light Theme Colors** — High-contrast ANSI colors tuned for both dark and bright backgrounds, with strict `NO_COLOR` support.
- 🎒 **Lightweight & Self-Contained** — Single binary with minimal dependencies, ready to drop into `/usr/local/bin`.

---

## 🚀 Quick Start

### Create or Attach Sessions

```bash
# Start a new session (auto-named from the current directory)
pock

# Create a session with an explicit name
pock new myproject

# Create a named session running a specific command
pock new myproject python app.py

# Attach to "myproject" if it already exists, or create it if not
pock new -A myproject
```

### Manage Sessions

```bash
# List all active sessions with status and relative time
pock list

# Attach to an existing session (or most recent if name omitted)
pock attach myproject
pock attach

# Terminate a session
pock remove myproject
pock rm myproject

# Terminate all active sessions (with interactive confirmation)
pock clear
pock c
pock clear --force
```

---

## ⌨️ Detaching from a Session

Press **`?.`** (question mark followed by a period) or **`~.`** immediately after hitting <kbd>Enter</kbd>.

```text
[Enter] ?.
[pock: 👋 detached "myproject"]
```

> **Why `?.`?**
> The `?.` escape sequence avoids conflicts with OpenSSH's default `~.` escape key and doesn't interfere with control keys inside interactive applications like `vim`, `nano`, or AI coding agents.

### Custom Detach Keys

You can customize detach keys via command-line flags or environment variables:

```bash
# Pass via CLI flag
pock -d ctrl-a new myproject
pock -d '?.' -d '~.' attach myproject

# Or configure via environment variables
export POCK_DETACH_KEY='?.'
export POCK_DETACH_KEY_1='~.'
export POCK_DETACH_KEY_2='ctrl-a'
```

---

## 📝 Commands & Aliases

| Command | Aliases | Description |
|:---|:---|:---|
| `pock` / `pock new [name] [cmd]` | `n`, `create` | Create a new session (auto-named if omitted) |
| `pock new -A [name]` | `n -A`, `create -A` | Attach to session if it exists, or create it |
| `pock attach [name]` | `a` | Attach to an existing session (default: most recent) |
| `pock list` | `ls` | List active sessions with status & last active time |
| `pock remove <name>` | `rm`, `delete`, `kill` | Terminate a running session |
| `pock clear` | `c` | Terminate all sessions (supports `--force` / `-f`) |
| `pock setup` | — | Configure shell prompt indicator (bash, zsh, fish) |

---

## 🔍 In-Session Indicators

**pock** has no intrusive status bar, but provides two lightweight, opt-in indicators:

### 1. Terminal Window / Tab Title
Pass `-T` or set `POCK_TITLE=1` to display `🎒 {name}` in your terminal tab title while attached:

```bash
pock -T attach myproject
export POCK_TITLE=1
export POCK_TITLE_FORMAT='🎒 [{name}]'
```

### 2. Shell Prompt Indicator (`pock setup`)
Automatically injects an indicator (`[🎒 $POCK_SESSION]`) into your shell configuration:

```bash
pock setup              # Auto-detects current shell and configures indicator
pock setup zsh          # Configure specifically for zsh
pock setup --uninstall  # Cleanly remove indicator configuration
```

---

## 🔧 Environment Variables

| Variable | Description |
|:---|:---|
| `POCK_SESSION` | Set automatically inside sessions to identify the session and prevent nesting |
| `POCK_DETACH_KEY` | Primary detach key sequence (default: `?.`) |
| `POCK_DETACH_KEY_1`, `_2`, ... | Additional detach key sequences |
| `POCK_TITLE` | When set to `1`, updates terminal title with active session name |
| `POCK_TITLE_FORMAT` | Custom terminal title format (`{name}` is replaced by session name) |

---

## 📦 Installation

### Pre-built Binary
Download the pre-compiled binary for your OS and architecture from [GitHub Releases](https://github.com/nghlt/pock/releases).

### Go Install
```bash
go install github.com/nghlt/pock@latest
```

### Build from Source
```bash
git clone git@github.com:nghlt/pock.git
cd pock
make install   # Builds and installs to /usr/local/bin/pock
```

---

## 🙏 Acknowledgments

**pock** was originally forked and inspired by [tuck](https://github.com/rot1024/tuck) created by [@rot1024](https://github.com/rot1024).

Enhancements introduced in **pock**:
- Unified `new` / `create` command UX with `-A, --attach` flag.
- Strict session attachment validation to prevent accidental zombie sessions.
- Theme-adaptive ANSI color system (supporting dark, light, and `NO_COLOR` terminals).
- Zero-allocation circular ring buffer for bounded session scrollback history.
- Vectored I/O syscall optimization (`net.Buffers` / `writev`).
- Non-blocking client broadcast workers.
- Adaptive startup socket connection backoff (~2ms attach speed).
- Clean `?.` escape sequence with lazy-flush buffer.

---

## 📄 License

[MIT License](LICENSE)
