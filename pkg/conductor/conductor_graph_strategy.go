package conductor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/valksor/kvelmo/pkg/agent/strategy"
	"github.com/valksor/kvelmo/pkg/discovery"
	"github.com/valksor/kvelmo/pkg/varpool"
)

// resolveAgent returns the agent name for a given phase.
// Checks per-phase overrides in settings, then returns empty (use default worker).
// Must be called with c.mu held (at least RLock).
func (c *Conductor) resolveAgent(phase string) string {
	if s := c.getEffectiveSettings(); s != nil {
		if agent, ok := s.Agent.PhaseAgent[phase]; ok && agent != "" {
			return agent
		}
	}

	return ""
}

// resolveStrategy returns the strategy for a given phase.
// Checks per-phase overrides first, then conductor default, then global default.
// Must be called with c.mu held (at least RLock).
func (c *Conductor) resolveStrategy(phase string) strategy.Strategy {
	if s, ok := c.phaseStrategies[phase]; ok {
		return s
	}
	if c.strategy != nil {
		return c.strategy
	}

	return strategy.Default()
}

// applyStrategy wraps a raw prompt through the resolved strategy for the phase.
// Must be called with c.mu held (at least RLock).
// applyStrategy wraps a raw prompt with the phase strategy and phase-aware context.
// It also builds phase-aware context using the context profile for the phase
// and emits context_metrics as a ConductorEvent.
func (c *Conductor) applyStrategy(ctx context.Context, phase, prompt string) string {
	s := c.resolveStrategy(phase)

	var varSummary string
	if c.varPool != nil {
		if summary := c.varPool.Summary(); summary != "" {
			varSummary = "\n" + summary
		}
	}

	// Build phase-aware context from the profile.
	var phaseContext string
	profiles := DefaultContextProfiles()
	if profile, ok := profiles[phase]; ok {
		var metrics ContextMetrics
		deps := c.buildContextDeps()
		phaseContext, metrics = BuildPhaseContext(ctx, profile, c.workUnit, c.varPool, deps)

		// Emit context metrics as an event (non-blocking, best-effort).
		if metricsData, err := json.Marshal(metrics); err == nil {
			go c.emit(ConductorEvent{
				Type:    "context_metrics",
				Message: fmt.Sprintf("Phase %s context: %d tokens used, %d sections", phase, metrics.TokensUsed, len(metrics.SectionsIncluded)),
				Data:    metricsData,
			})
		}
	}

	return s.BuildPrompt(strategy.Input{
		Task:    prompt,
		Phase:   phase,
		Context: phaseContext + varSummary,
	})
}

// VarPool returns the conductor's variable pool.
// The pool is initialized when the conductor is created and persisted
// alongside task state.
func (c *Conductor) VarPool() *varpool.Pool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.varPool
}

// populateStandardVars sets standard variables from the current work unit.
// Must be called with c.mu held.
func (c *Conductor) populateStandardVars() {
	if c.workUnit == nil || c.varPool == nil {
		return
	}

	// Scoped system variables (new convention).
	c.varPool.SetScoped(varpool.ScopeSystem, "task_id", c.workUnit.ID, "conductor")
	c.varPool.SetScoped(varpool.ScopeSystem, "task_title", c.workUnit.Title, "conductor")
	c.varPool.SetScoped(varpool.ScopeSystem, "task_description", c.workUnit.Description, "conductor")

	if c.workUnit.Branch != "" {
		c.varPool.SetScoped(varpool.ScopeSystem, "branch", c.workUnit.Branch, "conductor")
	}

	// Scan the project directory for available commands (Makefile targets, npm/bun
	// scripts, Taskfile tasks, bin/ executables) so agents know what tools are available.
	if tools := discovery.DiscoverTools(c.getWorkDir()); len(tools) > 0 {
		c.varPool.SetScoped(varpool.ScopeSystem, "project_commands", strings.Join(tools, "\n"), "conductor")
	} else {
		c.varPool.SetScoped(varpool.ScopeSystem, "project_commands", "", "conductor")
	}
}

// persistVarPool saves the variable pool to disk.
// Must be called with c.mu held.
func (c *Conductor) persistVarPool() {
	if c.varPool == nil || c.workUnit == nil || c.store == nil {
		return
	}

	path := filepath.Join(c.store.WorkDir(c.workUnit.ID), "varpool.json")
	if err := c.varPool.Save(path); err != nil {
		slog.Warn("persist varpool failed", "task_id", c.workUnit.ID, "error", err)
	}

	c.workUnit.VarPoolPath = path
}

// loadVarPool loads the variable pool from disk if a path is set.
// Must be called with c.mu held.
func (c *Conductor) loadVarPool() {
	if c.workUnit == nil || c.workUnit.VarPoolPath == "" {
		return
	}

	if c.varPool == nil {
		c.varPool = varpool.New()
	}

	if err := c.varPool.Load(c.workUnit.VarPoolPath); err != nil {
		slog.Debug("load varpool failed (may not exist yet)", "path", c.workUnit.VarPoolPath, "error", err)
	}
}
