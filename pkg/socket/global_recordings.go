package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/valksor/kvelmo/pkg/agent/recorder"
	"github.com/valksor/kvelmo/pkg/filter"
	"github.com/valksor/kvelmo/pkg/page"
	"github.com/valksor/kvelmo/pkg/paths"
)

type recordingsListParams struct {
	Job     string `json:"job,omitempty"`
	Since   string `json:"since,omitempty"`
	Page    int    `json:"page,omitempty"`
	PerPage int    `json:"per_page,omitempty"`
}

type recordingsViewParams struct {
	File string `json:"file"`
}

type recordingViewResult struct {
	Header  *recorder.Header  `json:"header"`
	Records []recorder.Record `json:"records"`
}

func recordingsDir() string {
	return filepath.Join(paths.Paths().BaseDir(), "recordings")
}

func (g *GlobalSocket) handleRecordingsList(_ context.Context, req *Request) (*Response, error) {
	var params recordingsListParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	dir := recordingsDir()
	infos, err := recorder.ListRecordings(dir)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	// Build filter chain
	f := filter.New[recorder.RecordingInfo]()
	if params.Job != "" {
		f.Eq(func(r recorder.RecordingInfo) any { return r.JobID }, params.Job)
	}
	if params.Since != "" {
		since, err := parseDurationString(params.Since)
		if err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid since duration: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
		cutoff := time.Now().Add(-since)
		f.Where(func(r recorder.RecordingInfo) bool {
			t, err := time.Parse(time.RFC3339, r.StartedAt)

			return err == nil && t.After(cutoff)
		})
	}
	infos = f.Apply(infos)

	if infos == nil {
		infos = []recorder.RecordingInfo{}
	}

	// Paginate
	pg := page.NewPage(infos, paginationWithDefault(params.Page, params.PerPage))

	return NewResultResponse(req.ID, map[string]any{
		"recordings": pg.Items,
		"total":      pg.Total,
		"page":       pg.PageNum,
		"per_page":   pg.PerPage,
		"has_next":   pg.HasNext,
	})
}

func (g *GlobalSocket) handleRecordingsView(_ context.Context, req *Request) (*Response, error) {
	var params recordingsViewParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil
	}

	if params.File == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "file is required"), nil
	}

	path := params.File
	if !filepath.IsAbs(path) {
		path = filepath.Join(recordingsDir(), path)
	}

	reader, err := recorder.OpenReader(path)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "open recording: "+err.Error()), nil
	}
	defer func() { _ = reader.Close() }()

	var records []recorder.Record
	for {
		rec, err := reader.Next()
		if err != nil {
			return NewErrorResponse(req.ID, ErrCodeInternal, "read record: "+err.Error()), nil
		}
		if rec == nil {
			break
		}
		records = append(records, *rec)
	}

	if records == nil {
		records = []recorder.Record{}
	}

	result := recordingViewResult{
		Header:  reader.Header(),
		Records: records,
	}

	return NewResultResponse(req.ID, result)
}

// paginationWithDefault returns a Pagination using the given values, defaulting
// to page 1 / per_page 100 when not specified. This preserves backward compatibility
// for handlers that previously returned unbounded arrays.
func paginationWithDefault(pg, perPage int) page.Pagination {
	p := page.Pagination{Page: pg, PerPage: perPage}
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PerPage < 1 {
		p.PerPage = 100
	}

	return p.Normalize()
}

// parseDurationString parses duration strings like "24h", "7d", "30d".
func parseDurationString(s string) (time.Duration, error) {
	if len(s) > 1 && s[len(s)-1] == 'd' {
		var days int
		if _, err := fmt.Sscanf(s, "%dd", &days); err != nil {
			return 0, fmt.Errorf("invalid day duration: %s", s)
		}

		return time.Duration(days) * 24 * time.Hour, nil
	}

	return time.ParseDuration(s)
}
