package conductor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/valksor/kvelmo/agent/recorder"
	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/internal/storage"
	"github.com/valksor/kvelmo/settings"
)

// parseOwnerRepo extracts owner and repo from an external ID like "owner/repo#123".
// Returns empty strings if the format is not recognized.
func parseOwnerRepo(externalID string) (string, string) {
	// Format: owner/repo#number
	repoPath, _, ok := strings.Cut(externalID, "#")
	if !ok {
		return "", ""
	}
	parts := strings.SplitN(repoPath, "/", 2)
	if len(parts) != 2 {
		return "", ""
	}

	return parts[0], parts[1]
}

// detectPRTemplate searches for PR templates in well-known locations within the repo.
// Returns empty string when no template is found.
func detectPRTemplate(repoPath string) string {
	candidates := []string{
		".github/PULL_REQUEST_TEMPLATE.md",
		".github/pull_request_template.md",
		".github/PULL_REQUEST_TEMPLATE",
		".gitlab/merge_request_templates/Default.md",
		"PULL_REQUEST_TEMPLATE.md",
		"pull_request_template.md",
		"docs/pull_request_template.md",
	}
	for _, candidate := range candidates {
		path := filepath.Join(repoPath, candidate)
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data)
		}
	}

	return ""
}

// fillPRTemplate attempts to auto-fill sections in a PR template using task metadata.
// Returns the filled template. Sections that can't be filled are left with placeholder markers.
func fillPRTemplate(template, description string, checkpointCount int, sourceURL string) string {
	lines := strings.Split(template, "\n")
	var result []string

	for i, line := range lines {
		result = append(result, line)
		trimmed := strings.TrimSpace(strings.ToLower(line))

		// Auto-fill known sections
		switch {
		case strings.Contains(trimmed, "summary") || strings.Contains(trimmed, "what changed") || strings.Contains(trimmed, "description"):
			// Insert summary after header if next line is empty or a placeholder
			if i+1 < len(lines) && (strings.TrimSpace(lines[i+1]) == "" || strings.HasPrefix(strings.TrimSpace(lines[i+1]), "<!--")) {
				result = append(result, "", description)
			}
		case strings.Contains(trimmed, "test") || strings.Contains(trimmed, "how to test"):
			if i+1 < len(lines) && (strings.TrimSpace(lines[i+1]) == "" || strings.HasPrefix(strings.TrimSpace(lines[i+1]), "<!--")) {
				result = append(result, "", "- [ ] Verify implementation matches specification", fmt.Sprintf("- [ ] %d checkpoint(s) reviewed", checkpointCount))
			}
		case strings.Contains(trimmed, "related") || strings.Contains(trimmed, "ticket") || strings.Contains(trimmed, "issue"):
			if sourceURL != "" && i+1 < len(lines) && (strings.TrimSpace(lines[i+1]) == "" || strings.HasPrefix(strings.TrimSpace(lines[i+1]), "<!--")) {
				result = append(result, "", sourceURL)
			}
		}
	}

	return strings.Join(result, "\n")
}

// prContext holds spec, review, and recording data for PR body generation.
type prContext struct {
	SpecContent   string
	ReviewSummary string
	Recordings    []map[string]any
}

// buildPRDescription constructs the PR body from task metadata.
// Takes explicit parameters so it can be called outside the lock with copied values.
func buildPRDescription(description string, specCount, checkpointCount int, diffStat string, fileStatuses []git.FileStatus, customSections []settings.PRSection, ctx *prContext) string {
	desc := fmt.Sprintf("## Summary\n\n%s\n", description)

	if len(fileStatuses) > 0 {
		desc += "\n## Changes\n\n"
		var changesSb strings.Builder
		for _, fs := range fileStatuses {
			var icon string
			switch fs.Status {
			case "added":
				icon = "+"
			case "deleted":
				icon = "-"
			case "modified":
				icon = "~"
			case "renamed":
				icon = ">"
			default:
				icon = "?"
			}
			fmt.Fprintf(&changesSb, "- `%s` %s\n", icon, fs.Path)
		}
		desc += changesSb.String()
	}

	if diffStat != "" {
		desc += "\n<details>\n<summary>Diff stats</summary>\n\n```\n" + strings.TrimSpace(diffStat) + "\n```\n</details>\n"
	}

	if specCount > 0 {
		specContent := ""
		if ctx != nil {
			specContent = ctx.SpecContent
		}
		if specContent != "" {
			// Truncate long specs to keep PR readable (rune-safe)
			const maxSpecLen = 2000
			runes := []rune(specContent)
			truncated := specContent
			if len(runes) > maxSpecLen {
				truncated = string(runes[:maxSpecLen]) + "\n\n…(truncated)"
			}
			// Sanitize closing tags that would break the <details> wrapper
			truncated = strings.ReplaceAll(truncated, "</details>", "&lt;/details&gt;")
			desc += "\n## Specification\n\n<details>\n<summary>Implementation specification</summary>\n\n" + truncated + "\n\n</details>\n"
		} else {
			desc += "\n## Implementation\n\nImplemented according to kvelmo specifications.\n"
		}
	}

	if ctx != nil && ctx.ReviewSummary != "" {
		desc += "\n## Review\n\n" + ctx.ReviewSummary + "\n"
	}

	if checkpointCount > 0 {
		desc += fmt.Sprintf("\n## Checkpoints\n\n%d checkpoint(s) created during development.\n", checkpointCount)
	}

	var sectionsSb strings.Builder
	for _, section := range customSections {
		fmt.Fprintf(&sectionsSb, "\n## %s\n\n%s\n", section.Title, section.Content)
	}
	desc += sectionsSb.String()

	desc += "\n---\n*Generated by [kvelmo](https://github.com/valksor/kvelmo)*"

	return desc
}

// buildPRDescriptionWithDecisions extends the PR body with AI decision context.
func buildPRDescriptionWithDecisions(description string, specCount, checkpointCount int, diffStat string, fileStatuses []git.FileStatus, customSections []settings.PRSection, ctx *prContext) string {
	base := buildPRDescription(description, specCount, checkpointCount, diffStat, fileStatuses, customSections, ctx)

	var recordings []map[string]any
	if ctx != nil {
		recordings = ctx.Recordings
	}
	decisions := ExtractDecisions(recordings)
	if len(decisions) == 0 {
		return base
	}

	aiSection := FormatDecisionsMarkdown(decisions, diffStat)

	return base + "\n" + aiSection
}

// loadRecordingsFlat reads recording JSONL files from phase metrics and flattens
// them into the format expected by ExtractDecisions.
// Uses streaming reader to avoid loading entire files into memory.
// Caps at maxRecordsPerFile records per file and maxTotalRecords total.
func loadRecordingsFlat(phaseMetrics map[string]*PhaseMetrics) []map[string]any {
	const maxRecordsPerFile = 5000
	const maxTotalRecords = 10000

	var flat []map[string]any

	for _, pm := range phaseMetrics {
		if pm == nil || pm.RecordingPath == "" {
			continue
		}

		r, err := recorder.OpenReader(pm.RecordingPath)
		if err != nil {
			slog.Debug("could not open recording for PR", "path", pm.RecordingPath, "error", err)

			continue
		}

		count := 0
		for count < maxRecordsPerFile && len(flat) < maxTotalRecords {
			rec, err := r.Next()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					slog.Debug("error reading recording", "path", pm.RecordingPath, "error", err)
				}

				break
			}
			if rec == nil {
				break
			}

			entry := map[string]any{
				"type": rec.Type,
			}

			// Unmarshal Event to extract tool name and file info
			var event map[string]any
			if err := json.Unmarshal(rec.Event, &event); err == nil {
				if content, ok := event["content"].(string); ok {
					entry["tool"] = content
				}
				if data, ok := event["data"].(map[string]any); ok {
					if fp, ok := data["file_path"].(string); ok {
						entry["file"] = fp
					}
				}
			}

			flat = append(flat, entry)
			count++
		}
		if err := r.Close(); err != nil {
			slog.Debug("could not close recording reader", "path", pm.RecordingPath, "error", err)
		}
	}

	return flat
}

// buildReviewSummary constructs a review summary string from work unit state.
func buildReviewSummary(store *storage.Store, taskID string, checklistChecked []string, qualityGatePassed *bool) string {
	var parts []string

	// Quality gate status
	if qualityGatePassed != nil {
		if *qualityGatePassed {
			parts = append(parts, "Quality gate: passed")
		} else {
			parts = append(parts, "Quality gate: failed")
		}
	}

	// Checklist items
	if len(checklistChecked) > 0 {
		parts = append(parts, fmt.Sprintf("Checklist: %d item(s) checked", len(checklistChecked)))
	}

	// Latest review status
	if store != nil && taskID != "" {
		reviStore := storage.NewReviewStore(store)
		if review, err := reviStore.GetLatestReview(taskID); err == nil && review != nil {
			status := review.Status
			if status == "" {
				status = "pending"
			}
			parts = append(parts, "Review status: "+status)
		}
	}

	return strings.Join(parts, "\n")
}

// validatePRSections checks that all required sections in the PR body contain
// meaningful content (not just placeholder text or empty lines).
// Returns the list of section keywords that are missing content.
func validatePRSections(body string, requiredSections []string) []string {
	lines := strings.Split(body, "\n")
	var missing []string

	for _, section := range requiredSections {
		sectionLower := strings.ToLower(section)
		found := false
		hasContent := false

		for i, line := range lines {
			trimmed := strings.TrimSpace(strings.ToLower(line))
			// Check if this line is a header containing the section keyword
			if !strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, sectionLower) {
				continue
			}
			found = true
			// Check lines after header for real content
			for j := i + 1; j < len(lines); j++ {
				nextLine := strings.TrimSpace(lines[j])
				// Stop at next header
				if strings.HasPrefix(nextLine, "#") {
					break
				}
				// Skip empty lines and HTML comments (placeholders)
				if nextLine == "" || strings.HasPrefix(nextLine, "<!--") {
					continue
				}
				hasContent = true

				break
			}

			break
		}

		if !found || !hasContent {
			missing = append(missing, section)
		}
	}

	return missing
}
