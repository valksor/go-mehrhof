package agent

import (
	"context"

	_maps "maps"
	_slices "slices"
)

// AliasAgent wraps an existing agent with pre-configured environment variables
// and CLI arguments. This allows users to define custom agent configurations in
// their workspace config without modifying the core agent code.
type AliasAgent struct {
	name        string
	description string
	base        Agent
	env         map[string]string
	args        []string
}

// NewAlias creates a new alias agent that wraps an existing agent with
// pre-configured environment variables and CLI arguments.
func NewAlias(name string, base Agent, env map[string]string, args []string, description string) *AliasAgent {
	return &AliasAgent{
		name:        name,
		description: description,
		base:        base,
		env:         env,
		args:        args,
	}
}

// Name returns the alias name
func (a *AliasAgent) Name() string {
	return a.name
}

// Description returns the human-readable description
func (a *AliasAgent) Description() string {
	return a.description
}

// BaseAgent returns the underlying agent being wrapped
func (a *AliasAgent) BaseAgent() Agent {
	return a.base
}

// configured returns the base agent with env vars and args applied.
func (a *AliasAgent) configured() Agent {
	agent := a.base
	for k, v := range a.env {
		agent = agent.WithEnv(k, v)
	}
	if len(a.args) > 0 {
		agent = agent.WithArgs(a.args...)
	}
	return agent
}

// Run executes the prompt by delegating to the base agent with env vars and args applied
func (a *AliasAgent) Run(ctx context.Context, prompt string) (*Response, error) {
	return a.configured().Run(ctx, prompt)
}

// RunStream executes the prompt and streams events
func (a *AliasAgent) RunStream(ctx context.Context, prompt string) (<-chan Event, <-chan error) {
	return a.configured().RunStream(ctx, prompt)
}

// RunWithCallback executes with a callback for each event
func (a *AliasAgent) RunWithCallback(ctx context.Context, prompt string, cb StreamCallback) (*Response, error) {
	return a.configured().RunWithCallback(ctx, prompt, cb)
}

// Available checks if the base agent is available
func (a *AliasAgent) Available() error {
	return a.base.Available()
}

// WithEnv adds an additional environment variable to the alias.
// This creates a new AliasAgent with the combined environment variables.
func (a *AliasAgent) WithEnv(key, value string) Agent {
	newEnv := _maps.Clone(a.env)
	newEnv[key] = value
	return &AliasAgent{
		name:        a.name,
		description: a.description,
		base:        a.base,
		env:         newEnv,
		args:        a.args, // Preserve args
	}
}

// WithArgs adds additional CLI arguments to the alias.
// This creates a new AliasAgent with the combined arguments.
func (a *AliasAgent) WithArgs(args ...string) Agent {
	newArgs := _slices.Concat(a.args, args)
	return &AliasAgent{
		name:        a.name,
		description: a.description,
		base:        a.base,
		env:         a.env,
		args:        newArgs,
	}
}

// Ensure AliasAgent implements Agent interface
var _ Agent = (*AliasAgent)(nil)
