package policy

import (
	"fmt"
	"path"
	"slices"
)

// Severity indicates how a policy violation affects the transition.
type Severity string

const (
	// SeverityError blocks the transition.
	SeverityError Severity = "error"
	// SeverityWarning allows the transition but warns the user.
	SeverityWarning Severity = "warning"
)

// Violation represents a single policy check failure.
type Violation struct {
	Severity Severity `json:"severity"`
	Rule     string   `json:"rule"`
	Message  string   `json:"message"`
}

// Settings holds configurable policy constraints evaluated before state transitions.
type Settings struct {
	RequiredPhases      []string         `yaml:"required_phases" json:"required_phases" schema:"label=Required Phases;desc=Workflow phases that cannot be skipped;type=tags"`
	SensitivePaths      []string         `yaml:"sensitive_paths" json:"sensitive_paths" schema:"label=Sensitive Paths;desc=Glob patterns for files requiring mandatory review;type=tags"`
	MinSpecSections     int              `yaml:"min_spec_sections" json:"min_spec_sections" schema:"label=Min Specifications;desc=Minimum specification files required before implementation;default=0;min=0;max=10"`
	RequireSecurityScan bool             `yaml:"require_security_scan" json:"require_security_scan" schema:"label=Require Security Scan;desc=Block submission when security findings exist;default=false"`
	DocRequirements     []DocRequirement `yaml:"doc_requirements" json:"doc_requirements"`
}

// DocRequirement defines a rule: when files matching Trigger change, files matching Requires must also change.
type DocRequirement struct {
	Trigger  string `yaml:"trigger" json:"trigger"`
	Requires string `yaml:"requires" json:"requires"`
}

// phaseOrder defines the normal workflow progression used to detect skipped phases.
var phaseOrder = []string{
	"loaded", "planning", "planned", "implementing", "implemented",
	"simplifying", "optimizing", "reviewing", "submitted",
}

// eventTarget maps events to their target state in the workflow.
var eventTarget = map[string]string{
	"start":     "loaded",
	"plan":      "planning",
	"implement": "implementing",
	"simplify":  "simplifying",
	"optimize":  "optimizing",
	"review":    "reviewing",
	"submit":    "submitted",
	"finish":    "submitted",
}

// Evaluate checks configurable constraints before a state transition and returns
// any policy violations. It does not block the transition itself; callers should
// inspect the returned violations and decide how to proceed.
func Evaluate(cfg Settings, event string, state string, specs []string, changedFiles []string) []Violation {
	var violations []Violation

	// Check security scan requirement on submit.
	if event == "submit" && cfg.RequireSecurityScan {
		violations = append(violations, Violation{
			Severity: SeverityWarning,
			Rule:     "require_security_scan",
			Message:  "security scan required before submit",
		})
	}

	// Check minimum spec sections before implement.
	if event == "implement" && cfg.MinSpecSections > 0 && len(specs) < cfg.MinSpecSections {
		violations = append(violations, Violation{
			Severity: SeverityError,
			Rule:     "min_spec_sections",
			Message:  fmt.Sprintf("need at least %d specification files before implementation, got %d", cfg.MinSpecSections, len(specs)),
		})
	}

	// Check required phases are not being skipped.
	violations = append(violations, checkRequiredPhases(cfg.RequiredPhases, event, state)...)

	// Check sensitive paths.
	violations = append(violations, checkSensitivePaths(cfg.SensitivePaths, changedFiles)...)

	// Check documentation requirements.
	violations = append(violations, checkDocRequirements(cfg.DocRequirements, changedFiles)...)

	return violations
}

// HasBlockingViolation returns true if any violation has SeverityError.
func HasBlockingViolation(violations []Violation) bool {
	return slices.ContainsFunc(violations, func(v Violation) bool {
		return v.Severity == SeverityError
	})
}

// checkRequiredPhases verifies that no required phase is skipped by the transition.
func checkRequiredPhases(requiredPhases []string, event string, currentState string) []Violation {
	if len(requiredPhases) == 0 {
		return nil
	}

	target, ok := eventTarget[event]
	if !ok {
		return nil
	}

	currentIdx := slices.Index(phaseOrder, currentState)
	targetIdx := slices.Index(phaseOrder, target)

	// If either state is not in the phase order, we can't determine skipping.
	if currentIdx < 0 || targetIdx < 0 || targetIdx <= currentIdx {
		return nil
	}

	var violations []Violation

	// Check every phase between current (exclusive) and target (exclusive).
	for i := currentIdx + 1; i < targetIdx; i++ {
		phase := phaseOrder[i]
		if slices.Contains(requiredPhases, phase) {
			violations = append(violations, Violation{
				Severity: SeverityError,
				Rule:     "required_phase",
				Message:  fmt.Sprintf("required phase %q would be skipped by %s from %s", phase, event, currentState),
			})
		}
	}

	return violations
}

// checkSensitivePaths checks if any changed file matches a sensitive path glob pattern.
func checkSensitivePaths(sensitivePaths []string, changedFiles []string) []Violation {
	if len(sensitivePaths) == 0 || len(changedFiles) == 0 {
		return nil
	}

	for _, file := range changedFiles {
		for _, pattern := range sensitivePaths {
			if matched, _ := path.Match(pattern, file); matched {
				return []Violation{{
					Severity: SeverityWarning,
					Rule:     "sensitive_paths",
					Message:  "sensitive files modified, review recommended",
				}}
			}
		}
	}

	return nil
}

// checkDocRequirements verifies that when files matching a trigger pattern change,
// files matching the corresponding requires pattern also change.
func checkDocRequirements(requirements []DocRequirement, changedFiles []string) []Violation {
	if len(requirements) == 0 || len(changedFiles) == 0 {
		return nil
	}

	var violations []Violation

	for _, req := range requirements {
		triggerMatched := false
		requiresMatched := false

		for _, file := range changedFiles {
			if matched, _ := path.Match(req.Trigger, file); matched {
				triggerMatched = true
			}
			if matched, _ := path.Match(req.Requires, file); matched {
				requiresMatched = true
			}
		}

		if triggerMatched && !requiresMatched {
			violations = append(violations, Violation{
				Severity: SeverityError,
				Rule:     "doc_requirement",
				Message:  fmt.Sprintf("files matching %q changed but required documentation matching %q was not updated", req.Trigger, req.Requires),
			})
		}
	}

	return violations
}
