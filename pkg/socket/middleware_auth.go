package socket

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/valksor/kvelmo/pkg/access"
)

// Auth error code (extends the JSON-RPC error code range).
const ErrCodeUnauthorized = -32003

// tokenRoleKey is a context key for the authenticated token role.
type tokenRoleKey struct{}

// RoleFromContext returns the access.Role stored in ctx by AuthMiddleware,
// or empty string if no auth was performed.
func RoleFromContext(ctx context.Context) access.Role {
	role, _ := ctx.Value(tokenRoleKey{}).(access.Role)

	return role
}

// AuthMiddleware validates an access token from the request's metadata.
// The token must be provided in the request's Params under the "_token" key.
//
// When store is nil, all requests pass through.
func AuthMiddleware(store *access.Store) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *Request) *Response {
			if store == nil {
				return next(ctx, req)
			}

			token := extractToken(req)
			if token == "" {
				slog.Debug("socket auth: missing token", "method", req.Method)

				return NewErrorResponse(req.ID, ErrCodeUnauthorized, "authentication required: provide access token (create one with 'kvelmo access create')")
			}

			tok, err := store.Validate(token)
			if err != nil {
				slog.Debug("socket auth: invalid token", "method", req.Method, "error", err)

				return NewErrorResponse(req.ID, ErrCodeUnauthorized, "invalid or expired access token")
			}

			ctx = context.WithValue(ctx, tokenRoleKey{}, tok.Role)

			// Strip _token from params so downstream handlers cannot leak it.
			req.Params = stripToken(req.Params)

			return next(ctx, req)
		}
	}
}

// stripToken removes the _token field from raw JSON params.
func stripToken(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}

	var params map[string]json.RawMessage
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil // Drop params rather than risk leaking _token
	}

	delete(params, "_token")

	out, err := json.Marshal(params)
	if err != nil {
		return nil // Drop params rather than risk leaking _token
	}

	return out
}

// extractToken pulls the access token from the request.
// It checks for a "_token" string in the raw JSON params.
func extractToken(req *Request) string {
	if req.Params == nil {
		return ""
	}

	var params map[string]any
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return ""
	}

	if token, ok := params["_token"].(string); ok {
		return token
	}

	return ""
}
