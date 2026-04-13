package tui

import (
	"encoding/json"
	"strings"
	"testing"
)

// ── parseSlashCommand ──────────────────────────────────────────────────────────

func TestParseSlashCommand(t *testing.T) {
	t.Run("known command without args", func(t *testing.T) {
		cmd, args := parseSlashCommand("/status")
		if cmd == nil {
			t.Fatal("expected command, got nil")
		}
		if cmd.name != "/status" {
			t.Errorf("name = %q, want %q", cmd.name, "/status")
		}
		if args != "" {
			t.Errorf("args = %q, want empty", args)
		}
	})

	t.Run("known command with args", func(t *testing.T) {
		cmd, args := parseSlashCommand("/tag add mytag")
		if cmd == nil {
			t.Fatal("expected command, got nil")
		}
		if cmd.name != "/tag add" {
			t.Errorf("name = %q, want %q", cmd.name, "/tag add")
		}
		if args != "mytag" {
			t.Errorf("args = %q, want %q", args, "mytag")
		}
	})

	t.Run("unknown command returns nil", func(t *testing.T) {
		cmd, args := parseSlashCommand("/foo")
		if cmd != nil {
			t.Errorf("expected nil for unknown command, got %q", cmd.name)
		}
		if args != "" {
			t.Errorf("args = %q, want empty", args)
		}
	})

	t.Run("non-slash input returns nil", func(t *testing.T) {
		cmd, args := parseSlashCommand("hello world")
		if cmd != nil {
			t.Errorf("expected nil for non-slash input, got %q", cmd.name)
		}
		if args != "" {
			t.Errorf("args = %q, want empty", args)
		}
	})

	t.Run("empty input returns nil", func(t *testing.T) {
		cmd, _ := parseSlashCommand("")
		if cmd != nil {
			t.Errorf("expected nil for empty input, got %q", cmd.name)
		}
	})

	t.Run("prefix ordering: /checkpoints goto beats /checkpoints", func(t *testing.T) {
		cmd, args := parseSlashCommand("/checkpoints goto sha123")
		if cmd == nil {
			t.Fatal("expected command, got nil")
		}
		if cmd.name != "/checkpoints goto" {
			t.Errorf("name = %q, want %q", cmd.name, "/checkpoints goto")
		}
		if args != "sha123" {
			t.Errorf("args = %q, want %q", args, "sha123")
		}
	})

	t.Run("bare /checkpoints matches the shorter form", func(t *testing.T) {
		cmd, args := parseSlashCommand("/checkpoints")
		if cmd == nil {
			t.Fatal("expected command, got nil")
		}
		if cmd.name != "/checkpoints" {
			t.Errorf("name = %q, want %q", cmd.name, "/checkpoints")
		}
		if args != "" {
			t.Errorf("args = %q, want empty", args)
		}
	})

	t.Run("prefix ordering: /changelog full beats /changelog", func(t *testing.T) {
		cmd, args := parseSlashCommand("/changelog full v1..v2")
		if cmd == nil {
			t.Fatal("expected command, got nil")
		}
		if cmd.name != "/changelog full" {
			t.Errorf("name = %q, want %q", cmd.name, "/changelog full")
		}
		if args != "v1..v2" {
			t.Errorf("args = %q, want %q", args, "v1..v2")
		}
	})

	t.Run("prefix ordering: /review fix beats /review", func(t *testing.T) {
		cmd, _ := parseSlashCommand("/review fix")
		if cmd == nil {
			t.Fatal("expected command, got nil")
		}
		if cmd.name != "/review fix" {
			t.Errorf("name = %q, want %q", cmd.name, "/review fix")
		}
	})

	t.Run("prefix ordering: /list search beats /list", func(t *testing.T) {
		cmd, args := parseSlashCommand("/list search myquery")
		if cmd == nil {
			t.Fatal("expected command, got nil")
		}
		if cmd.name != "/list search" {
			t.Errorf("name = %q, want %q", cmd.name, "/list search")
		}
		if args != "myquery" {
			t.Errorf("args = %q, want %q", args, "myquery")
		}
	})

	t.Run("prefix ordering: /show spec and /show plan", func(t *testing.T) {
		cmd, _ := parseSlashCommand("/show spec")
		if cmd == nil {
			t.Fatal("expected /show spec command, got nil")
		}
		if cmd.name != "/show spec" {
			t.Errorf("name = %q, want %q", cmd.name, "/show spec")
		}

		cmd, _ = parseSlashCommand("/show plan")
		if cmd == nil {
			t.Fatal("expected /show plan command, got nil")
		}
		if cmd.name != "/show plan" {
			t.Errorf("name = %q, want %q", cmd.name, "/show plan")
		}
	})

	t.Run("command with extra whitespace in args", func(t *testing.T) {
		cmd, args := parseSlashCommand("/quick   some source  ")
		if cmd == nil {
			t.Fatal("expected command, got nil")
		}
		if cmd.name != "/quick" {
			t.Errorf("name = %q, want %q", cmd.name, "/quick")
		}
		if args != "some source" {
			t.Errorf("args = %q, want %q", args, "some source")
		}
	})

	t.Run("worktree flag is correct", func(t *testing.T) {
		worktreeCmds := []string{"/status", "/plan", "/implement", "/submit", "/tag add"}
		for _, name := range worktreeCmds {
			cmd, _ := parseSlashCommand(name)
			if cmd == nil {
				t.Errorf("%s: expected command, got nil", name)

				continue
			}
			if !cmd.worktree {
				t.Errorf("%s: worktree = false, want true", name)
			}
		}

		globalCmds := []string{"/jobs", "/stats", "/workers", "/diagnose", "/batch"}
		for _, name := range globalCmds {
			cmd, _ := parseSlashCommand(name)
			if cmd == nil {
				t.Errorf("%s: expected command, got nil", name)

				continue
			}
			if cmd.worktree {
				t.Errorf("%s: worktree = true, want false", name)
			}
		}
	})

	t.Run("plan! and implement! variants parse", func(t *testing.T) {
		cmd, _ := parseSlashCommand("/plan!")
		if cmd == nil {
			t.Fatal("expected /plan! command, got nil")
		}
		if cmd.name != "/plan!" {
			t.Errorf("name = %q, want %q", cmd.name, "/plan!")
		}

		cmd, _ = parseSlashCommand("/implement!")
		if cmd == nil {
			t.Fatal("expected /implement! command, got nil")
		}
		if cmd.name != "/implement!" {
			t.Errorf("name = %q, want %q", cmd.name, "/implement!")
		}
	})

	t.Run("all group subcommands parse", func(t *testing.T) {
		groupCmds := []struct {
			input    string
			wantName string
			wantArgs string
		}{
			{"/group create my-group", "/group create", "my-group"},
			{"/group add gid tid", "/group add", "gid tid"},
			{"/group list", "/group list", ""},
			{"/group status gid", "/group status", "gid"},
			{"/group submit gid", "/group submit", "gid"},
			{"/group remove gid", "/group remove", "gid"},
		}
		for _, tc := range groupCmds {
			cmd, args := parseSlashCommand(tc.input)
			if cmd == nil {
				t.Errorf("%s: expected command, got nil", tc.input)

				continue
			}
			if cmd.name != tc.wantName {
				t.Errorf("%s: name = %q, want %q", tc.input, cmd.name, tc.wantName)
			}
			if args != tc.wantArgs {
				t.Errorf("%s: args = %q, want %q", tc.input, args, tc.wantArgs)
			}
		}
	})
}

// ── Handler map coverage ───────────────────────────────────────────────────────

func TestAllCommandsHaveHandlers(t *testing.T) {
	t.Run("every command has a handler", func(t *testing.T) {
		for _, cmd := range commands {
			if cmd.worktree {
				if _, ok := worktreeHandlers[cmd.name]; !ok {
					t.Errorf("worktree command %q has no handler in worktreeHandlers", cmd.name)
				}
			} else {
				if _, ok := globalHandlers[cmd.name]; !ok {
					t.Errorf("global command %q has no handler in globalHandlers", cmd.name)
				}
			}
		}
	})

	t.Run("no orphaned worktree handlers", func(t *testing.T) {
		cmdSet := make(map[string]bool)
		for _, cmd := range commands {
			cmdSet[cmd.name] = true
		}
		for name := range worktreeHandlers {
			if !cmdSet[name] {
				t.Errorf("worktreeHandlers contains %q which is not in the commands slice", name)
			}
		}
	})

	t.Run("no orphaned global handlers", func(t *testing.T) {
		cmdSet := make(map[string]bool)
		for _, cmd := range commands {
			cmdSet[cmd.name] = true
		}
		for name := range globalHandlers {
			if !cmdSet[name] {
				t.Errorf("globalHandlers contains %q which is not in the commands slice", name)
			}
		}
	})
}

// ── mustJSON ───────────────────────────────────────────────────────────────────

func TestMustJSON(t *testing.T) {
	t.Run("string value", func(t *testing.T) {
		got := string(mustJSON("hello"))
		if got != `"hello"` {
			t.Errorf("mustJSON(\"hello\") = %s, want %q", got, `"hello"`)
		}
	})

	t.Run("map value", func(t *testing.T) {
		got := string(mustJSON(map[string]any{"key": "val", "num": 42}))
		var parsed map[string]any
		if err := json.Unmarshal([]byte(got), &parsed); err != nil {
			t.Fatalf("failed to parse output: %v", err)
		}
		if parsed["key"] != "val" {
			t.Errorf("key = %v, want %q", parsed["key"], "val")
		}
		if parsed["num"] != float64(42) {
			t.Errorf("num = %v, want 42", parsed["num"])
		}
	})

	t.Run("nil value", func(t *testing.T) {
		got := string(mustJSON(nil))
		if got != "null" {
			t.Errorf("mustJSON(nil) = %s, want %q", got, "null")
		}
	})

	t.Run("bool value", func(t *testing.T) {
		got := string(mustJSON(true))
		if got != "true" {
			t.Errorf("mustJSON(true) = %s, want %q", got, "true")
		}
	})

	t.Run("panics on unmarshalable value", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for channel type, got none")
			}
		}()
		mustJSON(make(chan int))
	})
}

// ── optDryRun ──────────────────────────────────────────────────────────────────

func TestOptDryRun(t *testing.T) {
	t.Run("false returns nil", func(t *testing.T) {
		got := optDryRun(false)
		if got != nil {
			t.Errorf("optDryRun(false) = %s, want nil", string(got))
		}
	})

	t.Run("true returns JSON with dry_run", func(t *testing.T) {
		got := optDryRun(true)
		if got == nil {
			t.Fatal("optDryRun(true) = nil, want non-nil")
		}
		var parsed map[string]any
		if err := json.Unmarshal(got, &parsed); err != nil {
			t.Fatalf("failed to parse: %v", err)
		}
		if parsed["dry_run"] != true {
			t.Errorf("dry_run = %v, want true", parsed["dry_run"])
		}
	})
}

// ── formatTaskList ─────────────────────────────────────────────────────────────

func TestFormatTaskList(t *testing.T) {
	t.Run("empty tasks", func(t *testing.T) {
		data := json.RawMessage(`{"tasks": []}`)
		got := formatTaskList(data)
		if got != "No tasks." {
			t.Errorf("got %q, want %q", got, "No tasks.")
		}
	})

	t.Run("null tasks field", func(t *testing.T) {
		data := json.RawMessage(`{}`)
		got := formatTaskList(data)
		if got != "No tasks." {
			t.Errorf("got %q, want %q", got, "No tasks.")
		}
	})

	t.Run("single task", func(t *testing.T) {
		data := json.RawMessage(`{"tasks": [{"id": "abcdef1234567890", "title": "Fix bug", "state": "loaded"}]}`)
		got := formatTaskList(data)
		if !strings.Contains(got, "abcdef12") {
			t.Errorf("expected truncated ID 'abcdef12' in %q", got)
		}
		if !strings.Contains(got, "[loaded]") {
			t.Errorf("expected '[loaded]' in %q", got)
		}
		if !strings.Contains(got, "Fix bug") {
			t.Errorf("expected 'Fix bug' in %q", got)
		}
	})

	t.Run("multiple tasks", func(t *testing.T) {
		data := json.RawMessage(`{"tasks": [
			{"id": "aaaa1111bbbb2222", "title": "Task A", "state": "planned"},
			{"id": "cccc3333dddd4444", "title": "Task B", "state": "implementing"}
		]}`)
		got := formatTaskList(data)
		lines := strings.Split(got, "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 lines, got %d: %q", len(lines), got)
		}
		if !strings.Contains(lines[0], "Task A") {
			t.Errorf("line 0 = %q, expected 'Task A'", lines[0])
		}
		if !strings.Contains(lines[1], "Task B") {
			t.Errorf("line 1 = %q, expected 'Task B'", lines[1])
		}
	})

	t.Run("short ID not truncated beyond length", func(t *testing.T) {
		data := json.RawMessage(`{"tasks": [{"id": "abc", "title": "Short", "state": "none"}]}`)
		got := formatTaskList(data)
		if !strings.HasPrefix(got, "abc") {
			t.Errorf("short ID should not be over-truncated: %q", got)
		}
	})

	t.Run("invalid JSON returns no tasks", func(t *testing.T) {
		data := json.RawMessage(`not json`)
		got := formatTaskList(data)
		if got != "No tasks." {
			t.Errorf("got %q, want %q", got, "No tasks.")
		}
	})
}

// ── Handler smoke tests (arg validation) ───────────────────────────────────────

func TestWorktreeHandlerArgValidation(t *testing.T) {
	// These handlers require args and should return usage messages when called
	// with empty args. We cannot call them with a real socket.Client, but we
	// can verify that the handlers exist and have the correct type signature
	// by looking at representative handlers that do arg checking before any
	// RPC call.

	tests := []struct {
		name    string
		handler worktreeHandler
		wantMsg string
	}{
		{"/quick (no args)", worktreeHandlers["/quick"], "Usage: /quick"},
		{"/tag add (no args)", worktreeHandlers["/tag add"], "Usage: /tag add"},
		{"/tag remove (no args)", worktreeHandlers["/tag remove"], "Usage: /tag remove"},
		{"/queue add (no args)", worktreeHandlers["/queue add"], "Usage: /queue add"},
		{"/queue remove (no args)", worktreeHandlers["/queue remove"], "Usage: /queue remove"},
		{"/fork create (no args)", worktreeHandlers["/fork create"], "Usage: /fork create"},
		{"/fork select (no args)", worktreeHandlers["/fork select"], "Usage: /fork select"},
		{"/checkpoints goto (no args)", worktreeHandlers["/checkpoints goto"], "Usage: /checkpoints goto"},
		{"/list search (no args)", worktreeHandlers["/list search"], "Usage: /list search"},
		{"/checklist check (no args)", worktreeHandlers["/checklist check"], "Usage: /checklist check"},
		{"/checklist uncheck (no args)", worktreeHandlers["/checklist uncheck"], "Usage: /checklist uncheck"},
		{"/files search (no args)", worktreeHandlers["/files search"], "Usage: /files search"},
		{"/codegraph search (no args)", worktreeHandlers["/codegraph search"], "Usage: /codegraph search"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.handler == nil {
				t.Fatal("handler is nil")
			}
			// Call with nil client and empty args. Handlers that validate args
			// before making RPC calls should return a usage string without
			// touching the client.
			got, err := tt.handler(nil, nil, "", false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(got, tt.wantMsg) {
				t.Errorf("got %q, want it to contain %q", got, tt.wantMsg)
			}
		})
	}
}

func TestGlobalHandlerArgValidation(t *testing.T) {
	tests := []struct {
		name    string
		handler globalHandler
		wantMsg string
	}{
		{"/memory search (no args)", globalHandlers["/memory search"], "Usage: /memory search"},
		{"/group create (no args)", globalHandlers["/group create"], "Usage: /group create"},
		{"/group add (no args)", globalHandlers["/group add"], "Usage: /group add"},
		{"/group status (no args)", globalHandlers["/group status"], "Usage: /group status"},
		{"/group submit (no args)", globalHandlers["/group submit"], "Usage: /group submit"},
		{"/group remove (no args)", globalHandlers["/group remove"], "Usage: /group remove"},
		{"/batch (no args)", globalHandlers["/batch"], "Usage: /batch"},
		{"/projects unregister (no args)", globalHandlers["/projects unregister"], "Usage: /projects unregister"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.handler == nil {
				t.Fatal("handler is nil")
			}
			got, err := tt.handler(nil, nil, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(got, tt.wantMsg) {
				t.Errorf("got %q, want it to contain %q", got, tt.wantMsg)
			}
		})
	}
}

// ── Changelog arg validation ───────────────────────────────────────────────────

func TestChangelogHandlerArgValidation(t *testing.T) {
	handler := worktreeHandlers["/changelog"]
	if handler == nil {
		t.Fatal("/changelog handler is nil")
	}

	t.Run("empty args", func(t *testing.T) {
		got, err := handler(nil, nil, "", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(got, "Usage:") {
			t.Errorf("got %q, want usage message", got)
		}
	})

	t.Run("missing dotdot separator", func(t *testing.T) {
		got, err := handler(nil, nil, "v1 v2", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(got, "Usage:") {
			t.Errorf("got %q, want usage message for bad format", got)
		}
	})

	t.Run("empty source in range", func(t *testing.T) {
		got, err := handler(nil, nil, "..v2", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(got, "Usage:") {
			t.Errorf("got %q, want usage message for empty source", got)
		}
	})

	t.Run("empty target in range", func(t *testing.T) {
		got, err := handler(nil, nil, "v1..", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(got, "Usage:") {
			t.Errorf("got %q, want usage message for empty target", got)
		}
	})
}

// ── Command count consistency ──────────────────────────────────────────────────

func TestCommandCountConsistency(t *testing.T) {
	// Count commands by scope
	var worktreeCount, globalCount int
	for _, cmd := range commands {
		if cmd.worktree {
			worktreeCount++
		} else {
			globalCount++
		}
	}

	t.Run("worktree handler count matches", func(t *testing.T) {
		if len(worktreeHandlers) < worktreeCount {
			t.Errorf("worktreeHandlers has %d entries but need at least %d for worktree commands", len(worktreeHandlers), worktreeCount)
		}
	})

	t.Run("global handler count matches", func(t *testing.T) {
		if len(globalHandlers) < globalCount {
			t.Errorf("globalHandlers has %d entries but need at least %d for global commands", len(globalHandlers), globalCount)
		}
	})
}

// ── Commands slice ordering invariant ──────────────────────────────────────────

func TestCommandsLongerPrefixesFirst(t *testing.T) {
	// The comment on the commands slice says longer names MUST appear before
	// shorter prefixes. Verify this invariant for known prefix groups.
	prefixGroups := map[string][]string{
		"/checkpoints": {"/checkpoints goto", "/checkpoints"},
		"/changelog":   {"/changelog full", "/changelog"},
		"/review":      {"/review fix", "/review"},
		"/list":        {"/list search", "/list"},
		"/show":        {"/show spec", "/show plan"},
		"/queue":       {"/queue add", "/queue remove", "/queue list", "/queue"},
		"/fork":        {"/fork create", "/fork list", "/fork compare", "/fork select"},
		"/checklist":   {"/checklist check", "/checklist uncheck", "/checklist"},
		"/files":       {"/files search", "/files"},
		"/cache":       {"/cache stats", "/cache clear"},
		"/memory":      {"/memory search", "/memory stats"},
		"/group":       {"/group create", "/group add", "/group list", "/group status", "/group submit", "/group remove"},
		"/git":         {"/git status", "/git log"},
		"/remote":      {"/remote approve", "/remote merge"},
	}

	// Build index: command name -> position in commands slice
	nameIndex := make(map[string]int)
	for i, cmd := range commands {
		nameIndex[cmd.name] = i
	}

	for prefix, group := range prefixGroups {
		// For each group, if there is a bare prefix command, all subcommands
		// must appear before it.
		bareIdx, hasBare := nameIndex[prefix]
		if !hasBare {
			continue
		}
		for _, sub := range group {
			if sub == prefix {
				continue
			}
			subIdx, ok := nameIndex[sub]
			if !ok {
				t.Errorf("command %q not found in commands slice", sub)

				continue
			}
			if subIdx > bareIdx {
				t.Errorf("command %q (index %d) appears after %q (index %d); longer prefix must come first",
					sub, subIdx, prefix, bareIdx)
			}
		}
	}
}

// ── Command descriptions are non-empty ─────────────────────────────────────────

func TestAllCommandsHaveDescriptions(t *testing.T) {
	for _, cmd := range commands {
		if cmd.description == "" {
			t.Errorf("command %q has empty description", cmd.name)
		}
	}
}

// ── Command names start with slash ─────────────────────────────────────────────

func TestAllCommandNamesStartWithSlash(t *testing.T) {
	for _, cmd := range commands {
		if !strings.HasPrefix(cmd.name, "/") {
			t.Errorf("command name %q does not start with '/'", cmd.name)
		}
	}
}

// ── No duplicate command names ─────────────────────────────────────────────────

func TestNoDuplicateCommandNames(t *testing.T) {
	seen := make(map[string]bool)
	for _, cmd := range commands {
		if seen[cmd.name] {
			t.Errorf("duplicate command name: %q", cmd.name)
		}
		seen[cmd.name] = true
	}
}
