package provider

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStatusConstants(t *testing.T) {
	statuses := []Status{
		StatusOpen,
		StatusInProgress,
		StatusReview,
		StatusDone,
		StatusClosed,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("Status constant is empty")
		}
	}
}

func TestPriorityString(t *testing.T) {
	tests := []struct {
		want     string
		priority Priority
	}{
		{"low", PriorityLow},
		{"normal", PriorityNormal},
		{"high", PriorityHigh},
		{"critical", PriorityCritical},
		{"normal", Priority(99)}, // unknown defaults to normal
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.priority.String()
			if got != tt.want {
				t.Errorf("Priority.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCapabilityConstants(t *testing.T) {
	caps := []Capability{
		CapRead,
		CapList,
		CapDownloadAttachment,
		CapFetchComments,
		CapComment,
		CapUpdateStatus,
		CapManageLabels,
	}

	for _, c := range caps {
		if c == "" {
			t.Error("Capability constant is empty")
		}
	}
}

func TestCapabilitySetHas(t *testing.T) {
	cs := CapabilitySet{
		CapRead:    true,
		CapComment: true,
	}

	if !cs.Has(CapRead) {
		t.Error("CapabilitySet.Has(CapRead) = false, want true")
	}
	if !cs.Has(CapComment) {
		t.Error("CapabilitySet.Has(CapComment) = false, want true")
	}
	if cs.Has(CapList) {
		t.Error("CapabilitySet.Has(CapList) = true, want false")
	}
}

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()

	if cfg.options == nil {
		t.Error("Config.options is nil")
	}
}

func TestConfigSetAndGet(t *testing.T) {
	cfg := NewConfig()
	cfg = cfg.Set("key1", "value1")
	cfg = cfg.Set("key2", 42)

	if cfg.Get("key1") != "value1" {
		t.Errorf("Config.Get(key1) = %v, want %q", cfg.Get("key1"), "value1")
	}
	if cfg.Get("key2") != 42 {
		t.Errorf("Config.Get(key2) = %v, want 42", cfg.Get("key2"))
	}
	if cfg.Get("nonexistent") != nil {
		t.Errorf("Config.Get(nonexistent) = %v, want nil", cfg.Get("nonexistent"))
	}
}

func TestConfigGetString(t *testing.T) {
	cfg := NewConfig()
	cfg = cfg.Set("str", "hello")
	cfg = cfg.Set("num", 42)

	if cfg.GetString("str") != "hello" {
		t.Errorf("Config.GetString(str) = %q, want %q", cfg.GetString("str"), "hello")
	}
	if cfg.GetString("num") != "" {
		t.Errorf("Config.GetString(num) = %q, want empty (not a string)", cfg.GetString("num"))
	}
	if cfg.GetString("nonexistent") != "" {
		t.Errorf("Config.GetString(nonexistent) = %q, want empty", cfg.GetString("nonexistent"))
	}
}

func TestWorkUnitStruct(t *testing.T) {
	now := time.Now()
	wu := WorkUnit{
		ID:          "123",
		ExternalID:  "ext-123",
		Provider:    "test",
		Title:       "Test Task",
		Description: "Test description",
		Status:      StatusOpen,
		Priority:    PriorityHigh,
		Labels:      []string{"bug", "urgent"},
		Assignees: []Person{
			{ID: "1", Name: "John", Email: "john@example.com"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if wu.ID != "123" {
		t.Errorf("WorkUnit.ID = %q, want %q", wu.ID, "123")
	}
	if wu.Status != StatusOpen {
		t.Errorf("WorkUnit.Status = %q, want %q", wu.Status, StatusOpen)
	}
	if wu.Priority != PriorityHigh {
		t.Errorf("WorkUnit.Priority = %d, want %d", wu.Priority, PriorityHigh)
	}
	if len(wu.Labels) != 2 {
		t.Errorf("WorkUnit.Labels length = %d, want 2", len(wu.Labels))
	}
}

func TestPersonStruct(t *testing.T) {
	p := Person{
		ID:    "user1",
		Name:  "John Doe",
		Email: "john@example.com",
	}

	if p.ID != "user1" {
		t.Errorf("Person.ID = %q, want %q", p.ID, "user1")
	}
	if p.Name != "John Doe" {
		t.Errorf("Person.Name = %q, want %q", p.Name, "John Doe")
	}
}

func TestCommentStruct(t *testing.T) {
	now := time.Now()
	c := Comment{
		ID:        "comment1",
		Author:    Person{ID: "user1", Name: "John"},
		Body:      "This is a comment",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if c.ID != "comment1" {
		t.Errorf("Comment.ID = %q, want %q", c.ID, "comment1")
	}
	if c.Body != "This is a comment" {
		t.Errorf("Comment.Body = %q, want %q", c.Body, "This is a comment")
	}
}

func TestAttachmentStruct(t *testing.T) {
	a := Attachment{
		ID:          "att1",
		Name:        "file.pdf",
		URL:         "https://example.com/file.pdf",
		ContentType: "application/pdf",
		Size:        1024,
	}

	if a.ID != "att1" {
		t.Errorf("Attachment.ID = %q, want %q", a.ID, "att1")
	}
	if a.Size != 1024 {
		t.Errorf("Attachment.Size = %d, want 1024", a.Size)
	}
}

func TestSourceInfoStruct(t *testing.T) {
	now := time.Now()
	si := SourceInfo{
		Type:      "file",
		Reference: "task.md",
		SyncedAt:  now,
	}

	if si.Type != "file" {
		t.Errorf("SourceInfo.Type = %q, want %q", si.Type, "file")
	}
	if si.Reference != "task.md" {
		t.Errorf("SourceInfo.Reference = %q, want %q", si.Reference, "task.md")
	}
}

func TestListOptionsStruct(t *testing.T) {
	opts := ListOptions{
		Status:   StatusOpen,
		Labels:   []string{"bug"},
		Limit:    10,
		Offset:   0,
		OrderBy:  "created_at",
		OrderDir: "desc",
	}

	if opts.Status != StatusOpen {
		t.Errorf("ListOptions.Status = %q, want %q", opts.Status, StatusOpen)
	}
	if opts.Limit != 10 {
		t.Errorf("ListOptions.Limit = %d, want 10", opts.Limit)
	}
}

func TestSnapshotStruct(t *testing.T) {
	s := Snapshot{
		Type:    "directory",
		Ref:     "tasks/",
		Content: "",
		Files: []SnapshotFile{
			{Path: "task1.md", Content: "Task 1"},
			{Path: "task2.md", Content: "Task 2"},
		},
	}

	if s.Type != "directory" {
		t.Errorf("Snapshot.Type = %q, want %q", s.Type, "directory")
	}
	if len(s.Files) != 2 {
		t.Errorf("Snapshot.Files length = %d, want 2", len(s.Files))
	}
}

func TestProviderInfoStruct(t *testing.T) {
	info := ProviderInfo{
		Name:        "test",
		Description: "Test provider",
		Schemes:     []string{"test://", "t://"},
		Capabilities: CapabilitySet{
			CapRead: true,
			CapList: true,
		},
		Priority: 10,
	}

	if info.Name != "test" {
		t.Errorf("ProviderInfo.Name = %q, want %q", info.Name, "test")
	}
	if len(info.Schemes) != 2 {
		t.Errorf("ProviderInfo.Schemes length = %d, want 2", len(info.Schemes))
	}
	if info.Priority != 10 {
		t.Errorf("ProviderInfo.Priority = %d, want 10", info.Priority)
	}
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()

	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if r.providers == nil {
		t.Error("providers map not initialized")
	}
	if r.schemes == nil {
		t.Error("schemes map not initialized")
	}
}

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()

	info := ProviderInfo{
		Name:    "test",
		Schemes: []string{"test://"},
	}
	factory := func(ctx context.Context, cfg Config) (any, error) {
		return nil, errors.New("test provider not implemented")
	}

	err := r.Register(info, factory)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Verify registration
	gotInfo, gotFactory, ok := r.Get("test")
	if !ok {
		t.Fatal("Get returned false for registered provider")
	}
	if gotInfo.Name != "test" {
		t.Errorf("got info.Name = %q, want %q", gotInfo.Name, "test")
	}
	if gotFactory == nil {
		t.Error("got factory is nil")
	}
}

func TestRegistryRegisterDuplicate(t *testing.T) {
	r := NewRegistry()

	info := ProviderInfo{Name: "test"}
	factory := func(ctx context.Context, cfg Config) (any, error) {
		return nil, errors.New("test provider not implemented")
	}

	if err := r.Register(info, factory); err != nil {
		t.Fatalf("Register first: %v", err)
	}
	err := r.Register(info, factory)

	if err == nil {
		t.Error("Register should fail for duplicate provider")
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()

	info := ProviderInfo{Name: "test"}
	factory := func(ctx context.Context, cfg Config) (any, error) {
		return nil, errors.New("test provider not implemented")
	}
	if err := r.Register(info, factory); err != nil {
		t.Fatalf("Register(test): %v", err)
	}

	gotInfo, _, ok := r.Get("test")
	if !ok {
		t.Fatal("Get returned false")
	}
	if gotInfo.Name != "test" {
		t.Errorf("got Name = %q, want %q", gotInfo.Name, "test")
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	r := NewRegistry()

	_, _, ok := r.Get("nonexistent")
	if ok {
		t.Error("Get should return false for nonexistent provider")
	}
}

func TestRegistryGetByScheme(t *testing.T) {
	r := NewRegistry()

	info := ProviderInfo{
		Name:    "test",
		Schemes: []string{"test://", "t://"},
	}
	factory := func(ctx context.Context, cfg Config) (any, error) { return nil, nil } //nolint:nilnil // test mock
	if err := r.Register(info, factory); err != nil {
		t.Fatalf("Register(test): %v", err)
	}

	gotInfo, _, ok := r.GetByScheme("test://")
	if !ok {
		t.Fatal("GetByScheme returned false")
	}
	if gotInfo.Name != "test" {
		t.Errorf("got Name = %q, want %q", gotInfo.Name, "test")
	}

	// Test alternate scheme
	gotInfo, _, ok = r.GetByScheme("t://")
	if !ok {
		t.Fatal("GetByScheme returned false for alternate scheme")
	}
	if gotInfo.Name != "test" {
		t.Errorf("got Name = %q, want %q", gotInfo.Name, "test")
	}
}

func TestRegistryGetBySchemeNotFound(t *testing.T) {
	r := NewRegistry()

	_, _, ok := r.GetByScheme("unknown://")
	if ok {
		t.Error("GetByScheme should return false for unknown scheme")
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()

	factory := func(ctx context.Context, cfg Config) (any, error) { return nil, nil } //nolint:nilnil // test mock

	if err := r.Register(ProviderInfo{Name: "low", Priority: 1}, factory); err != nil {
		t.Fatalf("Register(low): %v", err)
	}
	if err := r.Register(ProviderInfo{Name: "high", Priority: 10}, factory); err != nil {
		t.Fatalf("Register(high): %v", err)
	}
	if err := r.Register(ProviderInfo{Name: "medium", Priority: 5}, factory); err != nil {
		t.Fatalf("Register(medium): %v", err)
	}

	list := r.List()
	if len(list) != 3 {
		t.Errorf("List returned %d providers, want 3", len(list))
	}

	// Should be sorted by priority descending
	if list[0].Name != "high" {
		t.Errorf("list[0].Name = %q, want %q (highest priority)", list[0].Name, "high")
	}
	if list[1].Name != "medium" {
		t.Errorf("list[1].Name = %q, want %q", list[1].Name, "medium")
	}
	if list[2].Name != "low" {
		t.Errorf("list[2].Name = %q, want %q (lowest priority)", list[2].Name, "low")
	}
}

func TestRegistryCreate(t *testing.T) {
	r := NewRegistry()

	created := false
	factory := func(ctx context.Context, cfg Config) (any, error) {
		created = true

		return "instance", nil
	}

	if err := r.Register(ProviderInfo{Name: "test"}, factory); err != nil {
		t.Fatalf("Register(test): %v", err)
	}

	ctx := context.Background()
	instance, err := r.Create(ctx, "test", NewConfig())
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if !created {
		t.Error("factory was not called")
	}
	if instance != "instance" {
		t.Errorf("instance = %v, want %q", instance, "instance")
	}
}

func TestRegistryCreateNotFound(t *testing.T) {
	r := NewRegistry()

	ctx := context.Background()
	_, err := r.Create(ctx, "nonexistent", NewConfig())
	if err == nil {
		t.Error("Create should fail for nonexistent provider")
	}
}

// mockIdentifier implements Identifier interface for testing.
type mockIdentifier struct {
	parseErr    error
	parseResult string
	matchResult bool
}

func (m *mockIdentifier) Match(input string) bool {
	return m.matchResult
}

func (m *mockIdentifier) Parse(input string) (string, error) {
	return m.parseResult, m.parseErr
}

// mockReader implements Reader interface for testing InferCapabilities.
type mockReader struct{}

func (m *mockReader) Fetch(ctx context.Context, id string) (*WorkUnit, error) {
	return nil, nil //nolint:nilnil // test mock
}

// mockLister implements Lister interface.
type mockLister struct{}

func (m *mockLister) List(ctx context.Context, opts ListOptions) ([]*WorkUnit, error) {
	return nil, nil
}

// mockFullProvider implements multiple interfaces.
type mockFullProvider struct {
	mockReader
	mockLister
}

func TestInferCapabilities(t *testing.T) {
	// Test with Reader only
	reader := &mockReader{}
	caps := InferCapabilities(reader)
	if !caps.Has(CapRead) {
		t.Error("InferCapabilities should detect CapRead")
	}
	if caps.Has(CapList) {
		t.Error("InferCapabilities should not detect CapList for Reader only")
	}

	// Test with full provider
	full := &mockFullProvider{}
	caps = InferCapabilities(full)
	if !caps.Has(CapRead) {
		t.Error("InferCapabilities should detect CapRead")
	}
	if !caps.Has(CapList) {
		t.Error("InferCapabilities should detect CapList")
	}
}

func TestParseScheme(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantScheme string
		wantIdent  string
	}{
		{
			name:       "file scheme",
			input:      "file:task.md",
			wantScheme: "file",
			wantIdent:  "task.md",
		},
		{
			name:       "github scheme",
			input:      "github:owner/repo#123",
			wantScheme: "github",
			wantIdent:  "owner/repo#123",
		},
		{
			name:       "dir scheme",
			input:      "dir:tasks/",
			wantScheme: "dir",
			wantIdent:  "tasks/",
		},
		{
			name:       "no scheme",
			input:      "task.md",
			wantScheme: "",
			wantIdent:  "task.md",
		},
		{
			name:       "windows path C drive",
			input:      "C:\\Users\\test\\task.md",
			wantScheme: "",
			wantIdent:  "C:\\Users\\test\\task.md",
		},
		{
			name:       "windows path C drive forward slash",
			input:      "C:/Users/test/task.md",
			wantScheme: "",
			wantIdent:  "C:/Users/test/task.md",
		},
		{
			name:       "empty input",
			input:      "",
			wantScheme: "",
			wantIdent:  "",
		},
		{
			name:       "colon at end",
			input:      "file:",
			wantScheme: "file",
			wantIdent:  "",
		},
		{
			name:       "multiple colons",
			input:      "http:example.com:8080",
			wantScheme: "http",
			wantIdent:  "example.com:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme, ident := parseScheme(tt.input)
			if scheme != tt.wantScheme {
				t.Errorf("parseScheme(%q) scheme = %q, want %q", tt.input, scheme, tt.wantScheme)
			}
			if ident != tt.wantIdent {
				t.Errorf("parseScheme(%q) ident = %q, want %q", tt.input, ident, tt.wantIdent)
			}
		})
	}
}

func TestRegistryListSchemes(t *testing.T) {
	r := NewRegistry()

	// Initially empty
	schemes := r.listSchemes()
	if len(schemes) != 0 {
		t.Errorf("listSchemes() on empty registry = %v, want empty", schemes)
	}

	// Register providers with schemes
	factory := func(ctx context.Context, cfg Config) (any, error) {
		return &mockIdentifier{}, nil
	}

	if err := r.Register(ProviderInfo{Name: "file", Schemes: []string{"file", "f"}}, factory); err != nil {
		t.Fatalf("Register(file): %v", err)
	}
	if err := r.Register(ProviderInfo{Name: "github", Schemes: []string{"github", "gh"}}, factory); err != nil {
		t.Fatalf("Register(github): %v", err)
	}

	schemes = r.listSchemes()
	if len(schemes) != 4 {
		t.Errorf("listSchemes() = %v, want 4 schemes", schemes)
	}

	// Should be sorted alphabetically
	expected := []string{"f", "file", "gh", "github"}
	for i, s := range schemes {
		if s != expected[i] {
			t.Errorf("listSchemes()[%d] = %q, want %q", i, s, expected[i])
		}
	}
}

func TestRegistryResolve(t *testing.T) {
	r := NewRegistry()

	// Register a file provider
	factory := func(ctx context.Context, cfg Config) (any, error) {
		return &mockIdentifier{matchResult: true, parseResult: "resolved-id"}, nil
	}
	if err := r.Register(ProviderInfo{Name: "file", Schemes: []string{"file"}}, factory); err != nil {
		t.Fatalf("Register(file): %v", err)
	}

	ctx := context.Background()

	// Test with explicit scheme
	instance, id, err := r.Resolve(ctx, "file:task.md", NewConfig(), ResolveOptions{})
	if err != nil {
		t.Fatalf("Resolve with scheme failed: %v", err)
	}
	if id != "resolved-id" {
		t.Errorf("id = %q, want %q", id, "resolved-id")
	}
	if instance == nil {
		t.Error("instance is nil")
	}

	// Test with default provider
	_, id, err = r.Resolve(ctx, "task.md", NewConfig(), ResolveOptions{DefaultProvider: "file"})
	if err != nil {
		t.Fatalf("Resolve with default provider failed: %v", err)
	}
	if id != "resolved-id" {
		t.Errorf("id = %q, want %q", id, "resolved-id")
	}

	// Test without scheme and without default - should error
	_, _, err = r.Resolve(ctx, "task.md", NewConfig(), ResolveOptions{})
	if err == nil {
		t.Error("Resolve without scheme or default should fail")
	}
}

func TestRegistryResolveUnknownScheme(t *testing.T) {
	r := NewRegistry()

	ctx := context.Background()
	_, _, err := r.Resolve(ctx, "unknown:task.md", NewConfig(), ResolveOptions{})
	if err == nil {
		t.Error("Resolve with unknown scheme should fail")
	}
}

func TestRegistryNoSchemeError(t *testing.T) {
	r := NewRegistry()

	// Register a provider to have schemes in error message
	factory := func(ctx context.Context, cfg Config) (any, error) {
		return &mockIdentifier{}, nil
	}
	if err := r.Register(ProviderInfo{Name: "file", Schemes: []string{"file"}}, factory); err != nil {
		t.Fatalf("Register(file): %v", err)
	}

	err := r.noSchemeError("task.md")
	if err == nil {
		t.Fatal("noSchemeError should return an error")
	}

	errStr := err.Error()
	if !contains(errStr, "task.md") {
		t.Error("error should contain the input")
	}
	if !contains(errStr, "file") {
		t.Error("error should list available schemes")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}

func TestConfigGetBool(t *testing.T) {
	tests := []struct {
		value    any
		name     string
		key      string
		expected bool
	}{
		{
			name:     "true bool",
			key:      "enabled",
			value:    true,
			expected: true,
		},
		{
			name:     "false bool",
			key:      "disabled",
			value:    false,
			expected: false,
		},
		{
			name:     "non-bool value",
			key:      "str",
			value:    "hello",
			expected: false,
		},
		{
			name:     "nil value",
			key:      "missing",
			value:    nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			if tt.value != nil {
				cfg = cfg.Set(tt.key, tt.value)
			}
			result := cfg.GetBool(tt.key)
			if result != tt.expected {
				t.Errorf("GetBool(%q) = %v, want %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestInferCapabilitiesMore(t *testing.T) {
	// Test with nil
	caps := InferCapabilities(nil)
	if len(caps) != 0 {
		t.Error("InferCapabilities(nil) should return empty set")
	}

	// Test with non-interface type
	caps = InferCapabilities("string")
	if len(caps) != 0 {
		t.Error("InferCapabilities(string) should return empty set")
	}
}
