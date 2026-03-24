package activitylog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeValidJSONL(t *testing.T, path string, entries []Entry) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	defer func() { _ = f.Close() }()

	prevHash := ""
	for _, e := range entries {
		e.PrevHash = prevHash
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		h := sha256.Sum256(data)
		prevHash = hex.EncodeToString(h[:])
		data = append(data, '\n')
		if _, err := f.Write(data); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
}

func writeValidJSONLWithPrev(t *testing.T, path string, entries []Entry, startPrevHash string) string {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	defer func() { _ = f.Close() }()

	prevHash := startPrevHash
	for _, e := range entries {
		e.PrevHash = prevHash
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		h := sha256.Sum256(data)
		prevHash = hex.EncodeToString(h[:])
		data = append(data, '\n')
		if _, err := f.Write(data); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	return prevHash
}

func TestVerifyChain_Valid(t *testing.T) {
	l := newTestLog(t)
	now := time.Now()
	l.nowFunc = func() time.Time { return now }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		l.Start(ctx)
		close(done)
	}()

	for i := range 5 {
		l.Record(Entry{
			Timestamp:  now.Add(time.Duration(i) * time.Second),
			Method:     "test.method",
			DurationMs: int64(10 + i),
			ParamsSize: 100 + i,
		})
	}

	cancel()
	<-done
	l.Close()

	result, err := l.VerifyChain()
	if err != nil {
		t.Fatalf("VerifyChain: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid chain, got error at line %d: %s", result.ErrorLine, result.ErrorMsg)
	}
	if result.EntriesRead != 5 {
		t.Errorf("EntriesRead = %d, want 5", result.EntriesRead)
	}
}

func TestVerifyChain_Tampered(t *testing.T) {
	l := newTestLog(t)
	now := time.Now()
	l.nowFunc = func() time.Time { return now }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		l.Start(ctx)
		close(done)
	}()

	for i := range 3 {
		l.Record(Entry{
			Timestamp:  now.Add(time.Duration(i) * time.Second),
			Method:     "test.method",
			DurationMs: int64(10 + i),
			ParamsSize: 100 + i,
		})
	}

	cancel()
	<-done
	l.Close()

	// Find the log file and tamper with the second line.
	files, err := l.logFilesSorted()
	if err != nil {
		t.Fatalf("logFilesSorted: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no log files found")
	}

	path := filepath.Join(l.dir, files[0])
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	// Replace a character in the middle of the file to break the chain.
	tampered := make([]byte, len(data))
	copy(tampered, data)
	mid := len(tampered) / 2
	if tampered[mid] == 'x' {
		tampered[mid] = 'y'
	} else {
		tampered[mid] = 'x'
	}
	if err := os.WriteFile(path, tampered, filePermissions); err != nil {
		t.Fatalf("write tampered: %v", err)
	}

	result, err := l.VerifyChain()
	if err != nil {
		t.Fatalf("VerifyChain: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid chain after tampering")
	}
	if result.ErrorLine == 0 {
		t.Error("expected non-zero ErrorLine")
	}
	if result.ErrorMsg == "" {
		t.Error("expected non-empty ErrorMsg")
	}
}

func TestVerifyChainForFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "activity-2026-03-24.jsonl")
	now := time.Now()

	writeValidJSONL(t, path, []Entry{
		{Timestamp: now, Method: "a", DurationMs: 1, ParamsSize: 10},
		{Timestamp: now.Add(time.Second), Method: "b", DurationMs: 2, ParamsSize: 20},
		{Timestamp: now.Add(2 * time.Second), Method: "c", DurationMs: 3, ParamsSize: 30},
	})

	result, err := VerifyChainForFile(path)
	if err != nil {
		t.Fatalf("VerifyChainForFile: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid, got error at line %d: %s", result.ErrorLine, result.ErrorMsg)
	}
	if result.EntriesRead != 3 {
		t.Errorf("EntriesRead = %d, want 3", result.EntriesRead)
	}
}

func TestVerifyChainForFile_BrokenChain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	now := time.Now()

	// Write first entry with correct empty prev_hash.
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	e1 := Entry{Timestamp: now, Method: "a", DurationMs: 1, ParamsSize: 10, PrevHash: ""}
	data1, err := json.Marshal(e1)
	if err != nil {
		t.Fatalf("marshal e1: %v", err)
	}
	_, _ = f.Write(append(data1, '\n'))

	// Write second entry with wrong prev_hash.
	e2 := Entry{Timestamp: now.Add(time.Second), Method: "b", DurationMs: 2, ParamsSize: 20, PrevHash: "wrong_hash"}
	data2, err := json.Marshal(e2)
	if err != nil {
		t.Fatalf("marshal e2: %v", err)
	}
	_, _ = f.Write(append(data2, '\n'))
	_ = f.Close()

	result, err := VerifyChainForFile(path)
	if err != nil {
		t.Fatalf("VerifyChainForFile: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid chain")
	}
	if result.ErrorLine != 2 {
		t.Errorf("ErrorLine = %d, want 2", result.ErrorLine)
	}
}

func TestVerifyChainForFile_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	if err := os.WriteFile(path, []byte("not valid json\n"), filePermissions); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := VerifyChainForFile(path)
	if err != nil {
		t.Fatalf("VerifyChainForFile: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result for malformed JSON")
	}
	if result.ErrorMsg == "" {
		t.Error("expected non-empty ErrorMsg")
	}
}

func TestVerifyChainDir_Valid(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	entries1 := []Entry{
		{Timestamp: now, Method: "a", DurationMs: 1, ParamsSize: 10},
		{Timestamp: now.Add(time.Second), Method: "b", DurationMs: 2, ParamsSize: 20},
	}
	entries2 := []Entry{
		{Timestamp: now.Add(2 * time.Second), Method: "c", DurationMs: 3, ParamsSize: 30},
		{Timestamp: now.Add(3 * time.Second), Method: "d", DurationMs: 4, ParamsSize: 40},
	}

	path1 := filepath.Join(dir, "activity-2026-03-23.jsonl")
	lastHash := writeValidJSONLWithPrev(t, path1, entries1, "")

	path2 := filepath.Join(dir, "activity-2026-03-24.jsonl")
	writeValidJSONLWithPrev(t, path2, entries2, lastHash)

	result, err := VerifyChainDir(dir)
	if err != nil {
		t.Fatalf("VerifyChainDir: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid, got error at line %d: %s", result.ErrorLine, result.ErrorMsg)
	}
	if result.EntriesRead != 4 {
		t.Errorf("EntriesRead = %d, want 4", result.EntriesRead)
	}
}

func TestVerifyChainDir_BreakInSecondFile(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	entries1 := []Entry{
		{Timestamp: now, Method: "a", DurationMs: 1, ParamsSize: 10},
	}
	path1 := filepath.Join(dir, "activity-2026-03-23.jsonl")
	writeValidJSONLWithPrev(t, path1, entries1, "")

	// Write second file with wrong starting prev_hash (breaks cross-file chain).
	path2 := filepath.Join(dir, "activity-2026-03-24.jsonl")
	writeValidJSONLWithPrev(t, path2, []Entry{
		{Timestamp: now.Add(time.Second), Method: "b", DurationMs: 2, ParamsSize: 20},
	}, "wrong_cross_file_hash")

	result, err := VerifyChainDir(dir)
	if err != nil {
		t.Fatalf("VerifyChainDir: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid chain in second file")
	}
	if result.ErrorLine != 2 {
		t.Errorf("ErrorLine = %d, want 2", result.ErrorLine)
	}
}

func TestLastHashFromFile_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(path, []byte{}, filePermissions); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := lastHashFromFile(path)
	if got != "" {
		t.Errorf("lastHashFromFile(empty) = %q, want empty", got)
	}
}

func TestLastHashFromFile_SingleLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "single.jsonl")
	now := time.Now()

	e := Entry{Timestamp: now, Method: "a", DurationMs: 1, ParamsSize: 10, PrevHash: ""}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), filePermissions); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := lastHashFromFile(path)
	h := sha256.Sum256(data)
	want := hex.EncodeToString(h[:])

	if got != want {
		t.Errorf("lastHashFromFile = %q, want %q", got, want)
	}
}

func TestLastHashFromFile_Nonexistent(t *testing.T) {
	got := lastHashFromFile("/nonexistent/path/does/not/exist.jsonl")
	if got != "" {
		t.Errorf("lastHashFromFile(nonexistent) = %q, want empty", got)
	}
}

func TestMatchesMethodPattern(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		pattern string
		want    bool
	}{
		{name: "empty pattern matches all", method: "task.start", pattern: "", want: true},
		{name: "single pattern match", method: "task.start", pattern: "start", want: true},
		{name: "single pattern no match", method: "task.start", pattern: "plan", want: false},
		{name: "pipe separated match first", method: "task.start", pattern: "start|plan", want: true},
		{name: "pipe separated match second", method: "task.plan", pattern: "start|plan", want: true},
		{name: "pipe separated no match", method: "task.review", pattern: "start|plan", want: false},
		{name: "empty method empty pattern", method: "", pattern: "", want: true},
		{name: "empty method non-empty pattern", method: "", pattern: "start", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesMethodPattern(tt.method, tt.pattern)
			if got != tt.want {
				t.Errorf("matchesMethodPattern(%q, %q) = %v, want %v", tt.method, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestDayFromFileName(t *testing.T) {
	tests := []struct {
		name string
		file string
		want string
	}{
		{name: "standard name", file: "activity-2026-03-24.jsonl", want: "2026-03-24"},
		{name: "different date", file: "activity-2025-01-01.jsonl", want: "2025-01-01"},
		{name: "no prefix or suffix", file: "2026-03-24", want: "2026-03-24"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dayFromFileName(tt.file)
			if got != tt.want {
				t.Errorf("dayFromFileName(%q) = %q, want %q", tt.file, got, tt.want)
			}
		})
	}
}
