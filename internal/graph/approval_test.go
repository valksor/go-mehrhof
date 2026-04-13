package graph

import (
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/worker"
)

func TestApprovalGate_FieldsPreserved(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{
		ID:               "a",
		JobType:          worker.JobTypePlan,
		Prompt:           "plan it",
		RequiresApproval: true,
		ApprovalPrompt:   "Ready to plan?",
	})
	_ = g.Validate()

	// Verify approval fields are set
	node := g.Nodes["a"]
	if !node.RequiresApproval {
		t.Fatal("expected RequiresApproval=true")
	}
	if node.ApprovalPrompt != "Ready to plan?" {
		t.Fatalf("expected approval prompt, got %q", node.ApprovalPrompt)
	}
}

func TestApprovalGate_YAMLParse(t *testing.T) {
	yaml := `
phase: implement
nodes:
  - id: review_migration
    job_type: implement
    prompt: "Apply database migration"
    requires_approval: true
    approval_prompt: "Review the migration SQL before applying"
    approval_timeout: 5m
`
	g, err := ParseGraphDef([]byte(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	node := g.Nodes["review_migration"]
	if !node.RequiresApproval {
		t.Fatal("expected RequiresApproval=true")
	}
	if node.ApprovalPrompt != "Review the migration SQL before applying" {
		t.Fatalf("unexpected prompt: %q", node.ApprovalPrompt)
	}
	if node.ApprovalTimeout != 5*time.Minute {
		t.Fatalf("expected 5m timeout, got %v", node.ApprovalTimeout)
	}
}

func TestScheduler_ApproveRejectUnknownNode(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{ID: "a", JobType: worker.JobTypePlan, Prompt: "x"})
	_ = g.Validate()

	s := NewScheduler(g, nil)

	// Approving/rejecting a node with no pending approval should return false
	if s.ApproveNode("a") {
		t.Fatal("should return false for non-pending approval")
	}
	if s.RejectNode("a") {
		t.Fatal("should return false for non-pending approval")
	}
	if s.ApproveNode("nonexistent") {
		t.Fatal("should return false for unknown node")
	}
}
