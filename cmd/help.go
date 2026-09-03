package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nghlt/pock/session"
	"github.com/spf13/cobra"
)

// setupHelpConfig configures beautiful, theme-adaptive colored help for all tuck commands
func setupHelpConfig(cmd *cobra.Command) {
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		renderCustomHelp(c, os.Stdout)
	})
}

func renderCustomHelp(cmd *cobra.Command, w io.Writer) {
	enable := session.IsColorEnabled(os.Stdout)

	sectionTitle := func(title string) string {
		return session.ColorBold(session.ColorYellow(title, enable), enable)
	}

	// 1. Header / Description
	if cmd.Parent() == nil {
		fmt.Fprintf(w, "%s %s - %s\n\n",
			"🎒",
			session.ColorBold(session.ColorCyan("pock", enable), enable),
			session.ColorDim("A lightweight terminal session manager", enable),
		)
	}

	longOrShort := cmd.Long
	if longOrShort == "" {
		longOrShort = cmd.Short
	}
	if longOrShort != "" {
		fmt.Fprintf(w, "%s\n\n", strings.TrimSpace(longOrShort))
	}

	// 2. Usage
	fmt.Fprintln(w, sectionTitle("USAGE:"))
	if cmd.Runnable() {
		fmt.Fprintf(w, "  %s\n", formatUsageLine(cmd.UseLine(), enable))
	}
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(w, "  %s %s\n",
			session.ColorCyan(cmd.CommandPath(), enable),
			session.ColorDim("[command]", enable),
		)
	}

	// 3. Aliases
	if len(cmd.Aliases) > 0 {
		fmt.Fprintf(w, "\n%s\n  %s\n",
			sectionTitle("ALIASES:"),
			session.ColorCyan(strings.Join(cmd.Aliases, ", "), enable),
		)
	}

	// 4. Subcommands
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(w, "\n%s\n", sectionTitle("COMMANDS:"))
		commands := cmd.Commands()
		maxLen := 0
		for _, c := range commands {
			if !c.IsAvailableCommand() || c.Hidden || c.Name() == "help" || c.Name() == "completion" {
				continue
			}
			if len(c.Name()) > maxLen {
				maxLen = len(c.Name())
			}
		}

		for _, c := range commands {
			if !c.IsAvailableCommand() || c.Hidden || c.Name() == "help" || c.Name() == "completion" {
				continue
			}
			padding := strings.Repeat(" ", maxLen-len(c.Name())+2)
			aliasStr := ""
			if len(c.Aliases) > 0 {
				aliasStr = " " + session.ColorDim(fmt.Sprintf("[aliases: %s]", strings.Join(c.Aliases, ", ")), enable)
			}
			fmt.Fprintf(w, "  %s%s%s%s\n",
				session.ColorBold(session.ColorGreen(c.Name(), enable), enable),
				padding,
				c.Short,
				aliasStr,
			)
		}
	}

	// 5. Flags
	localFlags := cmd.LocalFlags()
	if localFlags.HasAvailableFlags() {
		fmt.Fprintf(w, "\n%s\n", sectionTitle("FLAGS:"))
		fmt.Fprint(w, formatFlagUsages(localFlags.FlagUsages(), enable))
	}

	// 6. Inherited Flags
	inheritedFlags := cmd.InheritedFlags()
	if inheritedFlags.HasAvailableFlags() {
		fmt.Fprintf(w, "\n%s\n", sectionTitle("GLOBAL FLAGS:"))
		fmt.Fprint(w, formatFlagUsages(inheritedFlags.FlagUsages(), enable))
	}

	// 7. Examples
	if cmd.Example != "" {
		fmt.Fprintf(w, "\n%s\n", sectionTitle("EXAMPLES:"))
		for _, line := range strings.Split(strings.TrimSpace(cmd.Example), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				fmt.Fprintf(w, "  %s\n", session.ColorDim(trimmed, enable))
			} else if trimmed != "" {
				fmt.Fprintf(w, "  %s\n", formatCommandLine(trimmed, enable))
			} else {
				fmt.Fprintln(w)
			}
		}
	}

	// 8. Footer hint
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(w, "\n%s\n  Use '%s' for more information about a command.\n",
			sectionTitle("LEARN MORE:"),
			session.ColorCyan(cmd.CommandPath()+" [command] --help", enable),
		)
	}
}

// formatUsageLine colors the usage line
func formatUsageLine(useLine string, enable bool) string {
	parts := strings.Split(useLine, " ")
	for i, part := range parts {
		if i == 0 {
			parts[i] = session.ColorCyan(part, enable)
		} else if strings.HasPrefix(part, "[") && strings.HasSuffix(part, "]") {
			parts[i] = session.ColorDim(part, enable)
		} else if strings.HasPrefix(part, "<") && strings.HasSuffix(part, ">") {
			parts[i] = session.ColorYellow(part, enable)
		} else {
			parts[i] = session.ColorBold(part, enable)
		}
	}
	return strings.Join(parts, " ")
}

// formatCommandLine colors example command lines
func formatCommandLine(cmdLine string, enable bool) string {
	parts := strings.Split(cmdLine, " ")
	for i, part := range parts {
		if i == 0 && (part == "pock" || part == "tuck") {
			parts[i] = session.ColorBold(session.ColorCyan(part, enable), enable)
		} else if strings.HasPrefix(part, "-") {
			parts[i] = session.ColorCyan(part, enable)
		}
	}
	return strings.Join(parts, " ")
}

// formatFlagUsages colorizes Cobra flag usage output
func formatFlagUsages(usages string, enable bool) string {
	if !enable || usages == "" {
		return usages
	}

	var sb strings.Builder
	lines := strings.Split(strings.TrimRight(usages, "\n"), "\n")
	for _, line := range lines {
		if len(line) < 4 {
			sb.WriteString(line + "\n")
			continue
		}

		// Look for flag prefix indentation
		leadingSpaces := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := strings.TrimLeft(line, " ")

		// Look for separation between flag specification and description (at least 2 spaces or tab)
		descIdx := strings.Index(trimmed, "   ")
		if descIdx == -1 {
			descIdx = strings.Index(trimmed, "  ")
		}

		if descIdx > 0 {
			flagPart := trimmed[:descIdx]
			descPart := trimmed[descIdx:]

			// Colorize flag specification: flags in cyan, value types in dim
			coloredFlagPart := colorizeFlagSpec(flagPart, enable)

			sb.WriteString(strings.Repeat(" ", leadingSpaces))
			sb.WriteString(coloredFlagPart)
			sb.WriteString(descPart)
			sb.WriteString("\n")
		} else {
			sb.WriteString(line + "\n")
		}
	}
	return sb.String()
}

func colorizeFlagSpec(flagSpec string, enable bool) string {
	words := strings.Split(flagSpec, " ")
	for i, word := range words {
		if strings.HasPrefix(word, "-") {
			words[i] = session.ColorCyan(word, enable)
		} else if word != "" {
			words[i] = session.ColorDim(word, enable)
		}
	}
	return strings.Join(words, " ")
}
