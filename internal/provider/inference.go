package provider

import (
	"regexp"
	"strings"
)

var priorityLabels = map[string]string{
	"p0": "p0", "priority:critical": "p0", "urgent": "p0", "critical": "p0",
	"p1": "p1", "priority:high": "p1", "high": "p1",
	"p2": "p2", "priority:medium": "p2", "medium": "p2",
	"p3": "p3", "priority:low": "p3", "low": "p3",
}

func InferPriority(labels []string) string {
	for _, label := range labels {
		normalized := strings.ToLower(label)
		if priority, ok := priorityLabels[normalized]; ok {
			return priority
		}
	}

	return ""
}

var typeLabels = map[string]string{
	"bug": "bug", "defect": "bug", "fix": "bug",
	"feature": "feature", "enhancement": "feature", "feat": "feature",
	"chore": "chore", "maintenance": "chore", "tech-debt": "chore",
	"docs": "docs", "documentation": "docs",
}

func InferType(labels []string) string {
	for _, label := range labels {
		normalized := strings.ToLower(label)
		if taskType, ok := typeLabels[normalized]; ok {
			return taskType
		}
	}

	return ""
}

const maxSlugLength = 50

var (
	nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)
	multipleHyphens = regexp.MustCompile(`-+`)
)

func GenerateSlug(title string) string {
	if title == "" {
		return ""
	}
	slug := strings.ToLower(title)
	slug = nonAlphanumeric.ReplaceAllString(slug, "-")
	slug = multipleHyphens.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > maxSlugLength {
		slug = strings.TrimRight(slug[:maxSlugLength], "-")
	}

	return slug
}

func InferAll(title string, labels []string) (string, string, string) {
	return InferPriority(labels), InferType(labels), GenerateSlug(title)
}
