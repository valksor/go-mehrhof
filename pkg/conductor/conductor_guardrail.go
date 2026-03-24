package conductor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/valksor/kvelmo/pkg/findings"
	"github.com/valksor/kvelmo/pkg/policy"
)

// runPreGuardrails executes pre-phase guardrails configured for the given phase.
// Returns an error if any guardrail produces a High or Critical severity finding,
// which should block the phase from proceeding.
// Safe to call without c.mu held — acquires RLock internally to snapshot state.
func (c *Conductor) runPreGuardrails(ctx context.Context, phase string) error {
	s := c.getEffectiveSettings()
	if s == nil {
		return nil
	}

	cfg, ok := s.Workflow.PhaseGuardrails[phase]
	if !ok || len(cfg.Pre) == 0 {
		return nil
	}

	// Snapshot work unit data under RLock to avoid races.
	c.mu.RLock()
	var specs []string
	if c.workUnit != nil {
		specs = make([]string, len(c.workUnit.Specifications))
		copy(specs, c.workUnit.Specifications)
	}
	workDir := c.getWorkDir()
	c.mu.RUnlock()
	allFindings := executeGuardrails(ctx, cfg.Pre, phase, workDir, specs)

	if len(allFindings) == 0 {
		return nil
	}

	// Check for blocking findings (High or Critical).
	var blocking []string
	for _, f := range allFindings {
		slog.Info("pre-guardrail finding",
			"phase", phase,
			"rule", f.Rule,
			"severity", f.Severity.String(),
			"message", f.Message,
		)

		if f.Severity <= findings.SeverityHigh {
			blocking = append(blocking, fmt.Sprintf("[%s] %s", f.Rule, f.Message))
		}
	}

	if len(blocking) > 0 {
		return fmt.Errorf("pre-guardrail blocked %s: %s", phase, strings.Join(blocking, "; "))
	}

	return nil
}

// runPostGuardrails executes post-phase guardrails configured for the given phase.
// Returns findings for informational purposes; post-guardrails are non-blocking by default.
// Caller must hold c.mu.
//
//nolint:unused // wired once the phase router is implemented (#22)
func (c *Conductor) runPostGuardrails(ctx context.Context, phase string) []findings.Finding {
	s := c.getEffectiveSettings()
	if s == nil {
		return nil
	}

	cfg, ok := s.Workflow.PhaseGuardrails[phase]
	if !ok || len(cfg.Post) == 0 {
		return nil
	}

	var specs []string
	if c.workUnit != nil {
		specs = make([]string, len(c.workUnit.Specifications))
		copy(specs, c.workUnit.Specifications)
	}

	workDir := c.getWorkDir()
	result := executeGuardrails(ctx, cfg.Post, phase, workDir, specs)

	for _, f := range result {
		slog.Info("post-guardrail finding",
			"phase", phase,
			"rule", f.Rule,
			"severity", f.Severity.String(),
			"message", f.Message,
		)
	}

	return result
}

// executeGuardrails resolves and runs a list of guardrails by name.
func executeGuardrails(ctx context.Context, names []string, phase, workDir string, specs []string) []findings.Finding {
	var result []findings.Finding

	for _, name := range names {
		g, ok := policy.GetGuardrail(name)
		if !ok {
			slog.Warn("unknown guardrail, skipping", "name", name, "phase", phase)

			continue
		}

		ff := g.Check(ctx, phase, workDir, specs)
		result = append(result, ff...)
	}

	return result
}
