package agenttest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/agenttest"
)

func TestMockAgent_ImplementsInterface(t *testing.T) {
	var _ agent.Agent = (*agenttest.MockAgent)(nil)
}

func TestMockAgent_Name(t *testing.T) {
	m := agenttest.NewMockAgent("test-agent")
	if m.Name() != "test-agent" {
		t.Errorf("Name() = %q, want test-agent", m.Name())
	}
}

func TestMockAgent_Available(t *testing.T) {
	m := agenttest.NewMockAgent("test")
	if err := m.Available(); err != nil {
		t.Errorf("Available() returned unexpected error: %v", err)
	}
}

func TestMockAgent_AvailableError(t *testing.T) {
	m := agenttest.NewMockAgent("test").WithAvailableError(errors.New("not available"))
	if err := m.Available(); err == nil {
		t.Error("Available() should return error after WithAvailableError")
	}
}

func TestMockAgent_ConnectDisconnect(t *testing.T) {
	m := agenttest.NewMockAgent("test")
	if m.Connected() {
		t.Error("should not be connected initially")
	}

	if err := m.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	if !m.Connected() {
		t.Error("should be connected after Connect()")
	}
	if m.ConnectCalls != 1 {
		t.Errorf("ConnectCalls = %d, want 1", m.ConnectCalls)
	}

	if err := m.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	if m.Connected() {
		t.Error("should not be connected after Close()")
	}
	if m.CloseCalls != 1 {
		t.Errorf("CloseCalls = %d, want 1", m.CloseCalls)
	}
}

func TestMockAgent_SendPrompt_DefaultComplete(t *testing.T) {
	m := agenttest.NewMockAgent("test")
	ch, err := m.SendPrompt(context.Background(), "hello")
	if err != nil {
		t.Fatalf("SendPrompt() error: %v", err)
	}

	var events []agent.Event
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Type != agent.EventComplete {
		t.Errorf("event type = %q, want EventComplete", events[0].Type)
	}
	if len(m.Prompts) != 1 || m.Prompts[0] != "hello" {
		t.Errorf("Prompts = %v, want [hello]", m.Prompts)
	}
}

func TestMockAgent_SendPrompt_CustomEvents(t *testing.T) {
	m := agenttest.NewMockAgent(
		"test",
		agent.Event{Type: agent.EventStream, Content: "token1"},
		agent.Event{Type: agent.EventStream, Content: "token2"},
		agent.Event{Type: agent.EventComplete},
	)

	ch, err := m.SendPrompt(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("SendPrompt() error: %v", err)
	}

	var events []agent.Event
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
	if events[0].Content != "token1" {
		t.Errorf("events[0].Content = %q, want token1", events[0].Content)
	}
	if events[1].Content != "token2" {
		t.Errorf("events[1].Content = %q, want token2", events[1].Content)
	}
	if events[2].Type != agent.EventComplete {
		t.Errorf("events[2].Type = %q, want EventComplete", events[2].Type)
	}
}

func TestMockAgent_SendPrompt_ErrorTerminates(t *testing.T) {
	m := agenttest.NewMockAgent(
		"test",
		agent.Event{Type: agent.EventStream, Content: "partial"},
		agent.Event{Type: agent.EventError, Error: "boom"},
		agent.Event{Type: agent.EventStream, Content: "should not appear"},
	)

	ch, err := m.SendPrompt(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("SendPrompt() error: %v", err)
	}

	var events []agent.Event
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 (stream + error)", len(events))
	}
	if events[1].Type != agent.EventError {
		t.Errorf("events[1].Type = %q, want EventError", events[1].Type)
	}
}

func TestMockAgent_HandlePermission(t *testing.T) {
	m := agenttest.NewMockAgent("test")
	if err := m.HandlePermission("req-1", true); err != nil {
		t.Fatalf("HandlePermission() error: %v", err)
	}
	if err := m.HandlePermission("req-2", false); err != nil {
		t.Fatalf("HandlePermission() error: %v", err)
	}

	if len(m.Permissions) != 2 {
		t.Fatalf("Permissions = %d, want 2", len(m.Permissions))
	}
	if m.Permissions[0].RequestID != "req-1" || !m.Permissions[0].Approved {
		t.Errorf("Permissions[0] = %+v, want {req-1, true}", m.Permissions[0])
	}
	if m.Permissions[1].RequestID != "req-2" || m.Permissions[1].Approved {
		t.Errorf("Permissions[1] = %+v, want {req-2, false}", m.Permissions[1])
	}
}

func TestMockAgent_WithMethods_ReturnNewAgent(t *testing.T) {
	m := agenttest.NewMockAgent("test")
	a := m.WithEnv("K", "V")
	if a == m {
		t.Error("WithEnv should return a new agent")
	}
	b := m.WithArgs("--flag")
	if b == m {
		t.Error("WithArgs should return a new agent")
	}
	c := m.WithWorkDir("/tmp")
	if c == m {
		t.Error("WithWorkDir should return a new agent")
	}
	d := m.WithTimeout(5 * time.Minute)
	if d == m {
		t.Error("WithTimeout should return a new agent")
	}
}

func TestMockAgent_Timestamps(t *testing.T) {
	m := agenttest.NewMockAgent(
		"test",
		agent.Event{Type: agent.EventStream, Content: "hi"},
	)

	before := time.Now()
	ch, _ := m.SendPrompt(context.Background(), "prompt")
	var events []agent.Event
	for ev := range ch {
		events = append(events, ev)
	}
	after := time.Now()

	for i, ev := range events {
		if ev.Timestamp.IsZero() {
			t.Errorf("events[%d].Timestamp is zero", i)
		}
		if ev.Timestamp.Before(before) || ev.Timestamp.After(after) {
			t.Errorf("events[%d].Timestamp %v outside [%v, %v]", i, ev.Timestamp, before, after)
		}
	}
}

func TestMockAgent_ConcurrentSafe(t *testing.T) {
	m := agenttest.NewMockAgent(
		"test",
		agent.Event{Type: agent.EventStream, Content: "data"},
	)

	done := make(chan struct{})
	for range 10 {
		go func() {
			defer func() { done <- struct{}{} }()
			ch, _ := m.SendPrompt(context.Background(), "concurrent")
			for range ch {
			}
		}()
	}
	for range 10 {
		<-done
	}

	if len(m.Prompts) != 10 {
		t.Errorf("Prompts = %d, want 10", len(m.Prompts))
	}
}
