package commands

import (
	"os"
	"testing"

	"github.com/valksor/kvelmo/internal/socket"
)

// The stub socket has no stream.subscribe handler, so waitForJob fails after
// subscribing. These tests exercise the `--wait` dispatch branch of each phase
// command plus the connect/subscribe/parse path of waitForJob.

func TestRunPlan_WaitDispatch(t *testing.T) {
	setBoolPtr(t, &planWait, true)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("plan", map[string]any{"job_id": "job-w1"})

	// waitForJob fails (no stream.subscribe), surfacing an error — that's the
	// branch under test.
	if err := runPlan(PlanCmd, nil); err == nil {
		t.Error("expected waitForJob error via --wait (stub has no stream)")
	}
}

func TestRunImplement_WaitDispatch(t *testing.T) {
	setBoolPtr(t, &implementWait, true)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("implement", map[string]any{"job_id": "job-w2"})

	if err := runImplement(ImplementCmd, nil); err == nil {
		t.Error("expected waitForJob error via --wait")
	}
}

func TestRunReview_WaitDispatch(t *testing.T) {
	setBoolPtr(t, &reviewWait, true)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("review", map[string]any{"job_id": "job-w3", "status": "reviewing"})

	if err := runReview(ReviewCmd, nil); err == nil {
		t.Error("expected waitForJob error via --wait")
	}
}

func TestRunExplain_WaitDispatch(t *testing.T) {
	setBoolPtr(t, &explainWait, true)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("chat.send", map[string]any{"job_id": "job-w4", "status": "queued"})

	if err := runExplain(ExplainCmd, nil); err == nil {
		t.Error("expected waitForJob error via --wait")
	}
}

// TestWaitForJob_SubscribeError directly exercises waitForJob against a stub
// that lacks stream.subscribe, covering the subscribe-error branch.
func TestWaitForJob_SubscribeError(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	_ = stub

	cwd, _ := os.Getwd()
	if err := waitForJob(socket.WorktreeSocketPath(cwd), "job-x"); err == nil {
		t.Error("expected subscribe error from stub (no stream.subscribe handler)")
	}
}
