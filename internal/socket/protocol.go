package socket

import (
	"encoding/json"
	"fmt"
)

const (
	JSONRPCVersion  = "2.0"
	ProtocolVersion = "1"
)

type Request struct {
	JSONRPC         string          `json:"jsonrpc"`
	ProtocolVersion string          `json:"protocol_version,omitempty"`
	ID              string          `json:"id"`
	Method          string          `json:"method"`
	Params          json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

const (
	ErrCodeParse          = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternal       = -32603
	ErrCodeNotFound       = -32604
	ErrCodeTimeout        = -32000
	ErrCodeShuttingDown   = -32001
	ErrCodeRateLimited    = -32002
)

func EncodeRequest(req *Request) ([]byte, error) {
	req.JSONRPC = JSONRPCVersion
	req.ProtocolVersion = ProtocolVersion
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	return append(data, '\n'), nil
}

func DecodeRequest(data []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("unmarshal request: %w", err)
	}
	if req.JSONRPC != JSONRPCVersion {
		return nil, fmt.Errorf("invalid jsonrpc version: %s", req.JSONRPC)
	}

	return &req, nil
}

func EncodeResponse(resp *Response) ([]byte, error) {
	resp.JSONRPC = JSONRPCVersion
	data, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}

	return append(data, '\n'), nil
}

func DecodeResponse(data []byte) (*Response, error) {
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

func NewErrorResponse(id string, code int, message string) *Response {
	return &Response{
		ID: id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
}

func NewResultResponse(id string, result any) (*Response, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}

	return &Response{
		ID:     id,
		Result: data,
	}, nil
}

// encodeEvent encodes a streaming event as newline-delimited JSON.
// Events are distinguished from RPC responses by not having an "id" field.
func encodeEvent(event any) ([]byte, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}

	return append(data, '\n'), nil
}
