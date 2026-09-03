package session

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// IsColorEnabled checks whether ANSI color output should be enabled for f
func IsColorEnabled(f *os.File) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// Color helpers crafted for excellent contrast on BOTH dark and light terminal backgrounds
func ColorBold(s string, enable bool) string {
	if !enable {
		return s
	}
	return "\033[1m" + s + "\033[0m"
}

func ColorCyan(s string, enable bool) string {
	if !enable {
		return s
	}
	return "\033[36m" + s + "\033[0m"
}

func ColorGreen(s string, enable bool) string {
	if !enable {
		return s
	}
	return "\033[32m" + s + "\033[0m"
}

func ColorYellow(s string, enable bool) string {
	if !enable {
		return s
	}
	return "\033[33m" + s + "\033[0m"
}

func ColorRed(s string, enable bool) string {
	if !enable {
		return s
	}
	return "\033[31m" + s + "\033[0m"
}

func ColorDim(s string, enable bool) string {
	if !enable {
		return s
	}
	return "\033[2m" + s + "\033[0m"
}

// FormatStatusMsg formats tuck status banners nicely for dark and light backgrounds
func FormatStatusMsg(icon, action, name, extra string) string {
	enable := IsColorEnabled(os.Stderr)
	prefix := ColorCyan("["+AppName+":", enable)
	act := ColorGreen(icon+" "+action, enable)
	nm := ColorBold(fmt.Sprintf("%q", name), enable)
	closeBracket := ColorCyan("]", enable)

	if extra != "" {
		ext := ColorDim("("+extra+")", enable)
		return fmt.Sprintf("%s %s %s %s%s", prefix, act, nm, ext, closeBracket)
	}
	return fmt.Sprintf("%s %s %s%s", prefix, act, nm, closeBracket)
}
