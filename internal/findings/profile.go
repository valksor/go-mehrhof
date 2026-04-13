package findings

// Profile defines a named set of gate rules for a specific quality posture.
type Profile struct {
	Name  string
	Gates []GateRule
}

// HasBlockers evaluates all gate rules and returns true if any blocker exists.
func (p *Profile) HasBlockers(findings []Finding) bool {
	return len(Evaluate(p.Gates, findings)) > 0
}

// Check evaluates all gate rules and returns the blockers.
func (p *Profile) Check(findings []Finding) []Blocker {
	return Evaluate(p.Gates, findings)
}

// Quick profile: used during implement phase. Only critical findings block.
func Quick() *Profile {
	return &Profile{
		Name:  "quick",
		Gates: []GateRule{AnySeverity{SeverityCritical}},
	}
}

// Standard profile: used during review phase. Critical and high findings block.
func Standard() *Profile {
	return &Profile{
		Name:  "standard",
		Gates: []GateRule{AnySeverity{SeverityCritical}, AnySeverity{SeverityHigh}},
	}
}

// Strict profile: used before submit. Critical and high block; medium capped at 3.
func Strict() *Profile {
	return &Profile{
		Name: "strict",
		Gates: []GateRule{
			AnySeverity{SeverityCritical},
			AnySeverity{SeverityHigh},
			MaxCount{Level: SeverityMedium, Max: 3},
		},
	}
}

// ForPhase returns the appropriate profile for a lifecycle phase.
func ForPhase(phase string) *Profile {
	switch phase {
	case "implement", "simplify", "optimize":
		return Quick()
	case "review":
		return Standard()
	case "submit":
		return Strict()
	default:
		return Quick()
	}
}
