package socket

import (
	"context"
	"encoding/json"
	"testing"
)

func TestHandleNotifyTest_NoNotifier(t *testing.T) {
	old := notifier
	notifier = nil
	t.Cleanup(func() { notifier = old })

	ctx := context.Background()
	g := newTestGlobalSocket(t)

	req := &Request{ID: "1", Method: "notify.test"}
	resp, err := g.handleNotifyTest(ctx, req)
	if err != nil {
		t.Fatalf("handleNotifyTest() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleNotifyTest() returned error: %s", resp.Error.Message)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	sent, ok := result["sent"].(float64)
	if !ok {
		t.Fatal("sent should be a number")
	}
	if sent != 0 {
		t.Errorf("sent = %v, want 0", sent)
	}

	msg, ok := result["message"].(string)
	if !ok {
		t.Fatal("message should be a string")
	}
	if msg == "" {
		t.Error("message should not be empty")
	}
}
