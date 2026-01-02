// Package template provides task templates for common development patterns.
// Templates pre-configure task frontmatter, agent selection, and workflow settings.
package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Template represents a task template with frontmatter and configuration.
type Template struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Frontmatter map[string]any    `yaml:"frontmatter,omitempty"`
	Agent       string            `yaml:"agent,omitempty"`
	AgentSteps  map[string]any    `yaml:"agent_steps,omitempty"`
	Git         map[string]string `yaml:"git,omitempty"`
	Workflow    map[string]any    `yaml:"workflow,omitempty"`
}

// BuiltInTemplates returns all built-in template names.
func BuiltInTemplates() []string {
	return []string{"bug-fix", "feature", "refactor", "docs", "test", "chore"}
}

// LoadBuiltIn loads a built-in template by name.
func LoadBuiltIn(name string) (*Template, error) {
	// Read from embedded templates directory
	data, err := templatesFS.ReadFile("templates/" + name + ".yaml")
	if err != nil {
		return nil, fmt.Errorf("template not found: %s", name)
	}

	var tpl Template
	if err := yaml.Unmarshal(data, &tpl); err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	return &tpl, nil
}

// ApplyToContent applies template frontmatter to existing content.
// If content already has frontmatter, it merges the template values (template takes precedence).
// Returns the combined content with YAML frontmatter.
func (t *Template) ApplyToContent(content string) string {
	var existingFrontmatter map[string]any
	var bodyContent string

	// Check if content has existing frontmatter
	if strings.HasPrefix(content, "---\n") {
		parts := strings.SplitN(content[4:], "\n---", 2)
		if len(parts) == 2 {
			if err := yaml.Unmarshal([]byte(parts[0]), &existingFrontmatter); err == nil {
				bodyContent = strings.TrimLeft(parts[1], "\n")
			} else {
				// Failed to parse, treat entire content as body
				bodyContent = content
			}
		} else {
			bodyContent = content
		}
	} else {
		bodyContent = content
	}

	// Merge frontmatter (template values override existing)
	if existingFrontmatter == nil {
		existingFrontmatter = make(map[string]any)
	}
	for k, v := range t.Frontmatter {
		existingFrontmatter[k] = v
	}

	// Build output
	var builder strings.Builder
	builder.WriteString("---\n")

	// Write frontmatter
	fmData, _ := yaml.Marshal(existingFrontmatter)
	builder.Write(fmData)
	builder.WriteString("---\n\n")
	builder.WriteString(bodyContent)

	return builder.String()
}

// GetDescription returns a formatted description of what the template does.
func (t *Template) GetDescription() string {
	desc := t.Description
	if desc == "" {
		desc = "No description available"
	}

	var details []string
	if t.Agent != "" {
		details = append(details, "Agent: "+t.Agent)
	}
	if t.Frontmatter != nil {
		if taskType, ok := t.Frontmatter["type"].(string); ok {
			details = append(details, "Type: "+taskType)
		}
	}

	if len(details) > 0 {
		return fmt.Sprintf("%s (%s)", desc, strings.Join(details, ", "))
	}

	return desc
}

// templatesFS is an interface for accessing embedded template files.
// In production, this could use embed.FS. For now, we use os.ReadFile.
type osFS struct{}

func (osFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join("internal", "template", name))
}

var templatesFS osFS
