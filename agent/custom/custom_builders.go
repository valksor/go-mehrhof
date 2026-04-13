package custom

import (
	"maps"
	"time"

	"github.com/valksor/kvelmo/agent"
)

// WithEnv returns a new Agent with an added environment variable.
func (a *Agent) WithEnv(key, value string) agent.Agent {
	newCfg := a.config
	if newCfg.Environment == nil {
		newCfg.Environment = make(map[string]string)
	}
	env := make(map[string]string, len(a.config.Environment)+1)
	maps.Copy(env, a.config.Environment)
	env[key] = value
	newCfg.Environment = env

	return NewWithConfig(newCfg)
}

// WithArgs returns a new Agent with additional CLI arguments.
func (a *Agent) WithArgs(args ...string) agent.Agent {
	newCfg := a.config
	newArgs := make([]string, len(a.config.Args)+len(args))
	copy(newArgs, a.config.Args)
	copy(newArgs[len(a.config.Args):], args)
	newCfg.Args = newArgs

	return NewWithConfig(newCfg)
}

// WithWorkDir returns a new Agent with a different working directory.
func (a *Agent) WithWorkDir(dir string) agent.Agent {
	newCfg := a.config
	newCfg.WorkDir = dir

	return NewWithConfig(newCfg)
}

// WithTimeout returns a new Agent with a different timeout.
func (a *Agent) WithTimeout(d time.Duration) agent.Agent {
	newCfg := a.config
	newCfg.Timeout = d

	return NewWithConfig(newCfg)
}
