package memory

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewAdapterAuto_ForcesTFIDF(t *testing.T) {
	t.Setenv("KVELMO_EMBEDDER", "tfidf")
	dir := t.TempDir()

	adapter, indexer, err := NewAdapterAuto(context.Background(), dir)
	if err != nil {
		t.Fatalf("NewAdapterAuto error = %v", err)
	}
	if adapter == nil || indexer == nil {
		t.Fatal("NewAdapterAuto returned nil components")
	}
	if got := adapter.Stats().Embedder; got != "tfidf" {
		t.Errorf("embedder = %q, want tfidf", got)
	}
	if adapter.Store() == nil {
		t.Error("Store() returned nil")
	}
}

func TestSelectEmbedder_TFIDFOverride(t *testing.T) {
	t.Setenv("KVELMO_EMBEDDER", "TFIDF") // case-insensitive
	e := selectEmbedder()
	if e.Name() != "tfidf" {
		t.Errorf("selectEmbedder name = %q, want tfidf", e.Name())
	}
}

func TestDefaultModelsDir(t *testing.T) {
	dir := defaultModelsDir()
	// On a normal machine this resolves to ~/.valksor/kvelmo/models. It may be
	// empty only if the home dir is unavailable; assert it's well-formed when set.
	if dir != "" && !strings.Contains(dir, filepath.Join(".valksor", "kvelmo", "models")) {
		t.Errorf("defaultModelsDir = %q, unexpected shape", dir)
	}
}

func TestAdapter_AugmentPrompt_NoResults(t *testing.T) {
	store := newTestStore(t)
	adapter := NewAdapter(store, NewIndexer(store, t.TempDir()))

	out, err := adapter.AugmentPrompt(context.Background(), "brand new title", "never seen description")
	if err != nil {
		t.Fatalf("AugmentPrompt error = %v", err)
	}
	if out != "" {
		t.Errorf("expected empty augment for empty store, got %q", out)
	}
}

func TestAdapter_AugmentPrompt_WithMatch(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	adapter := NewAdapter(store, NewIndexer(store, t.TempDir()))

	// Store a spec, then query with the exact same text so cosine sim is high
	// enough to clear the 0.70 MinScore threshold (HashEmbedder is deterministic).
	content := "implement authentication using oauth2 with refresh tokens and pkce flow"
	if err := store.Store(ctx, &Document{
		ID: "spec1", TaskID: "task-7", Type: TypeSpecification, Content: content,
	}); err != nil {
		t.Fatal(err)
	}

	out, err := adapter.AugmentPrompt(ctx, content, "")
	if err != nil {
		t.Fatalf("AugmentPrompt error = %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty augment block for matching content")
	}
	if !strings.Contains(out, "Relevant Context") || !strings.Contains(out, "task-7") {
		t.Errorf("augment block missing expected sections: %q", out)
	}
}

func TestAdapter_AugmentPrompt_LongContentTruncated(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	adapter := NewAdapter(store, NewIndexer(store, t.TempDir()))

	long := strings.Repeat("token ", 200) // > 300 chars
	if err := store.Store(ctx, &Document{ID: "long", TaskID: "tL", Type: TypeSpecification, Content: long}); err != nil {
		t.Fatal(err)
	}
	out, err := adapter.AugmentPrompt(ctx, long, "")
	if err != nil {
		t.Fatal(err)
	}
	if out != "" && !strings.Contains(out, "...") {
		t.Errorf("expected truncation ellipsis in long preview: %q", out)
	}
}

func TestAdapter_SearchSimilarTasks(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	adapter := NewAdapter(store, NewIndexer(store, t.TempDir()))

	content := "fix the race condition in the socket reconnect loop with a mutex guard"
	if err := store.Store(ctx, &Document{ID: "s1", TaskID: "t9", Type: TypeSolution, Content: content}); err != nil {
		t.Fatal(err)
	}
	out, err := adapter.SearchSimilarTasks(ctx, content, 3)
	if err != nil {
		t.Fatalf("SearchSimilarTasks error = %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected at least one formatted result")
	}
	if !strings.Contains(out[0], "t9") {
		t.Errorf("formatted result missing task ID: %q", out[0])
	}
}

func TestAdapter_LearnFromCorrection(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	adapter := NewAdapter(store, NewIndexer(store, t.TempDir()))

	if err := adapter.LearnFromCorrection(ctx, "task-c", "wrong approach", "use channels instead"); err != nil {
		t.Fatalf("LearnFromCorrection error = %v", err)
	}
	docs := store.GetDocumentsForTask(ctx, "task-c")
	if len(docs) != 1 {
		t.Fatalf("expected 1 solution doc, got %d", len(docs))
	}
	if docs[0].Type != TypeSolution {
		t.Errorf("doc type = %q, want solution", docs[0].Type)
	}
	if !strings.Contains(docs[0].Content, "use channels") {
		t.Errorf("solution content = %q", docs[0].Content)
	}
}

func TestAdapter_ClearAndStats(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	adapter := NewAdapter(store, NewIndexer(store, t.TempDir()))

	_ = adapter.LearnFromCorrection(ctx, "t", "p", "s")
	if adapter.Stats().TotalDocuments == 0 {
		t.Fatal("expected docs before clear")
	}
	if err := adapter.Clear(ctx); err != nil {
		t.Fatalf("Clear error = %v", err)
	}
	if adapter.Stats().TotalDocuments != 0 {
		t.Errorf("after Clear total = %d", adapter.Stats().TotalDocuments)
	}
}

// --- Indexer tests ---

func TestIndexer_IndexTask_SpecsAndSessions(t *testing.T) {
	store := newTestStore(t)
	projectDir := t.TempDir()
	indexer := NewIndexer(store, projectDir)
	ctx := context.Background()

	// Seed a specification file.
	specDir := filepath.Join(projectDir, ".kvelmo", "specifications")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	specContent := "This specification describes the new authentication subsystem in detail " +
		"with multiple distinct requirements and acceptance criteria spanning many lines."
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Seed a session log named with the task ID prefix.
	sessDir := filepath.Join(projectDir, ".kvelmo", "sessions")
	if err := os.MkdirAll(sessDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sessContent := "ERROR panic FATAL stack trace during the migration rollback regression " +
		"this session encountered a deadlock and a security vulnerability in the schema"
	if err := os.WriteFile(filepath.Join(sessDir, "task-1-session.log"), []byte(sessContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := indexer.IndexTask(ctx, "task-1", "Auth", "Build auth", "", ""); err != nil {
		t.Fatalf("IndexTask error = %v", err)
	}

	docs := store.GetDocumentsForTask(ctx, "task-1")
	var sawSpec, sawSession bool
	for _, d := range docs {
		switch d.Type {
		case TypeSpecification:
			sawSpec = true
		case TypeSession:
			sawSession = true
		case TypeCodeChange, TypeDecision, TypeSolution:
			// Not asserted by this test.
		default:
			// Ignore any other document types.
		}
	}
	if !sawSpec {
		t.Error("expected a specification document to be indexed")
	}
	if !sawSession {
		t.Error("expected a significant session document to be indexed")
	}
}

func TestIndexer_IndexTask_MissingDirsNoError(t *testing.T) {
	store := newTestStore(t)
	indexer := NewIndexer(store, t.TempDir()) // empty project dir
	if err := indexer.IndexTask(context.Background(), "task-x", "T", "D", "", ""); err != nil {
		t.Fatalf("IndexTask with no artefacts should not error: %v", err)
	}
}

func TestIndexer_IndexCodeChange_WithGitDiff(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	store := newTestStore(t)
	ctx := context.Background()

	// Build a tiny git repo with two branches that differ.
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "base")
	run("checkout", "-q", "-b", "feature")
	change := "the migration introduces a breaking schema change with rollback and deadlock handling " +
		"plus extensive new logic across many distinct lines to clear the significance gate"
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte(change+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "feature change")

	indexer := NewIndexer(store, repo)
	if err := indexer.IndexTask(ctx, "task-diff", "Feat", change, "feature", "main"); err != nil {
		t.Fatalf("IndexTask error = %v", err)
	}

	docs := store.GetDocumentsForTask(ctx, "task-diff")
	var sawCode bool
	for _, d := range docs {
		if d.Type == TypeCodeChange {
			sawCode = true
		}
	}
	if !sawCode {
		t.Error("expected a code_change document from the git diff")
	}
}

func TestIndexer_StoreIfSignificant_AlwaysStoresSpec(t *testing.T) {
	store := newTestStore(t)
	indexer := NewIndexer(store, t.TempDir())
	ctx := context.Background()

	// Specifications bypass the significance gate even when trivial.
	doc := &Document{ID: "tiny-spec", Type: TypeSpecification, Content: "x"}
	if err := indexer.storeIfSignificant(ctx, doc, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, "tiny-spec"); err != nil {
		t.Error("trivial specification should still be stored")
	}
}

func TestIndexer_StoreIfSignificant_GatesTrivialSession(t *testing.T) {
	store := newTestStore(t)
	indexer := NewIndexer(store, t.TempDir())
	ctx := context.Background()

	// A trivial session log should be dropped by the significance gate.
	doc := &Document{ID: "tiny-sess", Type: TypeSession, Content: "ok"}
	if err := indexer.storeIfSignificant(ctx, doc, "a totally different task spec about networking"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, "tiny-sess"); err == nil {
		t.Error("trivial session should have been gated out")
	}
}

func TestIndexer_StatsAndSearchSimilar(t *testing.T) {
	store := newTestStore(t)
	indexer := NewIndexer(store, t.TempDir())
	ctx := context.Background()

	content := "implement a caching layer with an LRU eviction policy and TTL expiry semantics"
	if err := store.Store(ctx, &Document{ID: "c1", Type: TypeSpecification, Content: content}); err != nil {
		t.Fatal(err)
	}
	if indexer.Stats().TotalDocuments != 1 {
		t.Errorf("Stats total = %d, want 1", indexer.Stats().TotalDocuments)
	}
	if indexer.Store() == nil {
		t.Error("Indexer.Store() returned nil")
	}
	res, err := indexer.SearchSimilar(ctx, content, 5)
	if err != nil {
		t.Fatalf("SearchSimilar error = %v", err)
	}
	if len(res) == 0 {
		t.Error("SearchSimilar returned no results for matching content")
	}
}

func TestGitDiff_NonexistentRefsReturnsEmpty(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	// A directory that isn't a git repo -> diff command fails -> empty string.
	got := gitDiff(context.Background(), t.TempDir(), "", "nope")
	if got != "" {
		t.Errorf("gitDiff on non-repo = %q, want empty", got)
	}
}

func TestUniqueSuffix_Monotonic(t *testing.T) {
	a := uniqueSuffix()
	time.Sleep(time.Millisecond)
	b := uniqueSuffix()
	if a == "" || b == "" {
		t.Fatal("uniqueSuffix returned empty")
	}
	if a == b {
		t.Error("expected distinct suffixes across time")
	}
}
