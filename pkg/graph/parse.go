package graph

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/valksor/kvelmo/pkg/worker"
)

// GraphDef is the YAML-serializable definition of a graph.
type GraphDef struct {
	Phase string    `yaml:"phase"`
	Nodes []NodeDef `yaml:"nodes"`
}

// NodeDef is the YAML-serializable definition of a single graph node.
type NodeDef struct {
	ID         string   `yaml:"id"`
	Label      string   `yaml:"label"`
	JobType    string   `yaml:"job_type"`
	Prompt     string   `yaml:"prompt"`
	DependsOn  []string `yaml:"depends_on"`
	Frozen     bool     `yaml:"frozen"`
	FailBranch string   `yaml:"fail_branch"`

	// Iteration
	MaxIterations int `yaml:"max_iterations"`

	// Error strategy
	ErrorStrategy string        `yaml:"error_strategy"` // "fail", "default_value", "retry_then_fail"
	DefaultOutput string        `yaml:"default_output"`
	MaxRetries    int           `yaml:"max_retries"`
	RetryDelay    time.Duration `yaml:"retry_delay"`

	// Approval gate
	RequiresApproval bool          `yaml:"requires_approval"`
	ApprovalPrompt   string        `yaml:"approval_prompt"`
	ApprovalTimeout  time.Duration `yaml:"approval_timeout"`

	// Sub-task (mutually exclusive with Prompt)
	SubTask *SubTaskConfig `yaml:"sub_task"`
}

// ParseGraphDef parses a YAML graph definition and returns a validated Graph.
// IterationCheck functions cannot be expressed in YAML; nodes with MaxIterations
// will always iterate (the check always returns true). Use Build() for
// programmatic graphs that need custom iteration checks.
func ParseGraphDef(data []byte) (*Graph, error) {
	var def GraphDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("graph: parse yaml: %w", err)
	}

	return BuildFromDef(&def)
}

// ParseGraphDefFile reads and parses a YAML graph definition from a file.
func ParseGraphDefFile(path string) (*Graph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("graph: read file: %w", err)
	}

	return ParseGraphDef(data)
}

// BuildFromDef constructs a Graph from a parsed GraphDef.
func BuildFromDef(def *GraphDef) (*Graph, error) {
	if len(def.Nodes) == 0 {
		return nil, errors.New("graph: definition has no nodes")
	}

	g := New()

	for i, nd := range def.Nodes {
		if nd.ID == "" {
			return nil, fmt.Errorf("graph: node %d has no id", i)
		}

		node := &Node{
			ID:            NodeID(nd.ID),
			Label:         nd.Label,
			JobType:       resolveJobType(nd.JobType),
			Prompt:        nd.Prompt,
			Frozen:        nd.Frozen,
			MaxIterations: nd.MaxIterations,
			DefaultOutput: nd.DefaultOutput,
			MaxRetries:    nd.MaxRetries,
			RetryDelay:    nd.RetryDelay,
			SubTask:       nd.SubTask,
		}

		if node.Label == "" {
			node.Label = nd.ID
		}

		// Dependencies
		for _, dep := range nd.DependsOn {
			node.DependsOn = append(node.DependsOn, NodeID(dep))
		}

		// Fail branch
		if nd.FailBranch != "" {
			fb := NodeID(nd.FailBranch)
			node.FailBranch = &fb
		}

		// Error strategy
		node.ErrorStrategy = resolveErrorStrategy(nd.ErrorStrategy)

		// Iteration: YAML-defined iteration always returns true (re-execute until max).
		if nd.MaxIterations > 0 {
			node.IterationCheck = func(string) bool { return true }
		}

		// Approval gate
		node.RequiresApproval = nd.RequiresApproval
		node.ApprovalPrompt = nd.ApprovalPrompt
		node.ApprovalTimeout = nd.ApprovalTimeout

		if err := g.AddNode(node); err != nil {
			return nil, fmt.Errorf("graph: add node %q: %w", nd.ID, err)
		}
	}

	if err := g.Validate(); err != nil {
		return nil, fmt.Errorf("graph: validate: %w", err)
	}

	return g, nil
}

// ExpandPromptVars replaces {{var}} placeholders in all node prompts using the
// provided variable map. Unknown variables are left as-is.
func ExpandPromptVars(g *Graph, vars map[string]string) {
	for _, node := range g.Nodes {
		node.Prompt = expandVars(node.Prompt, vars)
	}
}

func expandVars(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}

	return s
}

func resolveJobType(s string) worker.JobType {
	switch strings.ToLower(s) {
	case "plan":
		return worker.JobTypePlan
	case "implement":
		return worker.JobTypeImplement
	case "review":
		return worker.JobTypeReview
	case "simplify":
		return worker.JobTypeSimplify
	case "optimize":
		return worker.JobTypeOptimize
	case "dry_run", "dryrun":
		return worker.JobTypeDryRun
	default:
		slog.Warn("unknown job type in graph definition, using as-is", "job_type", s)

		return worker.JobType(s)
	}
}

func resolveErrorStrategy(s string) ErrorStrategy {
	switch strings.ToLower(s) {
	case "default_value":
		return ErrorDefaultValue
	case "retry_then_fail":
		return ErrorRetryThenFail
	default:
		return ErrorFail
	}
}
