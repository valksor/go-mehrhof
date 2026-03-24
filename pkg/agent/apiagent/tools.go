package apiagent

// KvelmoTools returns the tool definitions that API agents expose to models.
// Tool names match the names used in kvelmo's conductor prompts (Write, Read, Edit, etc.)
// so the model can follow instructions like "Use the Write tool to save the file".
func KvelmoTools() []ToolDef {
	return []ToolDef{
		{
			Name:        "Bash",
			Description: "Run a shell command in the working directory. Use for git operations, running tests, installing packages, and other system commands.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "The shell command to execute",
					},
					"timeout": map[string]any{
						"type":        "integer",
						"description": "Optional timeout in seconds (default: 120)",
					},
				},
				"required": []string{"command"},
			},
		},
		{
			Name:        "Read",
			Description: "Read the contents of a file. Returns the file content with line numbers.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Path to the file (relative to working directory or absolute)",
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Line number to start reading from (1-based)",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of lines to read",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "Write",
			Description: "Create or overwrite a file with the given content. Creates parent directories if needed.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Path to the file (relative to working directory or absolute)",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "Content to write to the file",
					},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "Edit",
			Description: "Apply a targeted edit to a file by replacing old_string with new_string. The old_string must be unique in the file.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Path to the file",
					},
					"old_string": map[string]any{
						"type":        "string",
						"description": "Exact text to find and replace (must be unique in file)",
					},
					"new_string": map[string]any{
						"type":        "string",
						"description": "Replacement text",
					},
				},
				"required": []string{"path", "old_string", "new_string"},
			},
		},
		{
			Name:        "Glob",
			Description: "Find files matching a glob pattern (e.g., '**/*.go', 'src/**/*.ts').",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "Glob pattern to match files",
					},
					"path": map[string]any{
						"type":        "string",
						"description": "Directory to search in (default: working directory)",
					},
				},
				"required": []string{"pattern"},
			},
		},
		{
			Name:        "Grep",
			Description: "Search file contents using a regular expression pattern.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "Regular expression pattern to search for",
					},
					"path": map[string]any{
						"type":        "string",
						"description": "File or directory to search in (default: working directory)",
					},
					"glob": map[string]any{
						"type":        "string",
						"description": "Glob pattern to filter files (e.g., '*.go')",
					},
				},
				"required": []string{"pattern"},
			},
		},
		{
			Name:        "LS",
			Description: "List the contents of a directory.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Directory path (default: working directory)",
					},
				},
			},
		},
	}
}
