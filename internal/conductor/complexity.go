package conductor

import (
	"slices"
	"strings"
)

// TaskComplexity represents the detected complexity level of a task.
type TaskComplexity string

const (
	// ComplexitySimple indicates a straightforward task suitable for quick planning.
	ComplexitySimple TaskComplexity = "simple"
	// ComplexityMedium indicates a moderately complex task.
	ComplexityMedium TaskComplexity = "medium"
	// ComplexityComplex indicates a task requiring full planning treatment.
	ComplexityComplex TaskComplexity = "complex"
)

// Thresholds for complexity detection.
const (
	simpleTitleMaxLen   = 100
	simpleContentMaxLen = 500
	simpleMaxFiles      = 1
	complexMinFiles     = 4
)

// complexArchitecturalKeywords indicate tasks requiring deeper planning.
var complexArchitecturalKeywords = []string{
	"refactor", "redesign", "migrate",
	"breaking-change", "architecture",
	"integrate", "replace", "abstract",
	"rewrite", "restructure", "overhaul",
	"design pattern",
}

// DetectTaskComplexity analyzes task metadata to determine appropriate planning complexity.
// Simple if: title <= 100 chars AND content <= 500 chars AND no complexity keywords.
// Complex if: title > 100 chars OR content > 500 chars OR complexity keywords present OR file count > 1.
func DetectTaskComplexity(title, sourceContent string, fileCount int, taskType string, labels []string, hasParent bool) TaskComplexity {
	titleLower := strings.ToLower(title)
	contentLower := strings.ToLower(sourceContent)
	combinedText := titleLower + " " + contentLower

	// Check for explicit complexity signals first (complex takes priority)
	if isDefinitelyComplex(combinedText, fileCount, labels, hasParent) {
		return ComplexityComplex
	}

	// Check for simple task signals
	if isDefinitelySimple(title, sourceContent, fileCount) {
		return ComplexitySimple
	}

	// Default to medium for ambiguous cases
	return ComplexityMedium
}

// isDefinitelyComplex returns true if strong complexity signals are present.
func isDefinitelyComplex(combinedText string, fileCount int, labels []string, hasParent bool) bool {
	// Large file count indicates broad scope
	if fileCount >= complexMinFiles {
		return true
	}

	// Subtasks of larger work need full context
	if hasParent {
		return true
	}

	// Architectural keywords indicate need for thorough planning
	for _, keyword := range complexArchitecturalKeywords {
		if strings.Contains(combinedText, keyword) {
			return true
		}
	}

	// Architectural labels indicate scope
	complexLabels := []string{"architecture", "breaking-change", "migration", "refactor", "redesign", "epic"}
	for _, label := range labels {
		if slices.Contains(complexLabels, strings.ToLower(label)) {
			return true
		}
	}

	return false
}

// isDefinitelySimple returns true if the task appears straightforward.
func isDefinitelySimple(title, sourceContent string, fileCount int) bool {
	// Basic size checks
	if len(title) > simpleTitleMaxLen {
		return false
	}
	if len(sourceContent) > simpleContentMaxLen {
		return false
	}
	if fileCount > simpleMaxFiles {
		return false
	}

	return true
}
