package conductor

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/settings"
)

const hookTimeout = 60 * time.Second

// RunTransitionHooks executes all hooks configured for the given event.
// Returns an error if any required hook fails. Non-required hook failures are logged.
func (c *Conductor) RunTransitionHooks(ctx context.Context, event Event) error {
	s := c.getEffectiveSettings()
	if s == nil || s.Workflow.Hooks == nil {
		return nil
	}

	hookKey := "pre_" + string(event)
	hooks := s.Workflow.Hooks[hookKey]
	if len(hooks) == 0 {
		return nil
	}

	workDir := c.getWorkDir()
	for _, hook := range hooks {
		if err := executeHook(ctx, hook, workDir); err != nil {
			if hook.Required {
				return fmt.Errorf("required hook %q failed: %w", hookDescription(hook), err)
			}
			slog.Warn("optional hook failed", "hook", hookDescription(hook), "error", err)
		}
	}

	return nil
}

// executeHook runs a single shell command and returns an error if it fails.
func executeHook(ctx context.Context, hook settings.TransitionHook, workDir string) error {
	ctx, cancel := context.WithTimeout(ctx, hookTimeout)
	defer cancel()

	desc := hookDescription(hook)
	slog.Info("running transition hook", "description", desc, "required", hook.Required)

	cmd := exec.CommandContext(ctx, "sh", "-c", hook.Command)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\nOutput: %s", desc, err, strings.TrimSpace(string(output)))
	}

	return nil
}

// hookDescription returns a human-readable description for a hook.
func hookDescription(hook settings.TransitionHook) string {
	if hook.Description != "" {
		return hook.Description
	}

	return hook.Command
}

// ListHooks returns all configured hooks grouped by transition event.
func (c *Conductor) ListHooks() map[string][]TransitionHookStatus {
	s := c.getEffectiveSettings()
	if s == nil || s.Workflow.Hooks == nil {
		return nil
	}

	result := make(map[string][]TransitionHookStatus)
	for key, hooks := range s.Workflow.Hooks {
		for _, h := range hooks {
			result[key] = append(result[key], TransitionHookStatus{
				Command:     h.Command,
				Description: h.Description,
				Required:    h.Required,
			})
		}
	}

	return result
}

// TransitionHookStatus is the serializable hook info for API responses.
type TransitionHookStatus struct {
	Command     string `json:"command"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}
