package conductor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/valksor/kvelmo/pkg/findings"
	"github.com/valksor/kvelmo/pkg/settings"
	"github.com/valksor/kvelmo/pkg/storage"
	"github.com/valksor/kvelmo/pkg/worker"
)

// adversarialTimeout is the maximum time to wait for all adversarial agents to finish.
const adversarialTimeout = 10 * time.Minute

// runAdversarialReview dispatches review jobs with persona-specific prompts
// and collects findings. Runs personas in parallel.
// Must be called without c.mu held.
func (c *Conductor) runAdversarialReview(ctx context.Context) ([]findings.Finding, error) {
	cfg := c.getAdversarialConfig()
	if cfg == nil || !cfg.Enabled || len(cfg.Personas) == 0 {
		return nil, nil
	}

	allPersonas := DefaultPersonas()
	var activePersonas []ReviewPersona

	for _, name := range cfg.Personas {
		p, ok := allPersonas[name]
		if !ok {
			slog.Warn("unknown adversarial persona, skipping", "persona", name)
			continue
		}

		activePersonas = append(activePersonas, p)
	}

	if len(activePersonas) == 0 {
		return nil, errors.New("no valid personas configured")
	}

	c.mu.RLock()
	pool := c.pool
	workDir := c.getWorkDir()
	wu := c.workUnit
	st := c.store
	c.mu.RUnlock()

	if pool == nil {
		return nil, errors.New("no worker pool available")
	}

	// Get the diff for review context.
	diff := c.getAdversarialDiff(ctx)

	// Get the specification content for context.
	specContent := c.getAdversarialSpec(wu, st)

	c.emit(ConductorEvent{
		Type:    "adversarial_review_started",
		Message: fmt.Sprintf("Starting adversarial review with %d personas", len(activePersonas)),
	})

	ctx, cancel := context.WithTimeout(ctx, adversarialTimeout)
	defer cancel()

	// Dispatch review jobs in parallel, one per persona.
	type personaResult struct {
		persona string
		job     *worker.Job
		err     error
	}

	var wg sync.WaitGroup

	results := make([]personaResult, len(activePersonas))

	agentName := cfg.Agent

	for i, persona := range activePersonas {
		wg.Add(1)

		go func(idx int, p ReviewPersona) {
			defer wg.Done()

			prompt := buildAdversarialPrompt(p, diff, specContent)

			opts := &worker.JobOptions{
				WorkDir: workDir,
			}
			if agentName != "" {
				opts.Agent = agentName
			}

			job, err := pool.SubmitWithOptions(worker.JobTypeReview, workDir, prompt, opts)
			results[idx] = personaResult{persona: p.Name, job: job, err: err}
		}(i, persona)
	}

	wg.Wait()

	// Wait for all submitted jobs to complete and collect findings.
	groups := make(map[string][]findings.Finding)

	for _, r := range results {
		if r.err != nil {
			slog.Warn("adversarial persona submit failed", "persona", r.persona, "error", r.err)
			continue
		}

		job := c.waitForJobCompletion(ctx, pool, r.job.ID)
		if job == nil || job.Status == worker.JobStatusFailed {
			errMsg := ""
			if job != nil {
				errMsg = job.Error
			}

			slog.Warn("adversarial persona job failed", "persona", r.persona, "error", errMsg)
			continue
		}

		personaFindings := parseAdversarialFindings(r.persona, job.Result)
		groups[r.persona] = personaFindings

		slog.Info("adversarial persona completed", "persona", r.persona, "findings", len(personaFindings))
	}

	if len(groups) == 0 {
		return nil, errors.New("all adversarial personas failed")
	}

	merged := findings.DeduplicateFindings(groups)

	slog.Info("adversarial review complete",
		"personas_responded", len(groups),
		"total_findings", len(merged),
	)

	c.emit(ConductorEvent{
		Type:    "adversarial_review_complete",
		Message: fmt.Sprintf("Adversarial review found %d findings from %d personas", len(merged), len(groups)),
	})

	return merged, nil
}

// getAdversarialConfig returns the adversarial review config from effective settings.
func (c *Conductor) getAdversarialConfig() *settings.AdversarialReviewSettings {
	s := c.getEffectiveSettings()
	if s == nil {
		return nil
	}

	return s.Workflow.AdversarialReview
}

// getAdversarialDiff retrieves the git diff for adversarial review context.
func (c *Conductor) getAdversarialDiff(ctx context.Context) string {
	if c.git == nil {
		return ""
	}

	base, err := c.getBaseBranch(ctx)
	if err != nil || base == "" {
		return ""
	}

	diff, err := c.git.DiffAgainst(ctx, "origin/"+base, false)
	if err != nil {
		return ""
	}

	// Truncate diff to avoid overwhelming the agent.
	const maxDiffLen = 50000

	diffRunes := []rune(diff)
	if len(diffRunes) > maxDiffLen {
		return string(diffRunes[:maxDiffLen]) + "\n\n...(diff truncated)"
	}

	return diff
}

// getAdversarialSpec retrieves the specification content for adversarial review.
func (c *Conductor) getAdversarialSpec(wu *WorkUnit, st *storage.Store) string {
	if wu == nil || st == nil {
		return ""
	}

	specStore := storage.NewSpecStore(st)

	content, _, err := specStore.GetLatestSpecificationContent(wu.ID)
	if err != nil || content == "" {
		return ""
	}

	const maxSpecLen = 20000

	specRunes := []rune(content)
	if len(specRunes) > maxSpecLen {
		return string(specRunes[:maxSpecLen]) + "\n\n...(specification truncated)"
	}

	return content
}

// buildAdversarialPrompt constructs a review prompt combining persona instructions,
// the git diff, and specification content.
func buildAdversarialPrompt(persona ReviewPersona, diff, spec string) string {
	prompt := persona.Prompt + "\n\n"

	if spec != "" {
		prompt += "## Specification\n\n" + spec + "\n\n"
	}

	if diff != "" {
		prompt += "## Changes (diff)\n\n" + diff + "\n\n"
	}

	prompt += `## Output Format

Report each finding on its own line with the format:
file.go:42: [severity] description

Where severity is one of: critical, high, medium, low`

	return prompt
}

// parseAdversarialFindings extracts findings from a persona agent's review output.
// Each finding is attributed to the persona via the ReviewerPersona field.
func parseAdversarialFindings(persona, output string) []findings.Finding {
	raw := parseConsensusFindings(persona, output)

	// Attribute each finding to the persona.
	for i := range raw {
		raw[i].ReviewerPersona = persona
	}

	return raw
}

// GetAdversarialFindings returns the most recent adversarial review findings.
func (c *Conductor) GetAdversarialFindings() []findings.Finding {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return slices.Clone(c.adversarialFindings)
}

// RunAdversarialReview triggers an adversarial review manually and returns findings.
func (c *Conductor) RunAdversarialReview(ctx context.Context) ([]findings.Finding, error) {
	result, err := c.runAdversarialReview(ctx)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.adversarialFindings = result
	c.mu.Unlock()

	return result, nil
}
