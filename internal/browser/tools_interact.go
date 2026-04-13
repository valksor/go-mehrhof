package browser

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// ClickResult represents the result of a click action.
type ClickResult struct {
	Success bool   `json:"success"`
	Element string `json:"element,omitempty"`
}

// Click performs a click on an element.
func Click(ctx context.Context, opts *ExecOptions, selector string) (*ClickResult, error) {
	_, err := Exec(ctx, opts, "click", selector)
	if err != nil {
		return nil, fmt.Errorf("click: %w", err)
	}

	return &ClickResult{
		Success: true,
		Element: selector,
	}, nil
}

// TypeResult represents the result of typing text.
type TypeResult struct {
	Success bool   `json:"success"`
	Element string `json:"element,omitempty"`
	Text    string `json:"text,omitempty"`
}

// Type types text into an element.
func Type(ctx context.Context, opts *ExecOptions, selector, text string) (*TypeResult, error) {
	_, err := Exec(ctx, opts, "type", selector, text)
	if err != nil {
		return nil, fmt.Errorf("type: %w", err)
	}

	return &TypeResult{
		Success: true,
		Element: selector,
		Text:    text,
	}, nil
}

// WaitResult represents the result of a wait operation.
type WaitResult struct {
	Success  bool   `json:"success"`
	Selector string `json:"selector,omitempty"`
	Timeout  int    `json:"timeout_ms,omitempty"`
}

// Wait waits for an element to appear.
func Wait(ctx context.Context, opts *ExecOptions, selector string, timeoutMs int) (*WaitResult, error) {
	args := []string{"wait", selector}
	if timeoutMs > 0 {
		args = append(args, fmt.Sprintf("--timeout=%d", timeoutMs))
	}

	_, err := Exec(ctx, opts, args...)
	if err != nil {
		return nil, fmt.Errorf("wait: %w", err)
	}

	return &WaitResult{
		Success:  true,
		Selector: selector,
		Timeout:  timeoutMs,
	}, nil
}

// FillResult represents the result of filling an input.
type FillResult struct {
	Success  bool   `json:"success"`
	Selector string `json:"selector,omitempty"`
	Value    string `json:"value,omitempty"`
}

// Fill clears an input and sets its value.
func Fill(ctx context.Context, opts *ExecOptions, selector, value string) (*FillResult, error) {
	_, err := Exec(ctx, opts, "fill", selector, value)
	if err != nil {
		return nil, fmt.Errorf("fill: %w", err)
	}

	return &FillResult{
		Success:  true,
		Selector: selector,
		Value:    value,
	}, nil
}

// SelectResult represents the result of selecting an option.
type SelectResult struct {
	Success  bool     `json:"success"`
	Selector string   `json:"selector,omitempty"`
	Values   []string `json:"values,omitempty"`
}

// Select selects an option from a dropdown.
func Select(ctx context.Context, opts *ExecOptions, selector string, values ...string) (*SelectResult, error) {
	args := append([]string{"select", selector}, values...)
	_, err := Exec(ctx, opts, args...)
	if err != nil {
		return nil, fmt.Errorf("select: %w", err)
	}

	return &SelectResult{
		Success:  true,
		Selector: selector,
		Values:   values,
	}, nil
}

// HoverResult represents the result of a hover action.
type HoverResult struct {
	Success  bool   `json:"success"`
	Selector string `json:"selector,omitempty"`
}

// Hover hovers over an element.
func Hover(ctx context.Context, opts *ExecOptions, selector string) (*HoverResult, error) {
	_, err := Exec(ctx, opts, "hover", selector)
	if err != nil {
		return nil, fmt.Errorf("hover: %w", err)
	}

	return &HoverResult{
		Success:  true,
		Selector: selector,
	}, nil
}

// FocusResult represents the result of focusing an element.
type FocusResult struct {
	Success  bool   `json:"success"`
	Selector string `json:"selector,omitempty"`
}

// Focus focuses an element.
func Focus(ctx context.Context, opts *ExecOptions, selector string) (*FocusResult, error) {
	_, err := Exec(ctx, opts, "focus", selector)
	if err != nil {
		return nil, fmt.Errorf("focus: %w", err)
	}

	return &FocusResult{
		Success:  true,
		Selector: selector,
	}, nil
}

// ScrollResult represents the result of a scroll action.
type ScrollResult struct {
	Success   bool   `json:"success"`
	Direction string `json:"direction,omitempty"`
	Amount    int    `json:"amount,omitempty"`
}

// Scroll scrolls the page or an element.
func Scroll(ctx context.Context, opts *ExecOptions, direction string, amount int, selector string) (*ScrollResult, error) {
	args := []string{"scroll", direction}

	if amount > 0 {
		args = append(args, strconv.Itoa(amount))
	}

	if selector != "" {
		args = append(args, "--element="+selector)
	}

	_, err := Exec(ctx, opts, args...)
	if err != nil {
		return nil, fmt.Errorf("scroll: %w", err)
	}

	return &ScrollResult{
		Success:   true,
		Direction: direction,
		Amount:    amount,
	}, nil
}

// PressResult represents the result of pressing a key.
type PressResult struct {
	Success  bool   `json:"success"`
	Key      string `json:"key,omitempty"`
	Selector string `json:"selector,omitempty"`
}

// Press presses a key or key combination.
func Press(ctx context.Context, opts *ExecOptions, key string, selector string) (*PressResult, error) {
	args := []string{"press", key}

	if selector != "" {
		args = append(args, "--element="+selector)
	}

	_, err := Exec(ctx, opts, args...)
	if err != nil {
		return nil, fmt.Errorf("press: %w", err)
	}

	return &PressResult{
		Success:  true,
		Key:      key,
		Selector: selector,
	}, nil
}

// DialogResult represents the result of handling a dialog.
type DialogResult struct {
	Success bool   `json:"success"`
	Action  string `json:"action,omitempty"`
	Message string `json:"message,omitempty"`
}

// Dialog handles alert/confirm/prompt dialogs.
func Dialog(ctx context.Context, opts *ExecOptions, action string, text string) (*DialogResult, error) {
	args := []string{"dialog", action}

	if text != "" && action == "accept" {
		args = append(args, "--text="+text)
	}

	output, err := Exec(ctx, opts, args...)
	if err != nil {
		return nil, fmt.Errorf("dialog: %w", err)
	}

	return &DialogResult{
		Success: true,
		Action:  action,
		Message: strings.TrimSpace(string(output)),
	}, nil
}

// UploadResult represents the result of a file upload.
type UploadResult struct {
	Success  bool     `json:"success"`
	Selector string   `json:"selector,omitempty"`
	Files    []string `json:"files,omitempty"`
}

// Upload uploads files to a file input.
func Upload(ctx context.Context, opts *ExecOptions, selector string, files []string) (*UploadResult, error) {
	args := make([]string, 0, 2+len(files))
	args = append(args, "upload", selector)
	args = append(args, files...)

	_, err := Exec(ctx, opts, args...)
	if err != nil {
		return nil, fmt.Errorf("upload: %w", err)
	}

	return &UploadResult{
		Success:  true,
		Selector: selector,
		Files:    files,
	}, nil
}
