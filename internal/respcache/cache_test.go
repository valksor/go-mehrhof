package respcache

import (
	"sync"
	"testing"
	"time"
)

func TestCache_ExactHit(t *testing.T) {
	c := New(100, time.Hour)
	c.Put("what is 2+2?", "4", "plan")

	got, ok := c.Get("what is 2+2?")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got != "4" {
		t.Errorf("got %q, want %q", got, "4")
	}
}

func TestCache_Miss(t *testing.T) {
	c := New(100, time.Hour)
	c.Put("what is 2+2?", "4", "plan")

	_, ok := c.Get("what is 3+3?")
	if ok {
		t.Fatal("expected cache miss for different prompt")
	}
}

func TestCache_TTLExpiration(t *testing.T) {
	c := New(100, 50*time.Millisecond)
	c.Put("prompt", "response", "implement")

	// Should hit immediately.
	if _, ok := c.Get("prompt"); !ok {
		t.Fatal("expected hit before TTL")
	}

	time.Sleep(60 * time.Millisecond)

	// Should miss after TTL.
	if _, ok := c.Get("prompt"); ok {
		t.Fatal("expected miss after TTL expiration")
	}

	// Entry should be cleaned up.
	if c.Len() != 0 {
		t.Errorf("expected 0 entries after expiration, got %d", c.Len())
	}
}

func TestCache_MaxEntries(t *testing.T) {
	c := New(3, time.Hour)

	c.Put("p1", "r1", "plan")
	c.Put("p2", "r2", "plan")
	c.Put("p3", "r3", "plan")

	if c.Len() != 3 {
		t.Fatalf("expected 3 entries, got %d", c.Len())
	}

	// Adding a 4th should evict the oldest (p1).
	c.Put("p4", "r4", "plan")

	if c.Len() != 3 {
		t.Errorf("expected 3 entries after eviction, got %d", c.Len())
	}

	// p1 should be evicted.
	if _, ok := c.Get("p1"); ok {
		t.Error("expected p1 to be evicted")
	}

	// p4 should be present.
	if got, ok := c.Get("p4"); !ok || got != "r4" {
		t.Error("expected p4 to be present")
	}
}

func TestCache_Clear(t *testing.T) {
	c := New(100, time.Hour)
	c.Put("p1", "r1", "plan")
	c.Put("p2", "r2", "implement")

	c.Clear()

	if c.Len() != 0 {
		t.Errorf("expected 0 entries after clear, got %d", c.Len())
	}

	// Verify stats are reset (check before any Get calls that would increment misses).
	stats := c.Stats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Errorf("expected zeroed stats after clear, got hits=%d misses=%d", stats.Hits, stats.Misses)
	}

	if _, ok := c.Get("p1"); ok {
		t.Error("expected miss after clear")
	}
}

func TestCache_Stats(t *testing.T) {
	tests := []struct {
		name        string
		puts        []string
		gets        []string
		wantHits    int
		wantMisses  int
		wantEntries int
	}{
		{
			name:        "all hits",
			puts:        []string{"a", "b"},
			gets:        []string{"a", "b"},
			wantHits:    2,
			wantMisses:  0,
			wantEntries: 2,
		},
		{
			name:        "all misses",
			puts:        []string{"a"},
			gets:        []string{"x", "y", "z"},
			wantHits:    0,
			wantMisses:  3,
			wantEntries: 1,
		},
		{
			name:        "mixed",
			puts:        []string{"a", "b"},
			gets:        []string{"a", "c", "b"},
			wantHits:    2,
			wantMisses:  1,
			wantEntries: 2,
		},
		{
			name:        "empty cache",
			puts:        nil,
			gets:        []string{"a"},
			wantHits:    0,
			wantMisses:  1,
			wantEntries: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(100, time.Hour)

			for _, p := range tt.puts {
				c.Put(p, "resp-"+p, "test")
			}
			for _, g := range tt.gets {
				c.Get(g)
			}

			stats := c.Stats()
			if stats.Hits != tt.wantHits {
				t.Errorf("hits: got %d, want %d", stats.Hits, tt.wantHits)
			}
			if stats.Misses != tt.wantMisses {
				t.Errorf("misses: got %d, want %d", stats.Misses, tt.wantMisses)
			}
			if stats.Entries != tt.wantEntries {
				t.Errorf("entries: got %d, want %d", stats.Entries, tt.wantEntries)
			}

			total := tt.wantHits + tt.wantMisses
			if total > 0 {
				wantRate := float64(tt.wantHits) / float64(total)
				if stats.HitRate != wantRate {
					t.Errorf("hit rate: got %f, want %f", stats.HitRate, wantRate)
				}
			}
		})
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := New(1000, time.Hour)
	var wg sync.WaitGroup

	// Writers.
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := range 20 {
				prompt := string(rune('A'+n%26)) + string(rune('0'+j%10))
				c.Put(prompt, "response", "plan")
			}
		}(i)
	}

	// Readers.
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := range 20 {
				prompt := string(rune('A'+n%26)) + string(rune('0'+j%10))
				c.Get(prompt)
			}
		}(i)
	}

	wg.Wait()

	// Verify no panic and stats are consistent.
	stats := c.Stats()
	if stats.Hits+stats.Misses == 0 {
		t.Error("expected some gets to have been recorded")
	}
}

func TestCache_UpdateExisting(t *testing.T) {
	c := New(100, time.Hour)
	c.Put("prompt", "old-response", "plan")
	c.Put("prompt", "new-response", "plan")

	got, ok := c.Get("prompt")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got != "new-response" {
		t.Errorf("got %q, want %q", got, "new-response")
	}

	// Should not have created a duplicate entry.
	if c.Len() != 1 {
		t.Errorf("expected 1 entry, got %d", c.Len())
	}
}

func TestCache_DefaultParameters(t *testing.T) {
	// Zero values should use defaults.
	c := New(0, 0)
	if c.maxEntries != 1000 {
		t.Errorf("default maxEntries: got %d, want 1000", c.maxEntries)
	}
	if c.ttl != 168*time.Hour {
		t.Errorf("default ttl: got %v, want %v", c.ttl, 168*time.Hour)
	}
}

func TestCache_TokensSaved(t *testing.T) {
	c := New(100, time.Hour)
	// Response of 400 chars ~ 100 tokens.
	response := string(make([]byte, 400))
	c.Put("prompt", response, "implement")

	c.Get("prompt")
	c.Get("prompt")

	stats := c.Stats()
	// 2 hits * 400/4 = 200 tokens saved.
	if stats.TokensSaved != 200 {
		t.Errorf("tokens saved: got %d, want 200", stats.TokensSaved)
	}
}

func TestFormatHitSuffix(t *testing.T) {
	suffix := FormatHitSuffix()
	if suffix != " [cached]" {
		t.Errorf("got %q, want %q", suffix, " [cached]")
	}
}
