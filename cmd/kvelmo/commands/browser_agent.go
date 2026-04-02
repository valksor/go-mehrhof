package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/socket"
)

// Agent-friendly browser command implementations

func runBrowserNavigate(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	params := map[string]string{"url": args[0]}
	resp, err := client.Call(ctx, "browser.navigate", params)
	if err != nil {
		return fmt.Errorf("browser.navigate: %w", err)
	}

	// Output JSON for agent parsing
	fmt.Println(string(resp.Result))

	return nil
}

func runBrowserSnapshot(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	resp, err := client.Call(ctx, "browser.snapshot", nil)
	if err != nil {
		return fmt.Errorf("browser.snapshot: %w", err)
	}

	var result struct {
		Snapshot string `json:"snapshot"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	// Output plain text accessibility tree for agent parsing
	fmt.Println(result.Snapshot)

	return nil
}

func runBrowserScreenshot(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	params := map[string]any{}
	if screenshotOutput != "" {
		params["path"] = screenshotOutput
	}
	if screenshotFullPage {
		params["full_page"] = true
	}

	// If running inside a project, pass worktree_id so the socket stores the screenshot directly.
	cwd, _ := os.Getwd()
	wtPath := socket.WorktreeSocketPath(cwd)
	if socket.SocketExists(wtPath) {
		params["worktree_id"] = socket.WorktreeIDFromPath(cwd)
	}

	resp, err := client.Call(ctx, "browser.screenshot", params)
	if err != nil {
		return fmt.Errorf("browser.screenshot: %w", err)
	}

	fmt.Println(string(resp.Result))

	return nil
}

func runBrowserClick(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	params := map[string]string{"selector": args[0]}
	resp, err := client.Call(ctx, "browser.click", params)
	if err != nil {
		return fmt.Errorf("browser.click: %w", err)
	}

	fmt.Println(string(resp.Result))

	return nil
}

func runBrowserType(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	params := map[string]string{
		"selector": args[0],
		"text":     args[1],
	}
	resp, err := client.Call(ctx, "browser.type", params)
	if err != nil {
		return fmt.Errorf("browser.type: %w", err)
	}

	fmt.Println(string(resp.Result))

	return nil
}

func runBrowserWait(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	params := map[string]any{
		"selector":   args[0],
		"timeout_ms": browserWaitTimeout,
	}
	resp, err := client.Call(ctx, "browser.wait", params)
	if err != nil {
		return fmt.Errorf("browser.wait: %w", err)
	}

	fmt.Println(string(resp.Result))

	return nil
}

func runBrowserEval(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	params := map[string]string{"js": args[0]}
	resp, err := client.Call(ctx, "browser.eval", params)
	if err != nil {
		return fmt.Errorf("browser.eval: %w", err)
	}

	var result struct {
		Result string `json:"result"`
		Error  string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if result.Error != "" {
		return fmt.Errorf("eval error: %s", result.Error)
	}
	fmt.Println(result.Result)

	return nil
}

func runBrowserConsole(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	resp, err := client.Call(ctx, "browser.console", nil)
	if err != nil {
		return fmt.Errorf("browser.console: %w", err)
	}

	var result struct {
		Messages []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	// Output as readable text
	for _, msg := range result.Messages {
		fmt.Printf("[%s] %s\n", msg.Type, msg.Text)
	}

	return nil
}

func runBrowserNetwork(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	resp, err := client.Call(ctx, "browser.network", nil)
	if err != nil {
		return fmt.Errorf("browser.network: %w", err)
	}

	// Output JSON for agent parsing
	fmt.Println(string(resp.Result))

	return nil
}

func runBrowserFill(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	params := map[string]string{
		"selector": args[0],
		"value":    args[1],
	}
	resp, err := client.Call(ctx, "browser.fill", params)
	if err != nil {
		return fmt.Errorf("browser.fill: %w", err)
	}

	fmt.Println(string(resp.Result))

	return nil
}

func runBrowserSelect(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	params := map[string]any{
		"selector": args[0],
		"values":   args[1:],
	}
	resp, err := client.Call(ctx, "browser.select", params)
	if err != nil {
		return fmt.Errorf("browser.select: %w", err)
	}

	fmt.Println(string(resp.Result))

	return nil
}

func runBrowserHover(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	params := map[string]string{"selector": args[0]}
	resp, err := client.Call(ctx, "browser.hover", params)
	if err != nil {
		return fmt.Errorf("browser.hover: %w", err)
	}

	fmt.Println(string(resp.Result))

	return nil
}

func runBrowserFocus(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	params := map[string]string{"selector": args[0]}
	resp, err := client.Call(ctx, "browser.focus", params)
	if err != nil {
		return fmt.Errorf("browser.focus: %w", err)
	}

	fmt.Println(string(resp.Result))

	return nil
}

func runBrowserScroll(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	params := map[string]any{
		"direction": args[0],
	}
	if scrollAmount > 0 {
		params["amount"] = scrollAmount
	}
	if scrollSelector != "" {
		params["selector"] = scrollSelector
	}

	resp, err := client.Call(ctx, "browser.scroll", params)
	if err != nil {
		return fmt.Errorf("browser.scroll: %w", err)
	}

	fmt.Println(string(resp.Result))

	return nil
}

func runBrowserPress(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	params := map[string]any{
		"key": args[0],
	}
	if browserPressSelector != "" {
		params["selector"] = browserPressSelector
	}

	resp, err := client.Call(ctx, "browser.press", params)
	if err != nil {
		return fmt.Errorf("browser.press: %w", err)
	}

	fmt.Println(string(resp.Result))

	return nil
}

func runBrowserBack(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	resp, err := client.Call(ctx, "browser.back", nil)
	if err != nil {
		return fmt.Errorf("browser.back: %w", err)
	}

	fmt.Println(string(resp.Result))

	return nil
}

func runBrowserForward(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	resp, err := client.Call(ctx, "browser.forward", nil)
	if err != nil {
		return fmt.Errorf("browser.forward: %w", err)
	}

	fmt.Println(string(resp.Result))

	return nil
}

func runBrowserReload(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	resp, err := client.Call(ctx, "browser.reload", nil)
	if err != nil {
		return fmt.Errorf("browser.reload: %w", err)
	}

	fmt.Println(string(resp.Result))

	return nil
}

func runBrowserDialog(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	params := map[string]any{
		"action": args[0],
	}
	if browserDialogText != "" {
		params["text"] = browserDialogText
	}

	resp, err := client.Call(ctx, "browser.dialog", params)
	if err != nil {
		return fmt.Errorf("browser.dialog: %w", err)
	}

	fmt.Println(string(resp.Result))

	return nil
}

func runBrowserUpload(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	params := map[string]any{
		"selector": args[0],
		"files":    args[1:],
	}

	resp, err := client.Call(ctx, "browser.upload", params)
	if err != nil {
		return fmt.Errorf("browser.upload: %w", err)
	}

	fmt.Println(string(resp.Result))

	return nil
}

func runBrowserPDF(cmd *cobra.Command, args []string) error {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	params := map[string]any{}
	if pdfOutput != "" {
		params["path"] = pdfOutput
	}
	if pdfFormat != "" {
		params["format"] = pdfFormat
	}
	if pdfLandscape {
		params["landscape"] = true
	}

	resp, err := client.Call(ctx, "browser.pdf", params)
	if err != nil {
		return fmt.Errorf("browser.pdf: %w", err)
	}

	fmt.Println(string(resp.Result))

	return nil
}
