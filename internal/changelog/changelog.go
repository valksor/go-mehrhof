package changelog

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Entry represents a single changelog entry.
type Entry struct {
	Date        time.Time
	Title       string
	Description string
	TaskID      string
	PRURL       string
	Category    string // "Added", "Changed", "Fixed", "Removed"
}

// Categorize determines the changelog category from the title.
func Categorize(title string) string {
	lower := strings.ToLower(title)
	switch {
	case strings.HasPrefix(lower, "fix") || strings.Contains(lower, "bug"):
		return "Fixed"
	case strings.HasPrefix(lower, "remove") || strings.HasPrefix(lower, "delete"):
		return "Removed"
	case strings.HasPrefix(lower, "change") || strings.HasPrefix(lower, "update") || strings.HasPrefix(lower, "refactor"):
		return "Changed"
	default:
		return "Added"
	}
}

// AppendEntry appends a changelog entry to CHANGELOG.md following Keep a Changelog format.
// If the file doesn't exist, it creates it with a header.
func AppendEntry(changelogPath string, entry Entry) error {
	if entry.Category == "" {
		entry.Category = Categorize(entry.Title)
	}

	// Format the entry line
	line := "- " + entry.Title
	if entry.TaskID != "" {
		line += fmt.Sprintf(" (%s)", entry.TaskID)
	}
	if entry.PRURL != "" {
		line += fmt.Sprintf(" [PR](%s)", entry.PRURL)
	}

	existing, err := os.ReadFile(changelogPath)
	if err != nil {
		// Create new changelog
		content := fmt.Sprintf(`# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### %s

%s
`, entry.Category, line)

		return os.WriteFile(changelogPath, []byte(content), 0o644)
	}

	// Insert into existing changelog under [Unreleased] section
	content := string(existing)

	// Find the [Unreleased] section
	unreleasedIdx := strings.Index(content, "## [Unreleased]")
	if unreleasedIdx < 0 {
		// No Unreleased section - add one after the header
		headerEnd := strings.Index(content, "\n## ")
		if headerEnd < 0 {
			headerEnd = len(content)
		}
		insert := fmt.Sprintf("\n## [Unreleased]\n\n### %s\n\n%s\n", entry.Category, line)
		content = content[:headerEnd] + insert + content[headerEnd:]

		return os.WriteFile(changelogPath, []byte(content), 0o644)
	}

	// Find the category section under Unreleased, or create it
	afterUnreleased := content[unreleasedIdx:]
	categoryHeader := "### " + entry.Category
	catIdx := strings.Index(afterUnreleased, categoryHeader)

	if catIdx >= 0 {
		// Category exists - add entry after the header line
		insertPos := unreleasedIdx + catIdx + len(categoryHeader)
		// Find end of header line
		nlIdx := strings.Index(content[insertPos:], "\n")
		if nlIdx >= 0 {
			insertPos += nlIdx
		}
		content = content[:insertPos] + "\n" + line + content[insertPos:]
	} else {
		// Category doesn't exist - add it after [Unreleased] header
		insertPos := unreleasedIdx + len("## [Unreleased]")
		nlIdx := strings.Index(content[insertPos:], "\n")
		if nlIdx >= 0 {
			insertPos += nlIdx
		}
		content = content[:insertPos] + fmt.Sprintf("\n\n### %s\n\n%s", entry.Category, line) + content[insertPos:]
	}

	return os.WriteFile(changelogPath, []byte(content), 0o644)
}
