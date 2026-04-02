package settings

import (
	"reflect"
	"strconv"
	"strings"
)

// inferFieldType determines the UI field type from the Go type and tags.
func inferFieldType(goType reflect.Type, tags map[string]string) FieldType {
	// Check for explicit type in tags
	if explicitType := tags["type"]; explicitType != "" {
		switch explicitType {
		case "multiselect":
			return TypeSelect
		case "tags":
			return TypeTags
		case "keyvalue":
			return TypeKeyValue
		case "textarea":
			return TypeTextarea
		case "password":
			return TypePassword
		}
	}

	if tags["options"] != "" {
		return TypeSelect
	}
	if tags["sensitive"] == tagValueTrue {
		return TypePassword
	}

	// Infer from Go type
	switch goType.Kind() { //nolint:exhaustive // Only common Go types are handled
	case reflect.Bool:
		return TypeBoolean
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return TypeNumber
	case reflect.Slice:
		return TypeTags
	default:
		return TypeString
	}
}

// parseDefaultValue converts a default value string to the appropriate Go type.
func parseDefaultValue(defaultStr string, goType reflect.Type) any {
	switch goType.Kind() { //nolint:exhaustive // Only common Go types are handled
	case reflect.Bool:
		return defaultStr == tagValueTrue
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v, err := strconv.ParseInt(defaultStr, 10, 64); err == nil {
			return v
		}

		return 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v, err := strconv.ParseUint(defaultStr, 10, 64); err == nil {
			return v
		}

		return 0
	case reflect.Float32, reflect.Float64:
		if v, err := strconv.ParseFloat(defaultStr, 64); err == nil {
			return v
		}

		return 0.0
	default:
		return defaultStr
	}
}

// buildValidation creates ValidationRules from tag values.
func buildValidation(tags map[string]string) *ValidationRules {
	validation := &ValidationRules{
		Required:       tags["required"] == tagValueTrue,
		Pattern:        tags["pattern"],
		PatternMessage: tags["patternMsg"],
	}

	if minStr := tags["min"]; minStr != "" {
		if v, err := strconv.Atoi(minStr); err == nil {
			validation.Min = &v
		}
	}
	if maxStr := tags["max"]; maxStr != "" {
		if v, err := strconv.Atoi(maxStr); err == nil {
			validation.Max = &v
		}
	}
	if maxlenStr := tags["maxlen"]; maxlenStr != "" {
		if v, err := strconv.Atoi(maxlenStr); err == nil {
			validation.MaxLength = &v
		}
	}

	// Return nil if no validation is specified
	if !validation.Required &&
		validation.Min == nil &&
		validation.Max == nil &&
		validation.MaxLength == nil &&
		validation.Pattern == "" {
		return nil
	}

	return validation
}

// parseSchemaTag parses a schema struct tag using semicolon-separated syntax.
// Format: schema:"key=value;key=value;flag".
func parseSchemaTag(tag string) map[string]string {
	result := make(map[string]string)
	if tag == "" {
		return result
	}

	for pair := range strings.SplitSeq(tag, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		if idx := strings.Index(pair, "="); idx > 0 {
			key := strings.TrimSpace(pair[:idx])
			value := strings.TrimSpace(pair[idx+1:])
			result[key] = value
		} else {
			// Flag without value (e.g., "sensitive", "advanced", "required")
			result[pair] = tagValueTrue
		}
	}

	return result
}

// parseShowWhen parses a showWhen value in the format "path:value".
func parseShowWhen(value string) *Condition {
	if value == "" {
		return nil
	}

	idx := strings.Index(value, ":")
	if idx <= 0 {
		return nil
	}

	field := strings.TrimSpace(value[:idx])
	condValue := strings.TrimSpace(value[idx+1:])

	if neg, ok := strings.CutPrefix(condValue, "!"); ok {
		return &Condition{
			Field:     field,
			NotEquals: neg,
		}
	}

	var equalValue any = condValue
	switch condValue {
	case tagValueTrue:
		equalValue = true
	case "false":
		equalValue = false
	}

	return &Condition{
		Field:  field,
		Equals: equalValue,
	}
}

// parseOptions parses pipe-separated select options.
// Format: "option1|option2|option3" or "value:label|value:label".
func parseOptions(value string) []SelectOption {
	if value == "" {
		return nil
	}

	parts := strings.Split(value, "|")
	options := make([]SelectOption, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if idx := strings.Index(part, ":"); idx > 0 {
			options = append(options, SelectOption{
				Value: strings.TrimSpace(part[:idx]),
				Label: strings.TrimSpace(part[idx+1:]),
			})
		} else {
			options = append(options, SelectOption{
				Value: part,
				Label: capitalizeFirst(part),
			})
		}
	}

	return options
}

// GetSectionMeta returns the metadata for a section by its ID.
func GetSectionMeta(sectionID string) SectionMeta {
	if meta, ok := SectionRegistry[sectionID]; ok {
		return meta
	}

	return SectionMeta{
		Title:    capitalizeFirst(sectionID),
		Category: "features",
	}
}

// capitalizeFirst capitalizes the first letter of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}

	return strings.ToUpper(s[:1]) + s[1:]
}

// toSnakeCase converts a string from PascalCase to snake_case.
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(r)
	}

	return strings.ToLower(result.String())
}
