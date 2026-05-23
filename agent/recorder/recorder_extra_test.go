package recorder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"plain", "plain"},
		{"with/slash", "with_slash"},
		{"back\\slash", "back_slash"},
		{"colon:here", "colon_here"},
		{"tab\there", "tab_here"},
		{"newline\nhere", "newline_here"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := sanitizeFilename(tt.in); got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNew_InvalidJobID(t *testing.T) {
	cfg := Config{Dir: t.TempDir(), JobID: "bad id with spaces"}
	if _, err := New(cfg); err == nil {
		t.Error("New() should reject job ID with spaces")
	}
}

func TestNew_InvalidAgentName(t *testing.T) {
	cfg := Config{Dir: t.TempDir(), JobID: "ok-job", Agent: "bad agent!"}
	if _, err := New(cfg); err == nil {
		t.Error("New() should reject agent name with unsafe characters")
	}
}

func TestNew_DefaultsFilledIn(t *testing.T) {
	// Empty MaxLines should be backfilled from DefaultConfig.
	cfg := Config{
		Dir:      t.TempDir(),
		JobID:    "defaults-job",
		MaxLines: -5, // invalid → default
	}

	r, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = r.Close() }()

	if r.maxLines != DefaultConfig().MaxLines {
		t.Errorf("maxLines = %d, want default %d", r.maxLines, DefaultConfig().MaxLines)
	}
}

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxLines != 100000 {
		t.Errorf("DefaultConfig().MaxLines = %d, want 100000", cfg.MaxLines)
	}
	if cfg.Dir == "" {
		t.Error("DefaultConfig().Dir should not be empty")
	}
	if !strings.Contains(cfg.Dir, "recordings") {
		t.Errorf("DefaultConfig().Dir = %q, want it to end in recordings", cfg.Dir)
	}
}

func TestRecord_AfterClose(t *testing.T) {
	dir := t.TempDir()
	r, err := New(Config{Dir: dir, JobID: "closed-job", Agent: "claude"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	_ = r.Close()

	err = r.Record(Record{Direction: Outbound, Type: "stream", Event: json.RawMessage(`{}`)})
	if err == nil {
		t.Fatal("Record() on closed recorder should error")
	}
	if !strings.Contains(err.Error(), "closed") {
		t.Errorf("expected closed error, got %v", err)
	}
}

func TestRecord_SanitizesEvent(t *testing.T) {
	dir := t.TempDir()
	// Build a fake GitHub-token-shaped secret at runtime so no literal credential
	// is committed to source.
	secret := "ghp_" + strings.Repeat("A", 36)
	san := NewSanitizer([]string{secret})

	r, err := New(Config{Dir: dir, JobID: "sani-job", Agent: "claude", Sanitizer: san})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := r.RecordOutbound("stream", map[string]string{"content": "secret " + secret}); err != nil {
		t.Fatalf("RecordOutbound() error: %v", err)
	}
	path := r.Path()
	_ = r.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), secret) {
		t.Error("recorded data still contains the raw secret; sanitizer not applied")
	}
}

func TestRecordOutbound_MarshalError(t *testing.T) {
	dir := t.TempDir()
	r, err := New(Config{Dir: dir, JobID: "marshal-job", Agent: "claude"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = r.Close() }()

	// channels cannot be JSON-marshaled.
	err = r.RecordOutbound("stream", make(chan int))
	if err == nil {
		t.Fatal("RecordOutbound() should fail to marshal an unmarshalable value")
	}
	if !strings.Contains(err.Error(), "marshal event") {
		t.Errorf("expected marshal error, got %v", err)
	}
}

func TestScratchpad_FromRecorder(t *testing.T) {
	dir := t.TempDir()
	r, err := New(Config{Dir: dir, JobID: "pad-job", Agent: "claude"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	_ = r.RecordInbound("plan it")
	_ = r.RecordOutbound("assistant", map[string]string{"content": "thinking"})
	_ = r.RecordOutbound("tool_use", map[string]any{"name": "Read", "input": "main.go"})
	_ = r.RecordOutbound("complete", map[string]string{"content": "done"})

	pad, err := r.Scratchpad("plan")
	if err != nil {
		t.Fatalf("Scratchpad() error: %v", err)
	}
	_ = r.Close()

	if pad.JobID != "pad-job" {
		t.Errorf("pad.JobID = %q, want pad-job", pad.JobID)
	}
	if pad.Phase != "plan" {
		t.Errorf("pad.Phase = %q, want plan", pad.Phase)
	}
	// assistant, tool_use, complete → 3 thoughts (inbound prompt skipped).
	if len(pad.Thoughts) != 3 {
		t.Fatalf("len(pad.Thoughts) = %d, want 3: %+v", len(pad.Thoughts), pad.Thoughts)
	}
	if pad.Thoughts[1].ToolName != "Read" {
		t.Errorf("tool thought ToolName = %q, want Read", pad.Thoughts[1].ToolName)
	}
}

func TestBuildScratchpadFromFile(t *testing.T) {
	dir := t.TempDir()
	r, err := New(Config{Dir: dir, JobID: "file-pad", Agent: "claude"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	_ = r.RecordOutbound("assistant", map[string]string{"content": "reasoning step"})
	_ = r.RecordOutbound("complete", map[string]string{"content": "finished"})
	path := r.Path()
	_ = r.Close()

	pad, err := BuildScratchpadFromFile(path)
	if err != nil {
		t.Fatalf("BuildScratchpadFromFile() error: %v", err)
	}
	if pad.JobID != "file-pad" {
		t.Errorf("pad.JobID = %q, want file-pad", pad.JobID)
	}
	if len(pad.Thoughts) != 2 {
		t.Errorf("len(pad.Thoughts) = %d, want 2", len(pad.Thoughts))
	}
}

func TestBuildScratchpadFromFile_MissingFile(t *testing.T) {
	_, err := BuildScratchpadFromFile("/nonexistent/recording.jsonl")
	if err == nil {
		t.Fatal("BuildScratchpadFromFile() should fail for missing file")
	}
}

func TestBuildScratchpadFromFile_EmptyRecords(t *testing.T) {
	// A file with only a header line yields zero data records.
	dir := t.TempDir()
	path := filepath.Join(dir, "header-only.jsonl")
	header, err := json.Marshal(Header{JobID: "empty"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(header, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	pad, err := BuildScratchpadFromFile(path)
	if err != nil {
		t.Fatalf("BuildScratchpadFromFile() error: %v", err)
	}
	if len(pad.Thoughts) != 0 {
		t.Errorf("expected 0 thoughts for header-only file, got %d", len(pad.Thoughts))
	}
}

func TestExtractField(t *testing.T) {
	tests := []struct {
		name  string
		raw   json.RawMessage
		field string
		want  string
	}{
		{"string field", json.RawMessage(`{"name":"Read"}`), "name", "Read"},
		{"missing field", json.RawMessage(`{"other":"x"}`), "name", ""},
		{"empty raw", json.RawMessage(``), "name", ""},
		{"malformed json", json.RawMessage(`{bad`), "name", ""},
		{"non-string field", json.RawMessage(`{"input":{"path":"x"}}`), "input", `{"path":"x"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractField(tt.raw, tt.field)
			if got != tt.want {
				t.Errorf("extractField() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractField_TruncatesLongValue(t *testing.T) {
	// A non-string field longer than 200 runes should be truncated with "...".
	long := strings.Repeat("z", 300)
	raw := json.RawMessage(`{"input":[` + quoteList(long) + `]}`)
	got := extractField(raw, "input")
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected truncated value ending in ..., got len %d", len(got))
	}
}

func TestExtractContent_TruncatesLongRawJSON(t *testing.T) {
	// extractContent falls through to truncating raw JSON when it's neither a
	// string nor an object with content/message.
	long := strings.Repeat("9", 300)
	raw := json.RawMessage(`[` + long + `]`)
	got := extractContent(raw)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected truncated content ending in ..., got %q (len %d)", got, len(got))
	}
}

func TestRecordToThought_SkipsEmptyAssistant(t *testing.T) {
	seq := 0
	// Assistant with empty content returns nil and decrements seq.
	got := recordToThought(Record{Type: "assistant", Event: json.RawMessage(`{"content":""}`)}, &seq)
	if got != nil {
		t.Errorf("expected nil for empty assistant content, got %+v", got)
	}
	if seq != 0 {
		t.Errorf("seq should be reset to 0, got %d", seq)
	}
}

func TestRecordToThought_SkipsUnknownType(t *testing.T) {
	seq := 5
	got := recordToThought(Record{Type: "weird", Event: json.RawMessage(`{}`)}, &seq)
	if got != nil {
		t.Errorf("expected nil for unknown type, got %+v", got)
	}
	if seq != 5 {
		t.Errorf("seq should be restored to 5, got %d", seq)
	}
}

func TestRecordToThought_ToolUseFallsBackToName(t *testing.T) {
	seq := 0
	// tool_use with no input should fall back to the tool name as content.
	got := recordToThought(Record{Type: "tool_use", Event: json.RawMessage(`{"name":"Bash"}`)}, &seq)
	if got == nil {
		t.Fatal("expected a thought for tool_use")
	}
	if got.Content != "Bash" {
		t.Errorf("Content = %q, want Bash (fallback to name)", got.Content)
	}
}

func TestRecordToThought_CompleteFallback(t *testing.T) {
	seq := 0
	// Empty event → extractContent returns "" → complete falls back to a default.
	got := recordToThought(Record{Type: "complete", Event: json.RawMessage(``)}, &seq)
	if got == nil {
		t.Fatal("expected a thought for complete")
	}
	if got.Content != "Agent completed" {
		t.Errorf("Content = %q, want 'Agent completed' fallback", got.Content)
	}
}

func TestRecordToThought_SkipsEmptyToolResult(t *testing.T) {
	seq := 3
	got := recordToThought(Record{Type: "tool_result", Event: json.RawMessage(``)}, &seq)
	if got != nil {
		t.Errorf("expected nil for empty tool_result, got %+v", got)
	}
	if seq != 3 {
		t.Errorf("seq should be restored to 3, got %d", seq)
	}
}

func TestOpenReader_MissingFile(t *testing.T) {
	if _, err := OpenReader("/nonexistent/file.jsonl"); err == nil {
		t.Fatal("OpenReader() should fail for missing file")
	}
}

func TestOpenReader_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenReader(path); err == nil {
		t.Fatal("OpenReader() should fail for empty file")
	}
}

func TestOpenReader_BadHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad-header.jsonl")
	if err := os.WriteFile(path, []byte("not json at all\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenReader(path); err == nil {
		t.Fatal("OpenReader() should fail to parse a non-JSON header")
	}
}

func TestNext_MalformedRecord(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "malformed.jsonl")
	header, err := json.Marshal(Header{JobID: "j"})
	if err != nil {
		t.Fatal(err)
	}
	content := string(header) + "\n" + "{not valid json}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	reader, err := OpenReader(path)
	if err != nil {
		t.Fatalf("OpenReader() error: %v", err)
	}
	defer func() { _ = reader.Close() }()

	if _, err := reader.Next(); err == nil {
		t.Fatal("Next() should fail on a malformed record line")
	}
}

func TestNext_EOFReturnsNilNil(t *testing.T) {
	dir := t.TempDir()
	r, err := New(Config{Dir: dir, JobID: "eof-job", Agent: "claude"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	path := r.Path()
	_ = r.Close()

	reader, err := OpenReader(path)
	if err != nil {
		t.Fatalf("OpenReader() error: %v", err)
	}
	defer func() { _ = reader.Close() }()

	rec, err := reader.Next()
	if err != nil {
		t.Fatalf("Next() error: %v", err)
	}
	if rec != nil {
		t.Errorf("Next() at EOF should return nil record, got %+v", rec)
	}
}

func TestCleanOldRecordings_NonexistentDir(t *testing.T) {
	removed, err := CleanOldRecordings("/nonexistent/dir", time.Now().Unix())
	if err != nil {
		t.Fatalf("CleanOldRecordings() error: %v", err)
	}
	if removed != 0 {
		t.Errorf("expected 0 removed for missing dir, got %d", removed)
	}
}

func TestCleanOldRecordings_SkipsNonJSONL(t *testing.T) {
	dir := t.TempDir()
	// A non-jsonl old file must not be removed.
	other := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(other, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(other, old, old)

	removed, err := CleanOldRecordings(dir, time.Now().Unix())
	if err != nil {
		t.Fatalf("CleanOldRecordings() error: %v", err)
	}
	if removed != 0 {
		t.Errorf("non-jsonl file should be skipped, removed %d", removed)
	}
	if _, err := os.Stat(other); os.IsNotExist(err) {
		t.Error("non-jsonl file should not have been removed")
	}
}

func TestRecorder_RotationWritesHeaderToNewFile(t *testing.T) {
	dir := t.TempDir()
	r, err := New(Config{Dir: dir, JobID: "rot-job", Agent: "claude", MaxLines: 3})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = r.Close() }()

	// Header (1) + 2 records = 3 lines, then the next Record triggers rotation.
	for range 6 {
		if err := r.RecordInbound("p"); err != nil {
			t.Fatalf("RecordInbound() error: %v", err)
		}
	}
	_ = r.Flush()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 2 {
		t.Fatalf("expected rotation to create >=2 files, got %d", len(entries))
	}

	// Every rotated file must start with a valid header.
	for _, e := range entries {
		reader, oerr := OpenReader(filepath.Join(dir, e.Name()))
		if oerr != nil {
			t.Errorf("OpenReader(%s) error: %v", e.Name(), oerr)

			continue
		}
		if reader.Header() == nil || reader.Header().JobID != "rot-job" {
			t.Errorf("file %s missing valid header", e.Name())
		}
		_ = reader.Close()
	}
}

// quoteList wraps a string in JSON quotes for embedding in an array literal.
// strconv.Quote yields a valid JSON string literal for ASCII content.
func quoteList(s string) string {
	return strconv.Quote(s)
}
