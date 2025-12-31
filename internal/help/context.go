// Package help provides context-aware help for CLI commands.
package help

import (
	"os"

	"github.com/valksor/go-mehrhof/internal/storage"
)

// HelpContext holds information about the current workspace state
// for determining command availability.
type HelpContext struct {
	HasWorkspace      bool
	HasActiveTask     bool
	TaskID            string
	TaskState         string
	HasSpecifications bool
	UseGit            bool
}

// LoadContext loads the current workspace context for help display.
// This is a lightweight operation that avoids full conductor initialization.
func LoadContext() *HelpContext {
	ctx := &HelpContext{}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return ctx
	}

	// Try to open workspace
	ws, err := storage.OpenWorkspace(cwd)
	if err != nil {
		return ctx
	}
	ctx.HasWorkspace = true

	// Check for active task
	if !ws.HasActiveTask() {
		return ctx
	}

	active, err := ws.LoadActiveTask()
	if err != nil || active == nil {
		return ctx
	}

	ctx.HasActiveTask = true
	ctx.TaskID = active.ID
	ctx.TaskState = active.State
	ctx.UseGit = active.UseGit

	// Check for specifications
	specs, err := ws.ListSpecifications(active.ID)
	if err == nil && len(specs) > 0 {
		ctx.HasSpecifications = true
	}

	return ctx
}
