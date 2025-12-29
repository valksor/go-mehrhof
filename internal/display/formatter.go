package display

import (
	"fmt"
	"strings"
	"time"
)

// Formatting constants for consistent output across the CLI.
const (
	// Standard indentation levels.
	IndentNone  = ""
	IndentOne   = "  "
	IndentTwo   = "    "
	IndentThree = "      "

	// Standard widths for tables and progress bars.
	DefaultTableWidth  = 80
	ProgressBarWidth   = 30
	DefaultColumnWidth = 20
)

// Standard separators (variables since they use strings.Repeat).
var (
	SeparatorLine = strings.Repeat("─", 60)
	SeparatorDash = strings.Repeat("-", 60)
)

// TimestampFormat is the standard timestamp format for CLI output.
const TimestampFormat = "2006-01-02 15:04:05"

// ShortTimestampFormat is a shorter timestamp format.
const ShortTimestampFormat = "15:04:05"

// Formatter provides consistent output formatting.
type Formatter struct {
	indentLevel int
	width       int
	color       bool
}

// NewFormatter creates a new formatter with default settings.
func NewFormatter() *Formatter {
	return &Formatter{
		indentLevel: 0,
		width:       DefaultTableWidth,
		color:       true,
	}
}

// SetIndent sets the current indentation level.
func (f *Formatter) SetIndent(level int) *Formatter {
	f.indentLevel = level
	return f
}

// SetWidth sets the output width.
func (f *Formatter) SetWidth(w int) *Formatter {
	f.width = w
	return f
}

// Indent returns the current indentation string.
func (f *Formatter) Indent() string {
	return strings.Repeat(IndentOne, f.indentLevel)
}

// Section prints a section header with consistent formatting.
func (f *Formatter) Section(title string) string {
	separator := SeparatorLine
	if len(title) > 0 {
		return fmt.Sprintf("\n%s\n%s\n%s\n", Bold(title), separator, "")
	}
	return fmt.Sprintf("\n%s\n", separator)
}

// Subsection prints a subsection header.
func (f *Formatter) Subsection(title string) string {
	return fmt.Sprintf("\n%s\n%s\n", Muted(title), strings.Repeat("─", len(title)))
}

// KeyValue formats a key-value pair with consistent alignment.
func (f *Formatter) KeyValue(key, value string) string {
	indent := f.Indent()
	keyWidth := 12 // Standard key width
	return fmt.Sprintf("%s%-*s %s\n", indent, keyWidth, key+":", value)
}

// KeyValues formats multiple key-value pairs.
func (f *Formatter) KeyValues(pairs map[string]string) string {
	var sb strings.Builder
	indent := f.Indent()

	// Find the longest key for alignment
	maxLen := 0
	for key := range pairs {
		if len(key) > maxLen {
			maxLen = len(key)
		}
	}

	for key, value := range pairs {
		sb.WriteString(fmt.Sprintf("%s%-*s %s\n", indent, maxLen+1, key+":", value))
	}

	return sb.String()
}

// List formats a bulleted list.
func (f *Formatter) List(items []string) string {
	var sb strings.Builder
	indent := f.Indent()

	for i, item := range items {
		bullet := "•"
		if i < 10 {
			bullet = fmt.Sprintf("%d.", i+1)
		}
		sb.WriteString(fmt.Sprintf("%s%s %s\n", indent, Muted(bullet), item))
	}

	return sb.String()
}

// DefinitionList formats a definition list (term with description).
func (f *Formatter) DefinitionList(terms map[string]string) string {
	var sb strings.Builder
	indent := f.Indent()

	for term, definition := range terms {
		sb.WriteString(fmt.Sprintf("%s%s\n%s%s%s\n\n",
			indent, Bold(term),
			indent, IndentOne, definition,
		))
	}

	return sb.String()
}

// CodeBlock formats a code block with consistent indentation.
func (f *Formatter) CodeBlock(code string, language string) string {
	indent := f.Indent()
	var sb strings.Builder

	sb.WriteString(indent)
	sb.WriteString(Muted("```"))
	if language != "" {
		sb.WriteString(language)
	}
	sb.WriteString("\n")

	// Indent each line of code
	lines := strings.Split(code, "\n")
	for _, line := range lines {
		sb.WriteString(indent)
		sb.WriteString(IndentOne)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	sb.WriteString(indent)
	sb.WriteString(Muted("```"))
	sb.WriteString("\n")

	return sb.String()
}

// Timestamp formats a time.Time using the standard format.
func (f *Formatter) Timestamp(t time.Time) string {
	return t.Format(TimestampFormat)
}

// ShortTimestamp formats a time.Time using the short format.
func (f *Formatter) ShortTimestamp(t time.Time) string {
	return t.Format(ShortTimestampFormat)
}

// RelativeTimestamp formats a time.Time as a relative duration.
func (f *Formatter) RelativeTimestamp(t time.Time) string {
	duration := time.Since(t)

	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%d min ago", minutes)
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		return fmt.Sprintf("%d hr ago", hours)
	case duration < 30*24*time.Hour:
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%d day ago", days)
	default:
		return t.Format("2006-01-02")
	}
}

// Truncate truncates a string to a maximum length, adding "..." if truncated.
func (f *Formatter) Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."
	}
	return s[:maxLen-3] + "..."
}

// Table formats a simple table with headers.
func (f *Formatter) Table(headers []string, rows [][]string) string {
	var sb strings.Builder

	// Calculate column widths
	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Print header
	sb.WriteString(Bold(strings.Join(headers, "  ")))
	sb.WriteString("\n")

	// Print separator
	var separators []string
	for _, w := range colWidths {
		separators = append(separators, strings.Repeat("─", w))
	}
	sb.WriteString(Muted(strings.Join(separators, "  ")))
	sb.WriteString("\n")

	// Print rows
	for _, row := range rows {
		cells := make([]string, len(row))
		for i, cell := range row {
			if i < len(colWidths) {
				cells[i] = fmt.Sprintf("%-*s", colWidths[i], cell)
			} else {
				cells[i] = cell
			}
		}
		sb.WriteString(strings.Join(cells, "  "))
		sb.WriteString("\n")
	}

	return sb.String()
}

// Helper functions for quick formatting without a Formatter instance.

// Section formats a section header.
func Section(title string) string {
	return NewFormatter().Section(title)
}

// KeyValue formats a key-value pair.
func KeyValue(key, value string) string {
	return NewFormatter().KeyValue(key, value)
}

// List formats a bulleted list.
func List(items []string) string {
	return NewFormatter().List(items)
}

// Truncate truncates a string to a maximum length.
func Truncate(s string, maxLen int) string {
	return NewFormatter().Truncate(s, maxLen)
}
