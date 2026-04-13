package socket

import (
	"encoding/json"
	"testing"
)

// ============================================================
// injectSeqAndBuffer tests
// ============================================================

func TestInjectSeqAndBuffer_ValidObject(t *testing.T) {
	w := newTestWorktreeSocket(t)

	data := []byte(`{"type":"state_changed","state":"loaded"}`)
	result := w.injectSeqAndBuffer(data)

	if len(result) == 0 {
		t.Fatal("injectSeqAndBuffer() returned empty slice")
	}

	// Must parse as valid JSON with seq field
	var parsed map[string]any
	if err := json.Unmarshal(result[:len(result)-1], &parsed); err != nil {
		t.Fatalf("injectSeqAndBuffer() result is not valid JSON: %v (got: %s)", err, result)
	}

	if _, ok := parsed["seq"]; !ok {
		t.Error("result should contain 'seq' field")
	}
	if _, ok := parsed["type"]; !ok {
		t.Error("result should preserve 'type' field from original object")
	}
}

func TestInjectSeqAndBuffer_EmptyObject(t *testing.T) {
	w := newTestWorktreeSocket(t)

	data := []byte(`{}`)
	result := w.injectSeqAndBuffer(data)

	if len(result) == 0 {
		t.Fatal("injectSeqAndBuffer() returned empty slice for empty object")
	}

	var parsed map[string]any
	if err := json.Unmarshal(result[:len(result)-1], &parsed); err != nil {
		t.Fatalf("injectSeqAndBuffer() empty object result not valid JSON: %v", err)
	}
	if _, ok := parsed["seq"]; !ok {
		t.Error("empty object result should still contain 'seq'")
	}
}

func TestInjectSeqAndBuffer_InvalidInput_NotObject(t *testing.T) {
	w := newTestWorktreeSocket(t)

	// Non-object JSON (array) — should return safe fallback
	data := []byte(`[1,2,3]`)
	result := w.injectSeqAndBuffer(data)

	if len(result) == 0 {
		t.Fatal("injectSeqAndBuffer() returned empty slice for invalid input")
	}

	var parsed map[string]any
	if err := json.Unmarshal(result[:len(result)-1], &parsed); err != nil {
		t.Fatalf("injectSeqAndBuffer() invalid input result not valid JSON: %v", err)
	}
	if _, ok := parsed["seq"]; !ok {
		t.Error("fallback result should contain 'seq'")
	}
	if _, ok := parsed["error"]; !ok {
		t.Error("fallback result should contain 'error' field for invalid input")
	}
}

func TestInjectSeqAndBuffer_EmptySlice(t *testing.T) {
	w := newTestWorktreeSocket(t)

	data := []byte{}
	result := w.injectSeqAndBuffer(data)

	if len(result) == 0 {
		t.Fatal("injectSeqAndBuffer() returned empty slice for empty input")
	}

	// Should be a safe fallback
	var parsed map[string]any
	if err := json.Unmarshal(result[:len(result)-1], &parsed); err != nil {
		t.Fatalf("injectSeqAndBuffer() empty input result not valid JSON: %v", err)
	}
}

func TestInjectSeqAndBuffer_SingleByte(t *testing.T) {
	w := newTestWorktreeSocket(t)

	data := []byte(`{`)
	result := w.injectSeqAndBuffer(data)

	if len(result) == 0 {
		t.Fatal("injectSeqAndBuffer() returned empty slice for truncated input")
	}
}

func TestInjectSeqAndBuffer_SequenceMonotonicallyIncreases(t *testing.T) {
	w := newTestWorktreeSocket(t)

	data := []byte(`{"type":"event"}`)

	results := make([]map[string]any, 3)
	for i := range results {
		raw := w.injectSeqAndBuffer(data)
		var parsed map[string]any
		if err := json.Unmarshal(raw[:len(raw)-1], &parsed); err != nil {
			t.Fatalf("call %d: not valid JSON: %v", i, err)
		}
		results[i] = parsed
	}

	seq0, ok := results[0]["seq"].(float64)
	if !ok {
		t.Fatal("results[0][\"seq\"] is not float64")
	}
	seq1, ok := results[1]["seq"].(float64)
	if !ok {
		t.Fatal("results[1][\"seq\"] is not float64")
	}
	seq2, ok := results[2]["seq"].(float64)
	if !ok {
		t.Fatal("results[2][\"seq\"] is not float64")
	}

	if seq1 <= seq0 {
		t.Errorf("seq not monotonically increasing: %v, %v", seq0, seq1)
	}
	if seq2 <= seq1 {
		t.Errorf("seq not monotonically increasing: %v, %v", seq1, seq2)
	}
}

func TestInjectSeqAndBuffer_EndsWithNewline(t *testing.T) {
	w := newTestWorktreeSocket(t)

	data := []byte(`{"x":1}`)
	result := w.injectSeqAndBuffer(data)

	if len(result) == 0 {
		t.Fatal("unexpected empty result")
	}
	if result[len(result)-1] != '\n' {
		t.Errorf("injectSeqAndBuffer() result should end with newline, got %q", result[len(result)-1])
	}
}

func TestInjectSeqAndBuffer_BuffersLastNEvents(t *testing.T) {
	w := newTestWorktreeSocket(t)

	// Fill the replay buffer beyond its capacity
	data := []byte(`{"type":"test"}`)
	for range replayBufSize + 5 {
		w.injectSeqAndBuffer(data)
	}

	// Buffer should be full but not overflow — just verify no panic occurred
	w.replayMu.Lock()
	head := w.replayHead
	w.replayMu.Unlock()

	if head < 0 || head >= replayBufSize {
		t.Errorf("replayHead = %d, want in range [0, %d)", head, replayBufSize)
	}
}

// ============================================================
// resolveProviderToken tests
// ============================================================

func TestResolveProviderToken_UnknownProvider(t *testing.T) {
	token := resolveProviderToken("unknown-provider")
	if token != "" {
		t.Errorf("resolveProviderToken(unknown) = %q, want empty string", token)
	}
}

func TestResolveProviderToken_KnownProviders(t *testing.T) {
	// These should not panic, even if empty (no env vars set in test env)
	for _, name := range []string{"github", "gitlab", "linear", "wrike"} {
		token := resolveProviderToken(name)
		// token may be empty or set from env — just ensure no panic and correct type
		_ = token
	}
}
