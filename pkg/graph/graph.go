// Package graph provides a dependency graph for scheduling parallel sub-tasks
// within kvelmo's phase-based task lifecycle. Inspired by Dify's queue-based
// graph scheduling pattern.
package graph

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/valksor/kvelmo/pkg/worker"
)

// ErrorStrategy defines how a node handles execution failures.
type ErrorStrategy int

const (
	ErrorFail          ErrorStrategy = iota // Default: mark failed, cascade skip to dependents
	ErrorDefaultValue                       // Use DefaultOutput as result, mark done
	ErrorRetryThenFail                      // Retry up to MaxRetries times, then fail
)

// IterationCheck decides whether a completed node should re-execute.
// It receives the node's output and returns true if another iteration is needed.
type IterationCheck func(output string) bool

// NodeID uniquely identifies a node within a graph.
type NodeID string

// ConditionFunc decides whether a node should execute based on completed results.
// Returning false causes the node to be skipped.
type ConditionFunc func(results map[NodeID]string) bool

// SubTaskConfig defines a sub-task to be spawned as a graph node.
// Instead of running an agent prompt, the node creates an isolated worktree
// and runs the specified lifecycle phases within it.
type SubTaskConfig struct {
	Title       string            `json:"title"       yaml:"title"`
	Description string            `json:"description" yaml:"description"`
	Phases      []string          `json:"phases"      yaml:"phases"`   // e.g., ["plan", "implement"]
	Branch      string            `json:"branch"      yaml:"branch"`   // branch name for sub-task worktree
	Metadata    map[string]string `json:"metadata"    yaml:"metadata"` // passed to sub-task conductor
}

// Node represents a schedulable unit of work within a phase.
type Node struct {
	ID         NodeID
	Label      string         // Human-readable description
	JobType    worker.JobType // plan, implement, review, etc.
	Prompt     string
	DependsOn  []NodeID      // Nodes that must complete before this one runs
	Condition  ConditionFunc // Optional: skip if returns false (nil = always run)
	Frozen     bool          // If true, return cached result instead of re-executing (Langflow pattern)
	FailBranch *NodeID       // Optional: route to this node on failure instead of cascading skip

	// Iteration: re-execute the node until IterationCheck returns false or
	// MaxIterations is reached. 0 means no iteration (execute once).
	MaxIterations  int            // Max iteration count (0 = no loop)
	IterationCheck IterationCheck // Check output; return true to re-execute

	// Error strategy: controls how failures are handled before fail-branch routing.
	ErrorStrategy ErrorStrategy // Default: ErrorFail
	DefaultOutput string        // Fallback result when ErrorStrategy == ErrorDefaultValue
	MaxRetries    int           // Retry count when ErrorStrategy == ErrorRetryThenFail
	RetryDelay    time.Duration // Delay between retries

	// Approval gate: pause execution until human approves.
	RequiresApproval bool          // If true, emit approval event and wait before executing
	ApprovalPrompt   string        // Shown to user for context
	ApprovalTimeout  time.Duration // Auto-reject after timeout (0 = wait forever)

	// SubTask, if set, makes this node spawn a sub-task lifecycle instead of
	// submitting an agent prompt. The sub-task runs in an isolated worktree.
	SubTask *SubTaskConfig `json:"sub_task,omitempty"`
}

// IsSubTask returns true if the node spawns a sub-task lifecycle.
func (n *Node) IsSubTask() bool {
	return n.SubTask != nil
}

// EdgeState tracks the execution state of an edge between two nodes.
// Inspired by Dify's TAKEN/SKIPPED/UNKNOWN model.
type EdgeState int

const (
	EdgeUnknown EdgeState = iota // Not yet resolved
	EdgeTaken                    // Dependency completed successfully
	EdgeSkipped                  // Dependency was skipped (branch unreachable)
)

// Edge represents a dependency between two nodes (From must complete before To starts).
type Edge struct {
	From NodeID
	To   NodeID
}

// Graph holds a set of nodes with dependency edges.
type Graph struct {
	Nodes     map[NodeID]*Node
	edges     []Edge            // derived from node DependsOn
	failEdges map[NodeID]NodeID // source → fail-branch target
	depCounts map[NodeID]int    // cached transitive dependent counts
}

// New creates an empty graph.
func New() *Graph {
	return &Graph{
		Nodes: make(map[NodeID]*Node),
	}
}

// AddNode adds a node to the graph. Returns an error if the ID already exists.
func (g *Graph) AddNode(n *Node) error {
	if _, exists := g.Nodes[n.ID]; exists {
		return fmt.Errorf("graph: duplicate node ID %q", n.ID)
	}

	g.Nodes[n.ID] = n

	return nil
}

// SingleNode creates a graph with one node and no dependencies.
// This is the default for backward-compatible single-job phases.
func SingleNode(id NodeID, label string, jobType worker.JobType, prompt string) *Graph {
	g := New()
	_ = g.AddNode(&Node{
		ID:      id,
		Label:   label,
		JobType: jobType,
		Prompt:  prompt,
	})

	return g
}

// buildEdges derives edges from node DependsOn declarations and fail-branch mappings.
func (g *Graph) buildEdges() {
	g.edges = nil
	g.failEdges = make(map[NodeID]NodeID)

	for _, node := range g.Nodes {
		for _, dep := range node.DependsOn {
			g.edges = append(g.edges, Edge{From: dep, To: node.ID})
		}

		if node.FailBranch != nil {
			g.failEdges[node.ID] = *node.FailBranch
		}
	}
}

// FailBranchTarget returns the fail-branch target for a node, if configured.
func (g *Graph) FailBranchTarget(id NodeID) (NodeID, bool) {
	target, ok := g.failEdges[id]

	return target, ok
}

// IsFailBranchEdge returns true if the edge from→to is a fail-branch edge.
func (g *Graph) IsFailBranchEdge(from, to NodeID) bool {
	target, ok := g.failEdges[from]

	return ok && target == to
}

// FailErrorKey returns the results-map key that holds a failed node's error message.
// Fail-branch handler nodes can read results[FailErrorKey(sourceID)] to inspect the error.
func FailErrorKey(id NodeID) NodeID {
	return NodeID("__fail_error:" + string(id))
}

// Roots returns nodes with no dependencies (entry points).
func (g *Graph) Roots() []NodeID {
	var roots []NodeID

	for _, node := range g.Nodes {
		if len(node.DependsOn) == 0 {
			roots = append(roots, node.ID)
		}
	}

	slices.Sort(roots)

	return roots
}

// Dependents returns nodes that directly depend on the given node.
func (g *Graph) Dependents(id NodeID) []NodeID {
	var deps []NodeID

	for _, node := range g.Nodes {
		if slices.Contains(node.DependsOn, id) {
			deps = append(deps, node.ID)
		}
	}

	slices.Sort(deps)

	return deps
}

// TransitiveDepCount returns how many nodes transitively depend on the given node.
// Computed during Validate() and cached. Returns 0 for unknown/leaf nodes.
func (g *Graph) TransitiveDepCount(id NodeID) int {
	return g.depCounts[id]
}

// computeDepCounts calculates transitive dependent counts for all nodes.
// Builds a reverse adjacency map once, then BFS-counts reachable descendants
// per node in O(N * (N + E)) total with O(1) per-edge lookups.
func (g *Graph) computeDepCounts() {
	g.depCounts = make(map[NodeID]int, len(g.Nodes))

	// Build reverse adjacency map (node → direct dependents) once.
	reverse := make(map[NodeID][]NodeID, len(g.Nodes))
	for _, node := range g.Nodes {
		for _, dep := range node.DependsOn {
			reverse[dep] = append(reverse[dep], node.ID)
		}
	}

	for id := range g.Nodes {
		visited := make(map[NodeID]bool)
		queue := append([]NodeID(nil), reverse[id]...)
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			if visited[cur] {
				continue
			}
			visited[cur] = true
			queue = append(queue, reverse[cur]...)
		}
		g.depCounts[id] = len(visited)
	}
}

// Validate checks the graph for errors:
//   - All DependsOn references point to existing nodes
//   - No cycles exist
//   - At least one node exists
func (g *Graph) Validate() error {
	if len(g.Nodes) == 0 {
		return errors.New("graph: empty graph")
	}

	// Check for missing references.
	for _, node := range g.Nodes {
		for _, dep := range node.DependsOn {
			if _, exists := g.Nodes[dep]; !exists {
				return fmt.Errorf("graph: node %q depends on unknown node %q", node.ID, dep)
			}
		}
	}

	// Validate fail-branch references.
	for _, node := range g.Nodes {
		if node.FailBranch == nil {
			continue
		}

		target := *node.FailBranch
		if target == node.ID {
			return fmt.Errorf("graph: node %q cannot be its own fail-branch target", node.ID)
		}

		targetNode, exists := g.Nodes[target]
		if !exists {
			return fmt.Errorf("graph: node %q fail-branch references unknown node %q", node.ID, target)
		}

		if !slices.Contains(targetNode.DependsOn, node.ID) {
			return fmt.Errorf("graph: fail-branch target %q must list %q in DependsOn", target, node.ID)
		}
	}

	// Validate prompt/sub-task mutual exclusivity.
	for _, node := range g.Nodes {
		if node.SubTask != nil && node.Prompt != "" {
			return fmt.Errorf("graph: node %q cannot have both prompt and sub-task", node.ID)
		}
	}

	// Validate iteration config.
	for _, node := range g.Nodes {
		if node.MaxIterations > 0 && node.IterationCheck == nil {
			return fmt.Errorf("graph: node %q has MaxIterations=%d but no IterationCheck", node.ID, node.MaxIterations)
		}
		if node.MaxIterations < 0 {
			return fmt.Errorf("graph: node %q has negative MaxIterations", node.ID)
		}
	}

	// Validate error strategy config.
	for _, node := range g.Nodes {
		if node.ErrorStrategy == ErrorRetryThenFail && node.MaxRetries <= 0 {
			return fmt.Errorf("graph: node %q uses ErrorRetryThenFail but MaxRetries=%d", node.ID, node.MaxRetries)
		}
	}

	// Check for cycles via DFS.
	if err := g.detectCycles(); err != nil {
		return err
	}

	g.buildEdges()
	g.computeDepCounts()

	return nil
}

// detectCycles uses iterative DFS with coloring to detect cycles.
// White (0) = unvisited, Gray (1) = in current path, Black (2) = fully explored.
func (g *Graph) detectCycles() error {
	const (
		white = 0
		gray  = 1
		black = 2
	)

	color := make(map[NodeID]int, len(g.Nodes))

	// Collect and sort node IDs for deterministic traversal.
	ids := make([]NodeID, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}

	slices.Sort(ids)

	type frame struct {
		id       NodeID
		depIndex int
	}

	for _, startID := range ids {
		if color[startID] != white {
			continue
		}

		stack := []frame{{id: startID, depIndex: 0}}
		color[startID] = gray

		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			node := g.Nodes[top.id]
			deps := node.DependsOn

			if top.depIndex >= len(deps) {
				color[top.id] = black
				stack = stack[:len(stack)-1]

				continue
			}

			dep := deps[top.depIndex]
			top.depIndex++

			switch color[dep] {
			case white:
				color[dep] = gray
				stack = append(stack, frame{id: dep, depIndex: 0})
			case gray:
				return fmt.Errorf("graph: cycle detected involving node %q", dep)
			}
			// black = already explored, skip
		}
	}

	return nil
}

// NodeCount returns the number of nodes.
func (g *Graph) NodeCount() int {
	return len(g.Nodes)
}

// NodeIDs returns a sorted list of all node IDs.
func (g *Graph) NodeIDs() []NodeID {
	ids := make([]NodeID, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}

	slices.Sort(ids)

	return ids
}
