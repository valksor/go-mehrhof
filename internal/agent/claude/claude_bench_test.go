// Package claude tests
package claude

import (
	"testing"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
)

// Benchmark_BufferPool_Get benchmarks buffer pool retrieval.
func Benchmark_BufferPool_Get(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		bufPtr, ok := scannerBufferPool.Get().(*[]byte) //nolint:forcetypeassert,nolintlint // benchmark: controlled pool type
		if !ok {
			b.Fatal("scanner buffer pool returned wrong type")
		}
		scannerBufferPool.Put(bufPtr)
	}
}

// Benchmark_BufferPool_Parallel benchmarks concurrent buffer pool access.
func Benchmark_BufferPool_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bufPtr, ok := scannerBufferPool.Get().(*[]byte) //nolint:forcetypeassert,nolintlint // benchmark: controlled pool type
			if !ok {
				b.Fatal("scanner buffer pool returned wrong type")
			}
			scannerBufferPool.Put(bufPtr)
		}
	})
}

// Benchmark_New benchmarks agent creation.
func Benchmark_New(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		_ = New()
	}
}

// Benchmark_NewWithConfig benchmarks agent creation with config.
func Benchmark_NewWithConfig(b *testing.B) {
	cfg := agent.Config{
		Command:     []string{"claude"},
		Environment: make(map[string]string),
		Timeout:     30 * time.Minute,
		RetryCount:  3,
		RetryDelay:  time.Second,
	}
	b.ReportAllocs()
	for range b.N {
		_ = NewWithConfig(cfg)
	}
}

// Benchmark_WithTimeout benchmarks the WithTimeout method.
func Benchmark_WithTimeout(b *testing.B) {
	agent := New()
	b.ReportAllocs()
	for range b.N {
		_ = agent.WithTimeout(5 * time.Minute)
	}
}

// Benchmark_WithEnv benchmarks the WithEnv method.
func Benchmark_WithEnv(b *testing.B) {
	agent := New()
	b.ReportAllocs()
	for range b.N {
		_ = agent.WithEnv("KEY", "value")
	}
}

// Benchmark_WithArgs benchmarks the WithArgs method.
func Benchmark_WithArgs(b *testing.B) {
	agent := New()
	b.ReportAllocs()
	for range b.N {
		_ = agent.WithArgs([]string{"--model", "claude-sonnet-4"}...)
	}
}

// Benchmark_Agent_ConcurrentCreation benchmarks concurrent agent creation.
func Benchmark_Agent_ConcurrentCreation(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = New()
		}
	})
}

// Benchmark_Agent_ConcurrentWithEnv benchmarks concurrent WithEnv calls.
func Benchmark_Agent_ConcurrentWithEnv(b *testing.B) {
	agent := New()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = agent.WithEnv("KEY", "value")
		}
	})
}

// Benchmark_BuildArgs benchmarks argument building.
func Benchmark_BuildArgs(b *testing.B) {
	agent := New()
	b.ReportAllocs()
	for range b.N {
		_ = agent.buildArgs("test prompt")
	}
}

// Benchmark_BuildArgs_WithExtraArgs benchmarks argument building with extra args.
func Benchmark_BuildArgs_WithExtraArgs(b *testing.B) {
	cfg := agent.Config{
		Command: []string{"claude"},
		Args:    []string{"--model", "claude-opus-4"},
	}
	agent := NewWithConfig(cfg)
	b.ReportAllocs()
	for range b.N {
		_ = agent.buildArgs("test prompt")
	}
}
