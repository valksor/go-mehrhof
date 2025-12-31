package help

// ContextualHelpTemplate returns the custom help template for contextual command display.
// It separates commands into "Available Now" and "Other Commands" sections.
const ContextualHelpTemplate = `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

// ContextualUsageTemplate returns the custom usage template for contextual command display.
const ContextualUsageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$ctx := helpContext}}{{$available := filterAvailable .Commands $ctx}}{{$unavailable := filterUnavailable .Commands $ctx}}{{if $available}}

Available Now:{{range $available}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{if $unavailable}}

Other Commands:{{range $unavailable}}
  {{rpad .Name .NamePadding }} {{.Short}}{{with unavailableReason .Name}} ({{.}}){{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
