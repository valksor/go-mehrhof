package browser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SnapshotResult represents the result of a browser snapshot.
type SnapshotResult struct {
	URL      string `json:"url,omitempty"`
	Title    string `json:"title,omitempty"`
	Snapshot string `json:"snapshot"` // Accessibility tree as text
}

// Snapshot captures the accessibility snapshot of the current page.
func Snapshot(ctx context.Context, opts *ExecOptions) (*SnapshotResult, error) {
	output, err := Exec(ctx, opts, "snapshot")
	if err != nil {
		return nil, fmt.Errorf("snapshot: %w", err)
	}

	// playwright-cli snapshot returns plain text accessibility tree
	return &SnapshotResult{
		Snapshot: strings.TrimSpace(string(output)),
	}, nil
}

// EvalResult represents the result of JavaScript evaluation.
type EvalResult struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

// Eval executes JavaScript in the browser and returns the result.
func Eval(ctx context.Context, opts *ExecOptions, js string) (*EvalResult, error) {
	output, err := Exec(ctx, opts, "eval", js)
	if err != nil {
		// Check if error message contains eval result
		errStr := err.Error()
		if strings.Contains(errStr, "playwright-cli:") {
			return &EvalResult{
				Error: errStr,
			}, nil
		}

		return nil, fmt.Errorf("eval: %w", err)
	}

	return &EvalResult{
		Result: strings.TrimSpace(string(output)),
	}, nil
}

// ConsoleMessage represents a browser console message.
type ConsoleMessage struct {
	Type      string `json:"type"` // "log", "warn", "error", "info", "debug"
	Text      string `json:"text"`
	Timestamp string `json:"timestamp,omitempty"`
	Location  string `json:"location,omitempty"` // source:line:col
}

// ConsoleResult represents console messages from the browser.
type ConsoleResult struct {
	Messages []ConsoleMessage `json:"messages"`
}

// Console retrieves console messages from the browser.
func Console(ctx context.Context, opts *ExecOptions) (*ConsoleResult, error) {
	output, err := Exec(ctx, opts, "console")
	if err != nil {
		return nil, fmt.Errorf("console: %w", err)
	}

	// Try to parse as JSON first
	var result ConsoleResult
	if err := json.Unmarshal(output, &result); err != nil {
		// Fallback: treat output as plain text messages
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		result.Messages = make([]ConsoleMessage, 0, len(lines))
		for _, line := range lines {
			if line == "" {
				continue
			}
			result.Messages = append(result.Messages, ConsoleMessage{
				Type: "log",
				Text: line,
			})
		}
	}

	return &result, nil
}

// NetworkRequest represents a network request captured by the browser.
type NetworkRequest struct {
	URL        string            `json:"url"`
	Method     string            `json:"method"`
	Status     int               `json:"status,omitempty"`
	StatusText string            `json:"status_text,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Timing     *RequestTiming    `json:"timing,omitempty"`
}

// RequestTiming contains timing information for a request.
type RequestTiming struct {
	StartTime float64 `json:"start_time,omitempty"`
	EndTime   float64 `json:"end_time,omitempty"`
	Duration  float64 `json:"duration,omitempty"`
}

// NetworkResult represents network requests from the browser.
type NetworkResult struct {
	Requests []NetworkRequest `json:"requests"`
}

// Network retrieves network requests from the browser.
func Network(ctx context.Context, opts *ExecOptions) (*NetworkResult, error) {
	output, err := Exec(ctx, opts, "network")
	if err != nil {
		return nil, fmt.Errorf("network: %w", err)
	}

	// Try to parse as JSON first
	var result NetworkResult
	if err := json.Unmarshal(output, &result); err != nil {
		// Fallback: parse plain text format
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		result.Requests = make([]NetworkRequest, 0, len(lines))
		for _, line := range lines {
			if line == "" {
				continue
			}
			// Try to parse common formats like "GET https://example.com 200"
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				req := NetworkRequest{
					Method: parts[0],
					URL:    parts[1],
				}
				result.Requests = append(result.Requests, req)
			}
		}
	}

	return &result, nil
}

// ScreenshotOptions configures screenshot capture.
type ScreenshotOptions struct {
	Path     string // Output file path
	FullPage bool   // Capture full scrollable page
	Element  string // CSS selector for element to capture
	Format   string // "png" or "jpeg"
	Quality  int    // JPEG quality (1-100)
}

// ScreenshotResult represents the result of a screenshot capture.
type ScreenshotResult struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// Screenshot captures a screenshot of the current page.
func Screenshot(ctx context.Context, opts *ExecOptions, screenshotOpts *ScreenshotOptions) (*ScreenshotResult, error) {
	if screenshotOpts == nil {
		screenshotOpts = &ScreenshotOptions{}
	}

	// Build args
	args := []string{"screenshot"}

	if screenshotOpts.Path != "" {
		args = append(args, "--output="+screenshotOpts.Path)
	}

	if screenshotOpts.FullPage {
		args = append(args, "--full-page")
	}

	if screenshotOpts.Element != "" {
		args = append(args, "--element="+screenshotOpts.Element)
	}

	if screenshotOpts.Format != "" {
		args = append(args, "--format="+screenshotOpts.Format)
	}

	if screenshotOpts.Quality > 0 {
		args = append(args, fmt.Sprintf("--quality=%d", screenshotOpts.Quality))
	}

	output, err := Exec(ctx, opts, args...)
	if err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}

	// Determine output path
	path := screenshotOpts.Path
	if path == "" {
		// playwright-cli may output the path
		path = strings.TrimSpace(string(output))
	}

	// Get file size
	var size int64
	if info, err := os.Stat(path); err == nil {
		size = info.Size()
	}

	return &ScreenshotResult{
		Path: path,
		Size: size,
	}, nil
}

// NavigateResult represents the result of navigation.
type NavigateResult struct {
	URL    string `json:"url"`
	Title  string `json:"title,omitempty"`
	Status int    `json:"status,omitempty"`
}

// Navigate navigates to a URL.
func Navigate(ctx context.Context, opts *ExecOptions, url string) (*NavigateResult, error) {
	output, err := Exec(ctx, opts, "navigate", url)
	if err != nil {
		return nil, fmt.Errorf("navigate: %w", err)
	}

	return &NavigateResult{
		URL: url,
		// Title may be in output
		Title: strings.TrimSpace(string(output)),
	}, nil
}

// Back navigates back in browser history.
func Back(ctx context.Context, opts *ExecOptions) (*NavigateResult, error) {
	output, err := Exec(ctx, opts, "back")
	if err != nil {
		return nil, fmt.Errorf("back: %w", err)
	}

	return &NavigateResult{
		Title: strings.TrimSpace(string(output)),
	}, nil
}

// Forward navigates forward in browser history.
func Forward(ctx context.Context, opts *ExecOptions) (*NavigateResult, error) {
	output, err := Exec(ctx, opts, "forward")
	if err != nil {
		return nil, fmt.Errorf("forward: %w", err)
	}

	return &NavigateResult{
		Title: strings.TrimSpace(string(output)),
	}, nil
}

// Reload reloads the current page.
func Reload(ctx context.Context, opts *ExecOptions) (*NavigateResult, error) {
	output, err := Exec(ctx, opts, "reload")
	if err != nil {
		return nil, fmt.Errorf("reload: %w", err)
	}

	return &NavigateResult{
		Title: strings.TrimSpace(string(output)),
	}, nil
}

// PDFOptions configures PDF generation.
type PDFOptions struct {
	Path      string // Output file path
	Format    string // Paper format: A4, Letter, etc.
	Landscape bool   // Landscape orientation
}

// PDFResult represents the result of PDF generation.
type PDFResult struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Base64 string `json:"base64,omitempty"`
}

// GeneratePDF generates a PDF of the current page.
func GeneratePDF(ctx context.Context, opts *ExecOptions, pdfOpts *PDFOptions) (*PDFResult, error) {
	if pdfOpts == nil {
		pdfOpts = &PDFOptions{}
	}

	args := []string{"pdf"}

	outputPath := pdfOpts.Path
	if outputPath == "" {
		outputPath = filepath.Join(os.TempDir(), fmt.Sprintf("page-%d.pdf", time.Now().UnixNano()))
	}
	args = append(args, "--output="+outputPath)

	if pdfOpts.Format != "" {
		args = append(args, "--format="+pdfOpts.Format)
	}

	if pdfOpts.Landscape {
		args = append(args, "--landscape")
	}

	_, err := Exec(ctx, opts, args...)
	if err != nil {
		return nil, fmt.Errorf("pdf: %w", err)
	}

	result := &PDFResult{
		Path: outputPath,
	}

	// Get file size
	if info, err := os.Stat(outputPath); err == nil {
		result.Size = info.Size()
	}

	// If no output path was specified, include base64 in result
	if pdfOpts.Path == "" {
		if data, err := os.ReadFile(outputPath); err == nil {
			result.Base64 = base64.StdEncoding.EncodeToString(data)
		}
	}

	return result, nil
}
