package help

import (
	"github.com/spf13/cobra"
)

// cachedContext holds the lazily-loaded help context.
// It's loaded once per help invocation.
var cachedContext *HelpContext

// GetHelpContext returns the cached help context, loading it if necessary.
// This is called by template functions during help rendering.
func GetHelpContext() *HelpContext {
	if cachedContext == nil {
		cachedContext = LoadContext()
	}
	return cachedContext
}

// ResetContext clears the cached context.
// Call this before rendering help to ensure fresh context.
func ResetContext() {
	cachedContext = nil
}

// FilterAvailable returns commands that are available in the current context.
func FilterAvailable(commands []*cobra.Command, ctx *HelpContext) []*cobra.Command {
	var available []*cobra.Command
	for _, cmd := range commands {
		if cmd.IsAvailableCommand() && IsAvailable(cmd.Name(), ctx) {
			available = append(available, cmd)
		}
	}
	return available
}

// FilterUnavailable returns commands that are not available in the current context.
func FilterUnavailable(commands []*cobra.Command, ctx *HelpContext) []*cobra.Command {
	var unavailable []*cobra.Command
	for _, cmd := range commands {
		if cmd.IsAvailableCommand() && !IsAvailable(cmd.Name(), ctx) {
			unavailable = append(unavailable, cmd)
		}
	}
	return unavailable
}

// UnavailableReason returns the reason why a command is unavailable.
// This is used in the help template to show hints.
func UnavailableReason(cmdName string) string {
	return GetReason(cmdName)
}

// RegisterTemplateFuncs registers the help template functions with Cobra.
// Call this once during initialization.
func RegisterTemplateFuncs() {
	cobra.AddTemplateFunc("helpContext", GetHelpContext)
	cobra.AddTemplateFunc("filterAvailable", FilterAvailable)
	cobra.AddTemplateFunc("filterUnavailable", FilterUnavailable)
	cobra.AddTemplateFunc("unavailableReason", UnavailableReason)
}

// SetupContextualHelp configures a command to use contextual help.
// It sets the custom usage template and registers template functions.
func SetupContextualHelp(cmd *cobra.Command) {
	// Reset context to ensure fresh load
	ResetContext()

	// Register template functions
	RegisterTemplateFuncs()

	// Set custom usage template
	cmd.SetUsageTemplate(ContextualUsageTemplate)
}
