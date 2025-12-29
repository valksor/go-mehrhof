package notion

import (
	"testing"

	"github.com/valksor/go-mehrhof/internal/provider"
)

func TestParseReference(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "valid page ID with notion scheme",
			input:   "notion:a1b2c3d4e5f678901234567890abcdef",
			want:    "a1b2c3d4e5f678901234567890abcdef",
			wantErr: false,
		},
		{
			name:    "valid page ID with short scheme",
			input:   "nt:a1b2c3d4e5f678901234567890abcdef",
			want:    "a1b2c3d4e5f678901234567890abcdef",
			wantErr: false,
		},
		{
			name:    "valid Notion URL",
			input:   "https://www.notion.so/Page-Title-a1b2c3d4e5f678901234567890abcdef",
			want:    "a1b2c3d4e5f678901234567890abcdef",
			wantErr: false,
		},
		{
			name:    "Notion URL with username",
			input:   "https://www.notion.so/username/Page-Title-a1b2c3d4e5f678901234567890abcdef",
			want:    "a1b2c3d4e5f678901234567890abcdef",
			wantErr: false,
		},
		{
			name:    "UUID with dashes",
			input:   "a1b2c3d4-e5f6-7890-1234-567890abcdef",
			want:    "a1b2c3d4e5f678901234567890abcdef",
			wantErr: false,
		},
		{
			name:    "UUID with dashes and scheme",
			input:   "notion:a1b2c3d4-e5f6-7890-1234-567890abcdef",
			want:    "a1b2c3d4e5f678901234567890abcdef",
			wantErr: false,
		},
		{
			name:    "bare 32-char page ID",
			input:   "a1b2c3d4e5f678901234567890abcdef",
			want:    "a1b2c3d4e5f678901234567890abcdef",
			wantErr: false,
		},
		{
			name:    "empty input",
			input:   "",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "notion:invalid",
			want:    "",
			wantErr: true,
		},
		{
			name:    "too short",
			input:   "notion:abc123",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseReference(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseReference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && ref.PageID != tt.want {
				t.Errorf("ParseReference() PageID = %v, want %v", ref.PageID, tt.want)
			}
		})
	}
}

func TestExtractPageID(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "valid Notion URL",
			url:  "https://www.notion.so/Page-Title-a1b2c3d4e5f678901234567890abcdef",
			want: "a1b2c3d4e5f678901234567890abcdef",
		},
		{
			name: "Notion URL with query params",
			url:  "https://www.notion.so/Page-Title-a1b2c3d4e5f678901234567890abcdef?pvs=4",
			want: "a1b2c3d4e5f678901234567890abcdef",
		},
		{
			name: "Notion URL with username",
			url:  "https://www.notion.so/username/Page-Title-a1b2c3d4e5f678901234567890abcdef",
			want: "a1b2c3d4e5f678901234567890abcdef",
		},
		{
			name: "invalid URL",
			url:  "https://example.com/page",
			want: "",
		},
		{
			name: "empty string",
			url:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractPageID(tt.url); got != tt.want {
				t.Errorf("ExtractPageID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizePageID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{
			name: "32-char hex - unchanged",
			id:   "a1b2c3d4e5f678901234567890abcdef",
			want: "a1b2c3d4e5f678901234567890abcdef",
		},
		{
			name: "UUID with dashes - normalized",
			id:   "a1b2c3d4-e5f6-7890-1234-567890abcdef",
			want: "a1b2c3d4e5f678901234567890abcdef",
		},
		{
			name: "URL - extracted and normalized",
			id:   "https://www.notion.so/Page-a1b2c3d4e5f678901234567890abcdef",
			want: "a1b2c3d4e5f678901234567890abcdef",
		},
		{
			name: "mixed case UUID - lowercased",
			id:   "A1B2C3D4-E5F6-7890-1234-567890ABCDEF",
			want: "a1b2c3d4e5f678901234567890abcdef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizePageID(tt.id); got != tt.want {
				t.Errorf("NormalizePageID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMapNotionStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   provider.Status
	}{
		{
			name:   "Not Started -> open",
			status: "Not Started",
			want:   provider.StatusOpen,
		},
		{
			name:   "In Progress -> in_progress",
			status: "In Progress",
			want:   provider.StatusInProgress,
		},
		{
			name:   "In Review -> review",
			status: "In Review",
			want:   provider.StatusReview,
		},
		{
			name:   "Done -> done",
			status: "Done",
			want:   provider.StatusDone,
		},
		{
			name:   "Cancelled -> closed",
			status: "Cancelled",
			want:   provider.StatusClosed,
		},
		{
			name:   "unknown -> open",
			status: "Unknown Status",
			want:   provider.StatusOpen,
		},
		{
			name:   "case insensitive",
			status: "IN PROGRESS",
			want:   provider.StatusInProgress,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapNotionStatus(tt.status); got != tt.want {
				t.Errorf("mapNotionStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMapProviderStatusToNotion(t *testing.T) {
	tests := []struct {
		name   string
		status provider.Status
		want   string
	}{
		{
			name:   "open -> Not Started",
			status: provider.StatusOpen,
			want:   "Not Started",
		},
		{
			name:   "in_progress -> In Progress",
			status: provider.StatusInProgress,
			want:   "In Progress",
		},
		{
			name:   "review -> In Review",
			status: provider.StatusReview,
			want:   "In Review",
		},
		{
			name:   "done -> Done",
			status: provider.StatusDone,
			want:   "Done",
		},
		{
			name:   "closed -> Cancelled",
			status: provider.StatusClosed,
			want:   "Cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapProviderStatusToNotion(tt.status); got != tt.want {
				t.Errorf("mapProviderStatusToNotion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatch(t *testing.T) {
	p := &Provider{}

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "notion scheme - match",
			input: "notion:a1b2c3d4e5f678901234567890abcdef",
			want:  true,
		},
		{
			name:  "nt scheme - match",
			input: "nt:a1b2c3d4e5f678901234567890abcdef",
			want:  true,
		},
		{
			name:  "github scheme - no match",
			input: "github:123",
			want:  false,
		},
		{
			name:  "bare URL - no match",
			input: "https://www.notion.so/Page-a1b2c3d4e5f678901234567890abcdef",
			want:  false,
		},
		{
			name:  "empty - no match",
			input: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.Match(tt.input); got != tt.want {
				t.Errorf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInfo(t *testing.T) {
	info := Info()

	if info.Name != ProviderName {
		t.Errorf("Info().Name = %v, want %v", info.Name, ProviderName)
	}

	if len(info.Schemes) != 2 {
		t.Errorf("Info().Schemes length = %v, want 2", len(info.Schemes))
	}

	expectedSchemes := []string{"notion", "nt"}
	for i, scheme := range info.Schemes {
		if scheme != expectedSchemes[i] {
			t.Errorf("Info().Schemes[%d] = %v, want %v", i, scheme, expectedSchemes[i])
		}
	}

	// Check all declared capabilities are present
	expectedCaps := []provider.Capability{
		provider.CapRead, provider.CapList, provider.CapFetchComments, provider.CapComment,
		provider.CapUpdateStatus, provider.CapManageLabels, provider.CapCreateWorkUnit, provider.CapSnapshot,
	}
	for _, cap := range expectedCaps {
		if !info.Capabilities.Has(cap) {
			t.Errorf("Info().Capabilities missing %v", cap)
		}
	}
}

func TestBlocksToMarkdown(t *testing.T) {
	tests := []struct {
		name   string
		blocks []Block
		want   string
	}{
		{
			name:   "empty blocks",
			blocks: []Block{},
			want:   "",
		},
		{
			name: "paragraph block",
			blocks: []Block{
				{
					Type: "paragraph",
					Paragraph: &ParagraphBlock{
						Type: "paragraph",
						RichText: []RichText{
							{
								Type:      "text",
								PlainText: "Hello world",
							},
						},
					},
				},
			},
			want: "Hello world\n\n",
		},
		{
			name: "heading block",
			blocks: []Block{
				{
					Type: "heading_1",
					Heading1: &HeadingBlock{
						Type: "heading_1",
						RichText: []RichText{
							{
								Type:      "text",
								PlainText: "Title",
							},
						},
					},
				},
			},
			want: "# Title\n\n",
		},
		{
			name: "code block",
			blocks: []Block{
				{
					Type: "code",
					Code: &CodeBlock{
						Type:     "code",
						Language: "go",
						RichText: []RichText{
							{
								Type:      "text",
								PlainText: "fmt.Println(\"hello\")",
							},
						},
					},
				},
			},
			want: "```go\nfmt.Println(\"hello\")\n```\n\n",
		},
		{
			name: "divider block",
			blocks: []Block{
				{
					Type:    "divider",
					Divider: &DividerBlock{Type: "divider"},
				},
			},
			want: "---\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BlocksToMarkdown(tt.blocks)
			if got != tt.want {
				t.Errorf("BlocksToMarkdown() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractPlainText(t *testing.T) {
	tests := []struct {
		name string
		prop Property
		want string
	}{
		{
			name: "title property",
			prop: Property{
				Type: "title",
				Title: &TitleProp{
					Type: "title",
					Title: []RichText{
						{PlainText: "Task Title"},
					},
				},
			},
			want: "Task Title",
		},
		{
			name: "rich text property",
			prop: Property{
				Type: "rich_text",
				RichText: &RichTextProp{
					Type: "rich_text",
					RichText: []RichText{
						{PlainText: "Description text"},
					},
				},
			},
			want: "Description text",
		},
		{
			name: "select property",
			prop: Property{
				Type: "select",
				Select: &SelectProp{
					Name: "In Progress",
				},
			},
			want: "In Progress",
		},
		{
			name: "status property",
			prop: Property{
				Type: "status",
				Status: &StatusProp{
					Name: "Done",
				},
			},
			want: "Done",
		},
		{
			name: "empty property",
			prop: Property{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractPlainText(tt.prop); got != tt.want {
				t.Errorf("ExtractPlainText() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMakeTitleProperty(t *testing.T) {
	title := "Test Task"
	prop := MakeTitleProperty(title)

	if prop.Type != "title" {
		t.Errorf("MakeTitleProperty() Type = %v, want title", prop.Type)
	}

	if prop.Title == nil {
		t.Fatal("MakeTitleProperty() Title is nil")
	}

	if len(prop.Title.Title) != 1 {
		t.Fatalf("MakeTitleProperty() Title.Title length = %v, want 1", len(prop.Title.Title))
	}

	got := prop.Title.Title[0].PlainText
	if got != title {
		t.Errorf("MakeTitleProperty() PlainText = %v, want %v", got, title)
	}
}

func TestMakeStatusProperty(t *testing.T) {
	status := "In Progress"
	prop := MakeStatusProperty(status)

	if prop.Type != "status" {
		t.Errorf("MakeStatusProperty() Type = %v, want status", prop.Type)
	}

	if prop.Status == nil {
		t.Fatal("MakeStatusProperty() Status is nil")
	}

	if prop.Status.Name != status {
		t.Errorf("MakeStatusProperty() Name = %v, want %v", prop.Status.Name, status)
	}
}

func TestMakeMultiSelectProperty(t *testing.T) {
	labels := []string{"bug", "urgent"}
	prop := MakeMultiSelectProperty(labels)

	if prop.Type != "multi_select" {
		t.Errorf("MakeMultiSelectProperty() Type = %v, want multi_select", prop.Type)
	}

	if prop.MultiSelect == nil {
		t.Fatal("MakeMultiSelectProperty() MultiSelect is nil")
	}

	if len(prop.MultiSelect.Options) != len(labels) {
		t.Fatalf("MakeMultiSelectProperty() Options length = %v, want %v", len(prop.MultiSelect.Options), len(labels))
	}

	for i, opt := range prop.MultiSelect.Options {
		if opt.Name != labels[i] {
			t.Errorf("MakeMultiSelectProperty() Options[%v].Name = %v, want %v", i, opt.Name, labels[i])
		}
	}
}

func TestRefString(t *testing.T) {
	tests := []struct {
		name string
		ref  *Ref
		want string
	}{
		{
			name: "URL preferred",
			ref: &Ref{
				PageID: "a1b2c3d4e5f678901234567890abcdef",
				URL:    "https://www.notion.so/Page-a1b2c3d4e5f678901234567890abcdef",
			},
			want: "https://www.notion.so/Page-a1b2c3d4e5f678901234567890abcdef",
		},
		{
			name: "page ID when no URL",
			ref: &Ref{
				PageID: "a1b2c3d4e5f678901234567890abcdef",
			},
			want: "a1b2c3d4e5f678901234567890abcdef",
		},
		{
			name: "empty when both empty",
			ref:  &Ref{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.String(); got != tt.want {
				t.Errorf("Ref.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Benchmark tests
func BenchmarkParseReference(b *testing.B) {
	input := "notion:a1b2c3d4e5f678901234567890abcdef"
	for i := 0; i < b.N; i++ {
		_, _ = ParseReference(input)
	}
}

func BenchmarkExtractPageID(b *testing.B) {
	url := "https://www.notion.so/Page-Title-a1b2c3d4e5f678901234567890abcdef"
	for i := 0; i < b.N; i++ {
		ExtractPageID(url)
	}
}

func BenchmarkMapNotionStatus(b *testing.B) {
	status := "In Progress"
	for i := 0; i < b.N; i++ {
		mapNotionStatus(status)
	}
}

func BenchmarkBlocksToMarkdown(b *testing.B) {
	blocks := []Block{
		{
			Type: "paragraph",
			Paragraph: &ParagraphBlock{
				Type: "paragraph",
				RichText: []RichText{
					{
						Type:      "text",
						PlainText: "Hello world",
					},
				},
			},
		},
		{
			Type: "heading_1",
			Heading1: &HeadingBlock{
				Type: "heading_1",
				RichText: []RichText{
					{
						Type:      "text",
						PlainText: "Title",
					},
				},
			},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BlocksToMarkdown(blocks)
	}
}

func BenchmarkMakeTitleProperty(b *testing.B) {
	title := "Test Task Title"
	for i := 0; i < b.N; i++ {
		MakeTitleProperty(title)
	}
}

func BenchmarkMakeStatusProperty(b *testing.B) {
	status := "In Progress"
	for i := 0; i < b.N; i++ {
		MakeStatusProperty(status)
	}
}

func BenchmarkMakeMultiSelectProperty(b *testing.B) {
	labels := []string{"bug", "urgent", "enhancement"}
	for i := 0; i < b.N; i++ {
		MakeMultiSelectProperty(labels)
	}
}

func BenchmarkExtractPlainText(b *testing.B) {
	prop := Property{
		Type: "title",
		Title: &TitleProp{
			Type: "title",
			Title: []RichText{
				{PlainText: "Task Title"},
			},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractPlainText(prop)
	}
}

func BenchmarkNormalizePageID(b *testing.B) {
	id := "a1b2c3d4-e5f6-7890-1234-567890abcdef"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NormalizePageID(id)
	}
}

func BenchmarkMatch(b *testing.B) {
	p := &Provider{}
	input := "notion:a1b2c3d4e5f678901234567890abcdef"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Match(input)
	}
}
