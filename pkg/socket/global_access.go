package socket

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/valksor/kvelmo/pkg/access"
	"github.com/valksor/kvelmo/pkg/page"
)

// AccessTokenCreateParams is the request for access.token.create.
type AccessTokenCreateParams struct {
	Role  string `json:"role"`
	Label string `json:"label"`
}

// AccessTokenRevokeParams is the request for access.token.revoke.
type AccessTokenRevokeParams struct {
	ID string `json:"id"`
}

func (g *GlobalSocket) handleAccessTokenList(_ context.Context, req *Request) (*Response, error) {
	var params struct {
		Page    int `json:"page,omitempty"`
		PerPage int `json:"per_page,omitempty"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	store := access.New("")

	tokens, err := store.List()
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("list tokens: %v", err)), nil
	}

	pg := page.NewPage(tokens, paginationWithDefault(params.Page, params.PerPage))

	return NewResultResponse(req.ID, map[string]any{
		"tokens":   pg.Items,
		"total":    pg.Total,
		"page":     pg.PageNum,
		"per_page": pg.PerPage,
		"has_next": pg.HasNext,
	})
}

func (g *GlobalSocket) handleAccessTokenCreate(_ context.Context, req *Request) (*Response, error) {
	var params AccessTokenCreateParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	role := access.Role(params.Role)
	if role != access.RoleOperator && role != access.RoleViewer {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "role must be 'operator' or 'viewer'"), nil
	}

	store := access.New("")

	plaintext, err := store.Create(role, params.Label, nil)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("create token: %v", err)), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"token": plaintext,
	})
}

func (g *GlobalSocket) handleAccessTokenRevoke(_ context.Context, req *Request) (*Response, error) {
	var params AccessTokenRevokeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.ID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "id is required"), nil
	}

	store := access.New("")

	if err := store.Revoke(params.ID); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("revoke token: %v", err)), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"revoked": true,
	})
}
