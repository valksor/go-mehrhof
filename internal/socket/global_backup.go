package socket

import (
	"context"
	"encoding/json"

	"github.com/valksor/kvelmo/internal/backup"
	"github.com/valksor/kvelmo/paths"
)

type backupCreateParams struct {
	OutputPath string `json:"output_path,omitempty"`
}

func (g *GlobalSocket) handleBackupCreate(_ context.Context, req *Request) (*Response, error) {
	var params backupCreateParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	baseDir := paths.Paths().BaseDir()
	result, err := backup.Create(baseDir, params.OutputPath)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

type backupRestoreParams struct {
	ArchivePath string `json:"archive_path"`
}

func (g *GlobalSocket) handleBackupRestore(_ context.Context, req *Request) (*Response, error) {
	var params backupRestoreParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if params.ArchivePath == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "archive_path is required"), nil
	}

	targetDir := paths.Paths().BaseDir()
	result, err := backup.Restore(params.ArchivePath, targetDir)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

func (g *GlobalSocket) handleBackupList(_ context.Context, req *Request) (*Response, error) {
	baseDir := paths.Paths().BaseDir()
	backups, err := backup.List(baseDir)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	if backups == nil {
		backups = []backup.BackupInfo{}
	}

	return NewResultResponse(req.ID, map[string]any{
		"backups": backups,
	})
}
