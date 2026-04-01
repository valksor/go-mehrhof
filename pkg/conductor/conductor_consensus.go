package conductor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/valksor/kvelmo/pkg/findings"
	"github.com/valksor/kvelmo/pkg/settings"
	"github.com/valksor/kvelmo/pkg/worker"
)

// consensusTimeout is the maximum time to wait for all consensus agents to finish.
const consensusTimeout = 10 * time.Minute

// runConsensusReview dispatches the same review prompt to multiple agents
// in parallel and merges their findings with attribution.
func (c *Conductor) runConsensusReview(ctx context.Context, prompt string) ([]findings.Finding, error) {
	cfg := c.getConsensusConfig()
	if cfg == nil || !cfg.Enabled || len(cfg.Agents) == 0 {
		return nil, errors.New("consensus review not configured or disabled")
	}

	c.mu.RLock()
	pool := c.pool
	workDir := c.getWorkDir()
	c.mu.RUnlock()

	if pool == nil {
		return nil, errors.New("no worker pool available")
	}

	minAgreement := max(cfg.MinAgreement, 1)

	ctx, cancel := context.WithTimeout(ctx, consensusTimeout)
	defer cancel()

	// Dispatch review jobs in parallel, one per agent.
	type agentResult struct {
		agent string
		job   *worker.Job
		err   error
	}

	var wg sync.WaitGroup

	results := make([]agentResult, len(cfg.Agents))

	for i, agentName := range cfg.Agents {
		wg.Add(1)

		go func(idx int, name string) {
			defer wg.Done()

			opts := &worker.JobOptions{
				WorkDir: workDir,
				Agent:   name,
			}

			// Skip response cache for consensus — each agent must produce an independent opinion.
			job, err := pool.SubmitWithOptions(worker.JobTypeReview, workDir, prompt, opts)
			results[idx] = agentResult{agent: name, job: job, err: err}
		}(i, agentName)
	}

	wg.Wait()

	// Wait for all submitted jobs to complete.
	groups := make(map[string][]findings.Finding)

	for _, r := range results {
		if r.err != nil {
			slog.Warn("consensus agent submit failed", "agent", r.agent, "error", r.err)

			continue
		}

		job := c.waitForJobCompletion(ctx, pool, r.job.ID)
		if job == nil || job.Status == worker.JobStatusFailed {
			errMsg := ""
			if job != nil {
				errMsg = job.Error
			}

			slog.Warn("consensus agent job failed", "agent", r.agent, "error", errMsg)

			continue
		}

		agentFindings := parseConsensusFindings(r.agent, job.Result)
		groups[r.agent] = agentFindings

		slog.Info("consensus agent completed", "agent", r.agent, "findings", len(agentFindings))
	}

	if len(groups) == 0 {
		return nil, errors.New("all consensus agents failed")
	}

	merged := findings.DeduplicateFindings(groups)
	filtered := findings.FilterByMinAgreement(merged, minAgreement)

	slog.Info("consensus review complete",
		"agents_responded", len(groups),
		"total_findings", len(merged),
		"after_agreement_filter", len(filtered),
		"min_agreement", minAgreement,
	)

	return filtered, nil
}

// getConsensusConfig returns the consensus config from effective settings.
func (c *Conductor) getConsensusConfig() *settings.ConsensusConfig {
	s := c.getEffectiveSettings()
	if s == nil {
		return nil
	}

	return s.Agent.Consensus
}

// waitForJobCompletion polls a job until it reaches a terminal state or the
// context is cancelled.
func (c *Conductor) waitForJobCompletion(ctx context.Context, pool *worker.Pool, jobID string) *worker.Job {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			job := pool.GetJob(jobID)
			if job == nil {
				return nil
			}

			if job.Status == worker.JobStatusDone || job.Status == worker.JobStatusFailed {
				return job
			}
		}
	}
}

// parseConsensusFindings extracts findings from an agent's review output.
// Each non-empty line that contains a file reference is treated as a finding.
// This is a best-effort parser; structured output from agents would improve accuracy.
func parseConsensusFindings(agentName, output string) []findings.Finding {
	if output == "" {
		return nil
	}

	const maxFindings = 500

	var result []findings.Finding

	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if len(result) >= maxFindings {
			slog.Warn("consensus findings truncated", "agent", agentName, "max", maxFindings)

			break
		}

		f := findings.Finding{
			Source:     agentName,
			Message:    line,
			Category:   findings.CategoryQuality,
			Severity:   findings.SeverityMedium,
			DetectedBy: []string{agentName},
		}

		// Try to extract file:line from common patterns like "path/to/file.go:42:"
		if file, lineNum, rule, ok := parseFileLineRule(line); ok {
			f.File = file
			f.Line = lineNum
			f.Rule = rule
		}

		result = append(result, f)
	}

	return result
}

// parseFileLineRule attempts to extract file, line number, and rule from a line.
// Returns ok=false if the line doesn't match a recognizable pattern.
func parseFileLineRule(line string) (string, int, string, bool) {
	// Pattern: "file.go:42: message" or "file.go:42:13: message"
	parts := strings.SplitN(line, ":", 4)
	if len(parts) < 3 {
		return "", 0, "", false
	}

	file := strings.TrimSpace(parts[0])
	if file == "" || strings.ContainsAny(file, " \t") {
		return "", 0, "", false
	}

	var n int

	_, err := fmt.Sscanf(parts[1], "%d", &n)
	if err != nil {
		return "", 0, "", false
	}

	// Use remaining text as the rule identifier.
	var remaining string

	if len(parts) >= 4 {
		remaining = strings.TrimSpace(parts[3])
	} else {
		remaining = strings.TrimSpace(parts[2])
	}

	// Take first few words as rule key.
	words := strings.Fields(remaining)
	if len(words) > 3 {
		words = words[:3]
	}

	rule := strings.Join(words, " ")

	return file, n, rule, true
}
