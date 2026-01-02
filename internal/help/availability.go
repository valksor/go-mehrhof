package help

// CommandRule defines availability criteria for a command.
type CommandRule struct {
	// Available returns true if the command can be run in the given context.
	Available func(*HelpContext) bool
	// Reason explains why the command is unavailable (shown in help).
	Reason string
}

// commandRules maps command names to their availability rules.
var commandRules = map[string]CommandRule{
	// Always available commands
	"start":          {Available: always, Reason: ""},
	"auto":           {Available: always, Reason: ""},
	"list":           {Available: always, Reason: ""},
	"init":           {Available: always, Reason: ""},
	"config":         {Available: always, Reason: ""},
	"templates":      {Available: always, Reason: ""},
	"providers":      {Available: always, Reason: ""},
	"agents":         {Available: always, Reason: ""},
	"plugins":        {Available: always, Reason: ""},
	"workflow":       {Available: always, Reason: ""},
	"version":        {Available: always, Reason: ""},
	"update":         {Available: always, Reason: ""},
	"completion":     {Available: always, Reason: ""},
	"provider-login": {Available: always, Reason: ""},
	"help":           {Available: always, Reason: ""},

	// Plan has --standalone mode, so it's always available
	"plan": {Available: always, Reason: ""},

	// Commands that need an active task
	"status":   {Available: needsActiveTask, Reason: "needs active task"},
	"guide":    {Available: needsActiveTask, Reason: "needs active task"},
	"continue": {Available: needsActiveTask, Reason: "needs active task"},
	"cost":     {Available: needsActiveTask, Reason: "needs active task"},
	"note":     {Available: needsActiveTask, Reason: "needs active task"},
	"abandon":  {Available: needsActiveTask, Reason: "needs active task"},
	"answer":   {Available: needsActiveTask, Reason: "needs active task"},

	// Commands that need specifications
	"implement": {Available: needsSpecifications, Reason: "needs specifications"},
	"review":    {Available: needsSpecifications, Reason: "needs specifications"},
	"finish":    {Available: needsSpecifications, Reason: "needs specifications"},

	// Commands that need git
	"undo": {Available: needsGit, Reason: "needs git task"},
	"redo": {Available: needsGit, Reason: "needs git task"},
}

// Availability check functions

func always(_ *HelpContext) bool {
	return true
}

func needsActiveTask(ctx *HelpContext) bool {
	return ctx.HasActiveTask
}

func needsSpecifications(ctx *HelpContext) bool {
	return ctx.HasSpecifications
}

func needsGit(ctx *HelpContext) bool {
	return ctx.HasActiveTask && ctx.UseGit
}

// IsAvailable checks if a command is available in the given context.
func IsAvailable(cmdName string, ctx *HelpContext) bool {
	rule, ok := commandRules[cmdName]
	if !ok {
		// Unknown commands are always available
		return true
	}

	return rule.Available(ctx)
}

// GetReason returns the reason why a command is unavailable.
func GetReason(cmdName string) string {
	rule, ok := commandRules[cmdName]
	if !ok {
		return ""
	}

	return rule.Reason
}
