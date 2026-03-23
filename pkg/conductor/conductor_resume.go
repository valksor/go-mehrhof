package conductor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/valksor/kvelmo/pkg/agent/recorder"
)

// ResumeFromCheckpoint navigates to a checkpoint and restarts the associated
// phase with prior agent reasoning injected as context. This enables
// "rerun-from-any-step" debugging by combining code state rollback with
// agent context reconstruction.
func (c *Conductor) ResumeFromCheckpoint(ctx context.Context, sha string) error {
	c.mu.RLock()
	if c.workUnit == nil {
		c.mu.RUnlock()

		return errors.New("no task loaded")
	}

	// Find the phase metrics entry that matches this checkpoint.
	var targetPhase string
	var recordingPath string
	for phase, pm := range c.workUnit.PhaseMetrics {
		if pm.CheckpointSHA == sha {
			targetPhase = phase
			recordingPath = pm.RecordingPath

			break
		}
	}
	c.mu.RUnlock()

	if targetPhase == "" {
		return fmt.Errorf("no phase found for checkpoint %s", sha)
	}

	// Navigate to the checkpoint (code state rollback).
	// GotoCheckpoint handles its own locking.
	if err := c.GotoCheckpoint(ctx, sha); err != nil {
		return fmt.Errorf("goto checkpoint: %w", err)
	}

	slog.Info("resuming from checkpoint",
		"sha", sha,
		"phase", targetPhase,
		"recording", recordingPath,
	)

	// Build prior context from the recording and inject into varpool
	// so applyStrategy picks it up in the next phase prompt.
	if recordingPath != "" {
		if priorContext := buildPriorContext(recordingPath, targetPhase); priorContext != "" {
			c.mu.Lock()
			if c.varPool != nil {
				c.varPool.SetScoped("sys", "prior_context", priorContext, "resume")
			}
			c.mu.Unlock()
			slog.Info("injected prior agent context into varpool", "phase", targetPhase, "context_len", len(priorContext))
		}
	}

	// Re-dispatch the phase without holding the lock.
	// dispatchAutoAdvance calls Implement/Simplify/etc which acquire c.mu.Lock internally.
	go c.dispatchAutoAdvance(ctx, targetPhase)

	return nil
}

// buildPriorContext extracts a summary of agent reasoning from a recording file.
// Returns a string suitable for injection into the agent's next prompt.
func buildPriorContext(recordingPath, phase string) string {
	sp, err := recorder.BuildScratchpadFromFile(recordingPath)
	if err != nil {
		slog.Debug("could not build scratchpad for prior context", "error", err)

		return ""
	}

	if sp == nil || len(sp.Thoughts) == 0 {
		return ""
	}

	// Build a summary of the agent's prior reasoning.
	var summary string
	summary += fmt.Sprintf("## Prior %s Phase Context\n\n", phase)
	summary += "The following is a summary of the agent's previous reasoning during this phase:\n\n"

	for _, t := range sp.Thoughts {
		switch t.Type {
		case "decision":
			summary += fmt.Sprintf("- Decision: %s\n", truncate(t.Content, 200))
		case "reasoning":
			summary += fmt.Sprintf("- Reasoning: %s\n", truncate(t.Content, 200))
		}
	}

	return summary
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen] + "..."
}
