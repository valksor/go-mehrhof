package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// CommandMatch represents a command that matched a prefix query.
type CommandMatch struct {
	Command *cobra.Command
	Name    string
}

// DisambiguateCommand finds commands matching the given prefix.
// Returns the matching command if exactly one match, or nil with suggestion names.
func DisambiguateCommand(root *cobra.Command, prefix string) (*cobra.Command, []string) {
	if prefix == "" {
		return nil, nil
	}

	var matches []*cobra.Command
	var names []string

	for _, cmd := range root.Commands() {
		if cmd.Hidden {
			continue
		}

		// Exact name match
		if cmd.Name() == prefix {
			return cmd, nil
		}

		// Exact alias match
		if slices.Contains(cmd.Aliases, prefix) {
			return cmd, nil
		}

		// Prefix match on command name
		if strings.HasPrefix(cmd.Name(), prefix) {
			matches = append(matches, cmd)
			names = append(names, cmd.Name())

			continue
		}

		// Prefix match on aliases
		for _, alias := range cmd.Aliases {
			if strings.HasPrefix(alias, prefix) {
				matches = append(matches, cmd)
				names = append(names, cmd.Name())

				break
			}
		}
	}

	if len(matches) == 1 {
		return matches[0], nil
	}

	return nil, names
}

// FindPrefixMatches returns all commands matching the given prefix.
func FindPrefixMatches(root *cobra.Command, prefix string) []CommandMatch {
	if prefix == "" {
		return nil
	}

	var matches []CommandMatch

	for _, cmd := range root.Commands() {
		if cmd.Hidden {
			continue
		}

		if cmd.Name() == prefix {
			return []CommandMatch{{Command: cmd, Name: cmd.Name()}}
		}

		if strings.HasPrefix(cmd.Name(), prefix) {
			matches = append(matches, CommandMatch{Command: cmd, Name: cmd.Name()})
		}
	}

	return matches
}

// FormatAmbiguousError formats a user-friendly error when multiple commands match.
func FormatAmbiguousError(prefix string, names []string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Ambiguous command %q. Did you mean one of:\n", prefix)

	for _, name := range names {
		fmt.Fprintf(&b, "  %s\n", name)
	}

	return b.String()
}

// IsInteractive returns true if stdin is a terminal.
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
