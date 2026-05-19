package apiagent

const (
	schemaKeyType     = "type"
	schemaKeyProps    = "properties"
	schemaKeyDesc     = "description"
	schemaKeyRequired = "required"

	schemaTypeObject = "object"
	schemaTypeString = "string"

	paramPath = "path"
)

// KvelmoTools returns the tool definitions that API agents expose to models.
// Tool names match the names used in kvelmo's conductor prompts (Write, Read, Edit, etc.)
// so the model can follow instructions like "Use the Write tool to save the file".
func KvelmoTools() []ToolDef {
	return []ToolDef{
		{
			Name:        "Bash",
			Description: "Run a shell command in the working directory. Use for git operations, running tests, installing packages, and other system commands.",
			Parameters: map[string]any{
				schemaKeyType: schemaTypeObject,
				schemaKeyProps: map[string]any{
					"command": map[string]any{
						schemaKeyType: schemaTypeString,
						schemaKeyDesc: "The shell command to execute",
					},
					"timeout": map[string]any{
						schemaKeyType: "integer",
						schemaKeyDesc: "Optional timeout in seconds (default: 120)",
					},
				},
				schemaKeyRequired: []string{"command"},
			},
		},
		{
			Name:        "Read",
			Description: "Read the contents of a file. Returns the file content with line numbers.",
			Parameters: map[string]any{
				schemaKeyType: schemaTypeObject,
				schemaKeyProps: map[string]any{
					paramPath: map[string]any{
						schemaKeyType: schemaTypeString,
						schemaKeyDesc: "Path to the file (relative to working directory or absolute)",
					},
					"offset": map[string]any{
						schemaKeyType: "integer",
						schemaKeyDesc: "Line number to start reading from (1-based)",
					},
					"limit": map[string]any{
						schemaKeyType: "integer",
						schemaKeyDesc: "Maximum number of lines to read",
					},
				},
				schemaKeyRequired: []string{paramPath},
			},
		},
		{
			Name:        "Write",
			Description: "Create or overwrite a file with the given content. Creates parent directories if needed.",
			Parameters: map[string]any{
				schemaKeyType: schemaTypeObject,
				schemaKeyProps: map[string]any{
					paramPath: map[string]any{
						schemaKeyType: schemaTypeString,
						schemaKeyDesc: "Path to the file (relative to working directory or absolute)",
					},
					"content": map[string]any{
						schemaKeyType: schemaTypeString,
						schemaKeyDesc: "Content to write to the file",
					},
				},
				schemaKeyRequired: []string{paramPath, "content"},
			},
		},
		{
			Name:        "Edit",
			Description: "Apply a targeted edit to a file by replacing old_string with new_string. The old_string must be unique in the file.",
			Parameters: map[string]any{
				schemaKeyType: schemaTypeObject,
				schemaKeyProps: map[string]any{
					paramPath: map[string]any{
						schemaKeyType: schemaTypeString,
						schemaKeyDesc: "Path to the file",
					},
					"old_string": map[string]any{
						schemaKeyType: schemaTypeString,
						schemaKeyDesc: "Exact text to find and replace (must be unique in file)",
					},
					"new_string": map[string]any{
						schemaKeyType: schemaTypeString,
						schemaKeyDesc: "Replacement text",
					},
				},
				schemaKeyRequired: []string{paramPath, "old_string", "new_string"},
			},
		},
		{
			Name:        "Glob",
			Description: "Find files matching a glob pattern (e.g., '**/*.go', 'src/**/*.ts').",
			Parameters: map[string]any{
				schemaKeyType: schemaTypeObject,
				schemaKeyProps: map[string]any{
					"pattern": map[string]any{
						schemaKeyType: schemaTypeString,
						schemaKeyDesc: "Glob pattern to match files",
					},
					paramPath: map[string]any{
						schemaKeyType: schemaTypeString,
						schemaKeyDesc: "Directory to search in (default: working directory)",
					},
				},
				schemaKeyRequired: []string{"pattern"},
			},
		},
		{
			Name:        "Grep",
			Description: "Search file contents using a regular expression pattern.",
			Parameters: map[string]any{
				schemaKeyType: schemaTypeObject,
				schemaKeyProps: map[string]any{
					"pattern": map[string]any{
						schemaKeyType: schemaTypeString,
						schemaKeyDesc: "Regular expression pattern to search for",
					},
					paramPath: map[string]any{
						schemaKeyType: schemaTypeString,
						schemaKeyDesc: "File or directory to search in (default: working directory)",
					},
					"glob": map[string]any{
						schemaKeyType: schemaTypeString,
						schemaKeyDesc: "Glob pattern to filter files (e.g., '*.go')",
					},
				},
				schemaKeyRequired: []string{"pattern"},
			},
		},
		{
			Name:        "LS",
			Description: "List the contents of a directory.",
			Parameters: map[string]any{
				schemaKeyType: schemaTypeObject,
				schemaKeyProps: map[string]any{
					paramPath: map[string]any{
						schemaKeyType: schemaTypeString,
						schemaKeyDesc: "Directory path (default: working directory)",
					},
				},
			},
		},
	}
}
