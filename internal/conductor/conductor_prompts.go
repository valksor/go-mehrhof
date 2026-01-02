package conductor

import (
	"fmt"
	"strings"

	"github.com/valksor/go-mehrhof/internal/agent"
)

// buildPlanningPrompt creates the prompt for specification generation.
func buildPlanningPrompt(title, sourceContent, notes, existingSpecs string) string {
	prompt := fmt.Sprintf(`You are a software architect. Analyze this task and create a detailed implementation specification.

## Task
%s

## Source Content
%s
`, title, sourceContent)

	if existingSpecs != "" {
		prompt += fmt.Sprintf(`
## Previous Specifications
IMPORTANT: The following specifications already exist from previous planning iterations.
DO NOT start from scratch. Build upon these, refine them, or address any gaps:

%s

Your new specification should acknowledge what was already planned and either:
1. Refine/improve the existing specification
2. Add missing details
3. Address any gaps or questions that arose
`, existingSpecs)
	}

	if notes != "" {
		prompt += fmt.Sprintf(`
## Additional Notes
%s
`, notes)
	}

	prompt += `
## Instructions
Create a detailed specification that includes:
1. Overview of what needs to be implemented
2. Technical approach and architecture decisions
3. Step-by-step implementation plan
4. Files that need to be created or modified
5. Testing strategy
6. Acceptance criteria

Output your specification in a structured format with clear sections.`

	return prompt
}

// buildImplementationPrompt creates the prompt for implementation.
func buildImplementationPrompt(title, sourceContent, specsContent, notes string) string {
	prompt := fmt.Sprintf(`You are a software engineer. Implement the following task according to the specifications.

## Task
%s

## Original Requirements
%s

## Specifications
%s
`, title, sourceContent, specsContent)

	if notes != "" {
		prompt += fmt.Sprintf(`
## Additional Notes
%s
`, notes)
	}

	prompt += `
## Instructions
Implement this task according to the specifications. For each file you create or modify:
1. Use yaml:file blocks with path, operation (create/update/delete), and content
2. Follow existing code style and patterns
3. Include necessary imports
4. Add appropriate error handling
5. Write clean, maintainable code

Output each file change in a yaml:file block.`

	return prompt
}

// buildReviewPrompt creates the prompt for code review.
func buildReviewPrompt(title, sourceContent, specsContent string) string {
	return buildReviewPromptWithLint(title, sourceContent, specsContent, "")
}

// buildReviewPromptWithLint creates the prompt for code review including lint results.
// If lintResults is empty, it falls back to the standard review prompt.
func buildReviewPromptWithLint(title, sourceContent, specsContent, lintResults string) string {
	prompt := fmt.Sprintf(`You are a senior software engineer conducting a code review.

## Task
%s

## Original Requirements
%s

## Specifications
%s
`, title, sourceContent, specsContent)

	// Include lint results if available
	if lintResults != "" {
		prompt += fmt.Sprintf(`
%s
`, lintResults)
	}

	prompt += `
## Instructions
Review the implementation for:
1. Correctness - Does it meet the specifications?
2. Code quality - Is it clean, readable, and maintainable?
3. Security - Are there any vulnerabilities?
4. Performance - Are there any obvious bottlenecks?
5. Best practices - Does it follow language/framework conventions?`

	// Add lint-specific instruction if results were provided
	if lintResults != "" {
		prompt += `
6. Lint issues - Address all issues found by automated linters above`
	}

	prompt += `

Provide:
1. A summary of your findings
2. Any issues found (critical, major, minor)
3. Suggested improvements
4. If needed, provide corrected code in yaml:file blocks`

	return prompt
}

// formatSpecificationContent formats a specification file from agent response.
func formatSpecificationContent(num int, response *agent.Response) string {
	content := fmt.Sprintf("# Specification %d\n\n", num)

	if response.Summary != "" {
		content += "## Summary\n\n" + response.Summary + "\n\n"
	}

	if len(response.Messages) > 0 {
		content += "## Details\n\n"
		for _, msg := range response.Messages {
			content += msg + "\n\n"
		}
	}

	return content
}

// extractContextSummary extracts a brief summary from the agent response.
// Uses the Summary field if available, otherwise truncates the first message.
func extractContextSummary(response *agent.Response) string {
	if response.Summary != "" {
		return response.Summary
	}
	if len(response.Messages) > 0 {
		msg := response.Messages[0]
		// Truncate to ~2000 chars for token efficiency
		if len(msg) > 2000 {
			return msg[:2000] + "\n[truncated...]"
		}
		return msg
	}
	return ""
}

// buildFullContext combines all agent output into a single context string.
// This includes the summary and all messages.
func buildFullContext(response *agent.Response) string {
	var parts []string
	if response.Summary != "" {
		parts = append(parts, "## Summary\n"+response.Summary)
	}
	if len(response.Messages) > 0 {
		parts = append(parts, "## Messages\n"+strings.Join(response.Messages, "\n\n"))
	}
	return strings.Join(parts, "\n\n")
}

// extractExploredFiles extracts file paths from the agent response.
// Returns paths from file changes and attempts to find file references in messages.
func extractExploredFiles(response *agent.Response) []string {
	seen := make(map[string]bool)
	var files []string

	// Add files from file changes
	for _, fc := range response.Files {
		if !seen[fc.Path] {
			seen[fc.Path] = true
			files = append(files, fc.Path)
		}
	}

	return files
}
