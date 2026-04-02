package settings

import (
	"reflect"
	"strings"
)

// SchemaVersion is the current schema format version.
const SchemaVersion = "1.0"

// sectionCustomAgents is the section ID for custom agent configuration.
const sectionCustomAgents = "custom_agents"

// tagValueTrue is the string representation of boolean true in schema tags.
const tagValueTrue = "true"

// Generate creates a Schema from a Go struct type using reflection.
// It uses reflect.Type traversal to safely handle nil pointer fields.
//
// Usage:
//
//	schema := settings.Generate(reflect.TypeOf(settings.Settings{}))
func Generate(cfgType reflect.Type) *Schema {
	// Handle pointer types by getting underlying type
	if cfgType.Kind() == reflect.Ptr {
		cfgType = cfgType.Elem()
	}

	if cfgType.Kind() != reflect.Struct {
		return &Schema{Version: SchemaVersion}
	}

	sections := make(map[string]*Section)
	var sectionOrder []string

	// Traverse all fields in the config struct
	for structField := range cfgType.Fields() {
		processField(structField, "", sections, &sectionOrder)
	}

	// Build ordered sections list
	result := &Schema{
		Version:  SchemaVersion,
		Sections: make([]Section, 0, len(sectionOrder)),
	}

	for _, sectionID := range sectionOrder {
		if section, ok := sections[sectionID]; ok && len(section.Fields) > 0 {
			result.Sections = append(result.Sections, *section)
		}
	}

	return result
}

// GenerateSchema is a convenience function that generates the schema for Settings.
func GenerateSchema() *Schema {
	return Generate(reflect.TypeFor[Settings]())
}

// GenerateSchemaWithCustomAgents generates the schema and adds custom agents to the agent selection options.
func GenerateSchemaWithCustomAgents(s *Settings) *Schema {
	schema := GenerateSchema()

	// Collect custom agent names
	var customAgentNames []string
	if s != nil && len(s.CustomAgents) > 0 {
		for name := range s.CustomAgents {
			customAgentNames = append(customAgentNames, name)
		}
	}

	if len(customAgentNames) == 0 {
		return schema
	}

	// Add custom agents to relevant options
	for i := range schema.Sections {
		section := &schema.Sections[i]

		// Add to agent.default and agent.allowed options
		if section.ID == "agent" {
			for j := range section.Fields {
				field := &section.Fields[j]
				if field.Path == KeyAgentDefault || field.Path == "agent.allowed" {
					for _, name := range customAgentNames {
						field.Options = append(field.Options, SelectOption{
							Value: name,
							Label: name + " (custom)",
						})
					}
				}
			}
		}

		// Add to custom_agents extends options (so custom agents can extend other custom agents)
		if section.ID == sectionCustomAgents {
			for j := range section.Fields {
				field := &section.Fields[j]
				if field.Path == sectionCustomAgents && field.ItemSchema != nil {
					for k := range field.ItemSchema {
						itemField := &field.ItemSchema[k]
						if itemField.Path == "extends" {
							for _, name := range customAgentNames {
								itemField.Options = append(itemField.Options, SelectOption{
									Value: name,
									Label: name + " (custom)",
								})
							}
						}
					}
				}
			}
		}
	}

	return schema
}

// processField recursively processes a struct field and adds it to the appropriate section.
func processField(structField reflect.StructField, pathPrefix string, sections map[string]*Section, sectionOrder *[]string) {
	// Skip unexported fields
	if !structField.IsExported() {
		return
	}

	// Get JSON tag for path building (prefer yaml tag for yaml-based config)
	yamlTag := structField.Tag.Get("yaml")
	jsonTag := structField.Tag.Get("json")

	// Use yaml tag if available, otherwise json
	tag := yamlTag
	if tag == "" {
		tag = jsonTag
	}

	if tag == "-" {
		// Skip fields with yaml:"-" or json:"-" UNLESS they have an env tag
		// (sensitive fields have yaml:"-" but should still appear in schema)
		schemaTag := structField.Tag.Get("schema")
		if schemaTag == "" || !strings.Contains(schemaTag, "env=") {
			return
		}
	}

	// Extract field name from tag
	fieldName := strings.Split(tag, ",")[0]
	if fieldName == "" || fieldName == "-" {
		fieldName = toSnakeCase(structField.Name)
	}

	// Build the full path
	var path string
	if pathPrefix == "" {
		path = fieldName
	} else {
		path = pathPrefix + "." + fieldName
	}

	// Get field type, handling pointers
	fieldType := structField.Type
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}

	// Check for schema tag
	schemaTag := structField.Tag.Get("schema")

	// Handle nested structs
	if fieldType.Kind() == reflect.Struct {
		// Check if this is a struct with its own schema tags (leaf struct)
		// or a container struct that should be recursed into
		hasSchemaTag := schemaTag != ""

		if hasSchemaTag {
			// Treat as a single field
			addFieldToSection(path, schemaTag, fieldType, sections, sectionOrder)
		} else {
			// Recurse into nested struct
			for nestedField := range fieldType.Fields() {
				processField(nestedField, path, sections, sectionOrder)
			}
		}

		return
	}

	// Handle map types - special handling for custom_agents
	if fieldType.Kind() == reflect.Map {
		if path == sectionCustomAgents {
			addCustomAgentsSection(sections, sectionOrder)
		}

		return
	}

	// Handle slice types - include if has schema tag
	if fieldType.Kind() == reflect.Slice {
		if schemaTag != "" {
			addFieldToSection(path, schemaTag, fieldType, sections, sectionOrder)
		}

		return
	}

	// Process primitive fields with schema tags
	if schemaTag != "" {
		addFieldToSection(path, schemaTag, fieldType, sections, sectionOrder)
	}
}

// addFieldToSection parses the schema tag and adds a field to the appropriate section.
func addFieldToSection(path, schemaTag string, fieldType reflect.Type, sections map[string]*Section, sectionOrder *[]string) {
	// Parse the schema tag
	tags := parseSchemaTag(schemaTag)

	// Skip fields without a label (not intended for UI)
	label := tags["label"]
	if label == "" {
		return
	}

	// Determine section from path prefix
	sectionID := strings.Split(path, ".")[0]

	// Create section if needed
	if _, ok := sections[sectionID]; !ok {
		meta := GetSectionMeta(sectionID)
		sections[sectionID] = &Section{
			ID:          sectionID,
			Title:       meta.Title,
			Description: meta.Description,
			Icon:        meta.Icon,
			Category:    meta.Category,
			Fields:      []Field{},
		}
		*sectionOrder = append(*sectionOrder, sectionID)
	}

	// Determine field type
	uiType := inferFieldType(fieldType, tags)

	// Build the field
	field := Field{
		Path:        path,
		Type:        uiType,
		Label:       label,
		Description: tags["desc"],
		Placeholder: tags["placeholder"],
		Sensitive:   tags["sensitive"] == tagValueTrue,
		EnvVar:      tags["env"],
		HelpURL:     tags["helpUrl"],
		Advanced:    tags["advanced"] == tagValueTrue,
		ShowWhen:    parseShowWhen(tags["showWhen"]),
		Options:     parseOptions(tags["options"]),
		Multiple:    tags["type"] == "multiselect",
	}

	// Parse default value based on type
	if defaultStr := tags["default"]; defaultStr != "" {
		field.Default = parseDefaultValue(defaultStr, fieldType)
	}

	// Build validation rules
	validation := buildValidation(tags)
	if validation != nil {
		field.Validation = validation
	}

	// Add field to section
	sections[sectionID].Fields = append(sections[sectionID].Fields, field)
}

// addCustomAgentsSection adds a special section for custom_agents configuration.
// Custom agents use a dynamic list UI since they're stored as map[string]CustomAgent.
func addCustomAgentsSection(sections map[string]*Section, sectionOrder *[]string) {
	sectionID := sectionCustomAgents
	meta := GetSectionMeta(sectionID)

	// Create section
	sections[sectionID] = &Section{
		ID:          sectionID,
		Title:       meta.Title,
		Description: meta.Description,
		Icon:        meta.Icon,
		Category:    meta.Category,
		Fields:      []Field{},
	}
	*sectionOrder = append(*sectionOrder, sectionID)

	// Build itemSchema from CustomAgent struct schema tags
	itemSchema := []Field{
		{
			Path:        "extends",
			Type:        TypeSelect,
			Label:       "Base Agent",
			Description: "Agent to wrap",
			Options:     parseOptions("claude|codex"),
		},
		{
			Path:        "description",
			Type:        TypeString,
			Label:       "Description",
			Description: "Human-readable description",
		},
		{
			Path:        "args",
			Type:        TypeTags,
			Label:       "CLI Arguments",
			Description: "Additional arguments passed to agent",
		},
		{
			Path:        "env",
			Type:        TypeKeyValue,
			Label:       "Environment",
			Description: "Environment variables for this agent",
		},
	}

	// Add the list field
	sections[sectionID].Fields = append(sections[sectionID].Fields, Field{
		Path:        sectionCustomAgents,
		Type:        TypeList,
		Label:       "Custom Agents",
		Description: "Define custom agent configurations that wrap base agents with additional settings",
		ItemSchema:  itemSchema,
	})
}
