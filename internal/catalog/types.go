package catalog

// Template defines a reusable task template that can be loaded
// from the catalog directory.
type Template struct {
	Name              string         `yaml:"name" json:"name"`
	Description       string         `yaml:"description" json:"description"`
	Source            string         `yaml:"source" json:"source"`
	Agent             string         `yaml:"agent" json:"agent"`
	Tags              []string       `yaml:"tags" json:"tags"`
	WorkflowOverrides map[string]any `yaml:"workflow_overrides" json:"workflow_overrides"`
}
