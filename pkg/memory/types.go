// Package memory provides a semantic memory system for kvelmo.
// It stores and retrieves past task context to augment agent prompts.
package memory

import (
	"context"
	"time"
)

// DocumentType represents the type of content stored in memory.
type DocumentType string

const (
	// TypeSpecification stores planning/specification content.
	TypeSpecification DocumentType = "specification"
	// TypeCodeChange stores git diff / code change content.
	TypeCodeChange DocumentType = "code_change"
	// TypeSession stores agent session transcripts.
	TypeSession DocumentType = "session"
	// TypeDecision stores architectural or design decisions.
	TypeDecision DocumentType = "decision"
	// TypeSolution stores corrections and learned solutions.
	TypeSolution DocumentType = "solution"
)

// Document is a single entry in the vector store.
type Document struct {
	ID        string                 `json:"id"`
	TaskID    string                 `json:"task_id"`
	Type      DocumentType           `json:"type"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata"`
	Embedding []float32              `json:"embedding"`
	CreatedAt time.Time              `json:"created_at"`
	Tags      []string               `json:"tags"`
	// AccessCount tracks how many times this document has been returned in search results.
	// Kept for backward-compatible JSON deserialization; new code uses AccessTimes.
	AccessCount int `json:"access_count"`
	// LastAccessed records when this document was last returned in search results.
	LastAccessed time.Time `json:"last_accessed,omitempty"`
	// AccessTimes stores individual access timestamps for ACT-R activation scoring.
	// Capped at maxAccessTimes entries to bound memory usage.
	AccessTimes []time.Time `json:"access_times,omitempty"`
	// Importance is a user/LLM-assigned score (0.0-1.0) indicating how significant
	// this document is. Higher importance boosts search ranking.
	Importance float64 `json:"importance,omitempty"`
	// Category labels this document for named-block retrieval (e.g. "requirements",
	// "decisions", "feedback", "learnings").
	Category string `json:"category,omitempty"`
}

// SearchResult pairs a document with its similarity score.
type SearchResult struct {
	Document *Document
	Score    float32
}

// SearchOptions controls search behaviour.
type SearchOptions struct {
	// Limit caps the number of results returned (0 = no cap).
	Limit int
	// MinScore is the minimum cosine similarity (0–1) to include.
	MinScore float32
	// DocumentTypes filters by type; empty means all types.
	DocumentTypes []DocumentType
	// TimeRange filters by document creation time.
	TimeRange *TimeRange
	// MetadataFilters matches exact metadata key/value pairs.
	MetadataFilters map[string]interface{}
}

// TimeRange is an inclusive time window for search filtering.
type TimeRange struct {
	From time.Time
	To   time.Time
}

// Stats holds aggregate statistics about the vector store.
type Stats struct {
	TotalDocuments int            `json:"total_documents"`
	ByType         map[string]int `json:"by_type"`
	// Embedder identifies which embedding backend is active:
	// "onnx", "tfidf", or "hash".
	Embedder string `json:"embedder,omitempty"`
}

// Embedder generates vector embeddings for text.
type Embedder interface {
	// Embed returns an embedding vector for the given text.
	Embed(ctx context.Context, text string) ([]float32, error)
	// EmbedBatch returns embeddings for multiple texts.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	// Dimension returns the length of each embedding vector.
	Dimension() int
	// Name returns the short identifier for this embedder: "onnx", "tfidf", or "hash".
	Name() string
}
