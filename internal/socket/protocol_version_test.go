package socket

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"
)

func TestProtocolCompatible(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"", true},              // legacy client without negotiation
		{ProtocolVersion, true}, // current
		{"999", false},          // future major
		{"0", false},            // older major
	}
	for _, tt := range tests {
		if got := protocolCompatible(tt.version); got != tt.want {
			t.Errorf("protocolCompatible(%q) = %v, want %v", tt.version, got, tt.want)
		}
	}
}

func TestDispatchProtocolEnforcement(t *testing.T) {
	s := NewServer(filepath.Join(t.TempDir(), "test.sock"))
	s.Handle("echo", func(_ context.Context, req *Request) (*Response, error) {
		return NewResultResponse(req.ID, map[string]string{"ok": "yes"})
	})

	cases := []struct {
		name        string
		req         *Request
		wantErr     bool
		wantErrCode int
	}{
		{"current version", &Request{ID: "1", Method: "echo", ProtocolVersion: ProtocolVersion}, false, 0},
		{"absent version allowed", &Request{ID: "2", Method: "echo"}, false, 0},
		{"incompatible version rejected", &Request{ID: "3", Method: "echo", ProtocolVersion: "999"}, true, ErrCodeInvalidRequest},
		{"handshake reachable cross-version", &Request{ID: "4", Method: MethodCapabilities, ProtocolVersion: "999"}, false, 0},
		{"ping reachable cross-version", &Request{ID: "5", Method: "ping", ProtocolVersion: "999"}, false, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := s.dispatch(context.Background(), tc.req, nil)
			if tc.wantErr {
				if resp.Error == nil || resp.Error.Code != tc.wantErrCode {
					t.Fatalf("got %+v, want error code %d", resp.Error, tc.wantErrCode)
				}

				return
			}
			// ping has no registered handler on a bare Server; it is exempt from
			// the version check but will surface MethodNotFound, which is fine —
			// the point is that it is NOT rejected as a version mismatch.
			if resp.Error != nil && resp.Error.Code == ErrCodeInvalidRequest {
				t.Fatalf("unexpected protocol-mismatch rejection: %+v", resp.Error)
			}
		})
	}
}

func TestCapabilitiesHandshake(t *testing.T) {
	s := NewServer(filepath.Join(t.TempDir(), "test.sock"))
	s.Handle("echo", func(_ context.Context, req *Request) (*Response, error) {
		return NewResultResponse(req.ID, nil)
	})

	resp := s.dispatch(context.Background(), &Request{ID: "1", Method: MethodCapabilities, ProtocolVersion: ProtocolVersion}, nil)
	if resp.Error != nil {
		t.Fatalf("capabilities errored: %v", resp.Error)
	}

	var caps Capabilities
	if err := json.Unmarshal(resp.Result, &caps); err != nil {
		t.Fatalf("unmarshal capabilities: %v", err)
	}
	if caps.ProtocolVersion != ProtocolVersion {
		t.Errorf("ProtocolVersion = %q, want %q", caps.ProtocolVersion, ProtocolVersion)
	}
	if !slices.Contains(caps.Methods, "echo") {
		t.Errorf("methods should include registered handler echo: %v", caps.Methods)
	}
	if !slices.Contains(caps.Methods, MethodCapabilities) {
		t.Errorf("methods should include the handshake itself: %v", caps.Methods)
	}
	if !slices.Contains(caps.Methods, "shutdown") {
		t.Errorf("methods should include built-in shutdown: %v", caps.Methods)
	}
}
