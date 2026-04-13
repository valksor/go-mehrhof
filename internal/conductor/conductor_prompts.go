package conductor

import (
	"fmt"
	"strings"
)

func (c *Conductor) buildImplementPrompt() string {
	wu := c.workUnit

	// Format specifications as readable list instead of Go slice notation
	specs := ""
	if len(wu.Specifications) > 0 {
		specStr := strings.Join(wu.Specifications, "\n- ")
		specs = "\n\nSpecifications:\n- " + specStr
	}

	hierarchySection := buildHierarchySection(wu.Hierarchy)

	// When implementing without specs (skip-plan), emphasize the description as the sole guide
	header := "Implement the following task based on the specification:"
	if len(wu.Specifications) == 0 {
		header = "Implement the following task directly from the description (planning was skipped):"
	}

	prompt := fmt.Sprintf(`%s

Title: %s
Description: %s
%s%s
%s
Please implement the code following the plan. Create all necessary files and make required modifications.
Commit your changes with meaningful commit messages.
`, header, wu.Title, wu.Description, hierarchySection, specs, browserToolsSection())

	prompt += c.buildProjectCommandsSection()
	prompt += c.buildGitConventionInstructions()

	return prompt
}

// buildProjectCommandsSection returns a prompt section listing discovered project
// commands (Makefile targets, npm/bun scripts, etc.) so the agent knows what tools
// are available. Returns an empty string when no commands were discovered.
func (c *Conductor) buildProjectCommandsSection() string {
	if c.varPool == nil {
		return ""
	}
	v, ok := c.varPool.Get("project_commands")
	if !ok {
		return ""
	}
	cmds, _ := v.Value.(string)
	if cmds == "" {
		return ""
	}

	return fmt.Sprintf(`
## Available Project Commands

The following commands are available in this project:
%s

Use these commands for building, testing, and other project operations.
`, cmds)
}

// browserToolsSection returns guidance for using browser automation tools.
func browserToolsSection() string {
	return `## Browser Automation

If you need to interact with a browser (navigate, click, screenshot, etc.), use these CLI commands instead of Playwright MCP tools:

| Command | Description |
|---------|-------------|
| kvelmo browser navigate <url> | Navigate to a URL |
| kvelmo browser snapshot | Capture accessibility tree (for understanding page structure) |
| kvelmo browser screenshot | Take a screenshot (auto-saved to Screenshots panel) |
| kvelmo browser click <selector> | Click an element |
| kvelmo browser type <selector> <text> | Type text into an element |
| kvelmo browser wait <selector> | Wait for an element to appear |
| kvelmo browser eval <js> | Evaluate JavaScript |
| kvelmo browser console | Show console messages |
| kvelmo browser network | Show network requests |

These commands integrate with kvelmo's screenshot store - screenshots appear in the web UI's Screenshots panel for user visibility.
`
}

func (c *Conductor) buildSimplifyPrompt() string {
	wu := c.workUnit

	return fmt.Sprintf(`Simplify the implementation for the following task:

Title: %s
Description: %s

Please review the code that was just implemented and simplify it for clarity:
1. Remove unnecessary complexity and abstractions
2. Simplify control flow where possible
3. Remove dead code and unused variables
4. Consolidate duplicate logic
5. Use clearer, more descriptive names
6. Break down overly long functions
7. Prefer standard library solutions over custom implementations

Focus on making the code easier to understand and maintain.
Do NOT add new features or change functionality - only simplify.
Commit your changes with meaningful commit messages.
`, wu.Title, wu.Description)
}

func (c *Conductor) buildOptimizePrompt() string {
	wu := c.workUnit

	return fmt.Sprintf(`Review and optimize the implementation for the following task:

Title: %s
Description: %s

Please review the code that was just implemented and optimize it:
1. Improve code quality and readability
2. Add missing error handling
3. Optimize performance where applicable
4. Ensure proper documentation/comments
5. Check for edge cases and add handling
6. Ensure tests are comprehensive

Make any improvements while maintaining the existing functionality.
Commit your changes with meaningful commit messages.
`, wu.Title, wu.Description)
}
