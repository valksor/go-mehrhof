// Package storage tests
package storage

import (
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// Benchmark_AddUsage_Sequential benchmarks sequential AddUsage calls.
// With buffering enabled, this should be much faster as writes are batched.
func Benchmark_AddUsage_Sequential(b *testing.B) {
	tmpDir := b.TempDir()
	ws, err := OpenWorkspace(tmpDir, nil)
	if err != nil {
		b.Fatal(err)
	}
	taskID := "test-task"
	source := SourceInfo{Type: "file", Ref: "test"}
	work, err := ws.CreateWork(taskID, source)
	if err != nil {
		b.Fatal(err)
	}
	_ = work

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		if err := ws.AddUsage(taskID, "planning", 1000, 500, 100, 0.01); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark_AddUsage_Flush benchmarks AddUsage with explicit flush.
func Benchmark_AddUsage_Flush(b *testing.B) {
	tmpDir := b.TempDir()
	ws, err := OpenWorkspace(tmpDir, nil)
	if err != nil {
		b.Fatal(err)
	}
	taskID := "test-task"
	source := SourceInfo{Type: "file", Ref: "test"}
	work, err := ws.CreateWork(taskID, source)
	if err != nil {
		b.Fatal(err)
	}
	_ = work

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		for range 10 {
			if err := ws.AddUsage(taskID, "planning", 1000, 500, 100, 0.01); err != nil {
				b.Fatal(err)
			}
		}
		if err := ws.FlushUsage(); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark_AddUsage_Concurrent benchmarks concurrent AddUsage calls.
func Benchmark_AddUsage_Concurrent(b *testing.B) {
	tmpDir := b.TempDir()
	ws, err := OpenWorkspace(tmpDir, nil)
	if err != nil {
		b.Fatal(err)
	}
	taskID := "test-task"
	source := SourceInfo{Type: "file", Ref: "test"}
	work, err := ws.CreateWork(taskID, source)
	if err != nil {
		b.Fatal(err)
	}
	_ = work

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := ws.AddUsage(taskID, "planning", 1000, 500, 100, 0.01); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Benchmark_AddUsage_MultipleTasks benchmarks AddUsage across multiple tasks.
func Benchmark_AddUsage_MultipleTasks(b *testing.B) {
	tmpDir := b.TempDir()
	ws, err := OpenWorkspace(tmpDir, nil)
	if err != nil {
		b.Fatal(err)
	}

	// Create 10 tasks
	taskIDs := make([]string, 10)
	for i := range 10 {
		taskID := GenerateTaskID()
		taskIDs[i] = taskID
		source := SourceInfo{Type: "file", Ref: "test"}
		work, err := ws.CreateWork(taskID, source)
		if err != nil {
			b.Fatal(err)
		}
		_ = work
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := range b.N {
		taskID := taskIDs[i%10]
		if err := ws.AddUsage(taskID, "planning", 1000, 500, 100, 0.01); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark_FlushUsage benchmarks the FlushUsage operation.
func Benchmark_FlushUsage(b *testing.B) {
	tmpDir := b.TempDir()
	ws, err := OpenWorkspace(tmpDir, nil)
	if err != nil {
		b.Fatal(err)
	}
	taskID := "test-task"
	source := SourceInfo{Type: "file", Ref: "test"}
	work, err := ws.CreateWork(taskID, source)
	if err != nil {
		b.Fatal(err)
	}
	_ = work

	// Pre-populate buffer
	for range 100 {
		if err := ws.AddUsage(taskID, "planning", 1000, 500, 100, 0.01); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		// Add more usage
		for range 10 {
			if err := ws.AddUsage(taskID, "implementing", 1000, 500, 100, 0.01); err != nil {
				b.Fatal(err)
			}
		}
		if err := ws.FlushUsage(); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark_LoadWork benchmarks loading work metadata.
func Benchmark_LoadWork(b *testing.B) {
	tmpDir := b.TempDir()
	ws, err := OpenWorkspace(tmpDir, nil)
	if err != nil {
		b.Fatal(err)
	}
	taskID := "test-task"
	source := SourceInfo{Type: "file", Ref: "test"}
	work, err := ws.CreateWork(taskID, source)
	if err != nil {
		b.Fatal(err)
	}
	// Add some usage data
	for range 100 {
		if err := ws.AddUsage(taskID, "planning", 1000, 500, 100, 0.01); err != nil {
			b.Fatal(err)
		}
	}
	if err := ws.FlushUsage(); err != nil {
		b.Fatal(err)
	}
	_ = work

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		_, err := ws.LoadWork(taskID)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark_SaveWork benchmarks saving work metadata.
func Benchmark_SaveWork(b *testing.B) {
	tmpDir := b.TempDir()
	ws, err := OpenWorkspace(tmpDir, nil)
	if err != nil {
		b.Fatal(err)
	}
	taskID := "test-task"
	source := SourceInfo{Type: "file", Ref: "test"}
	work, err := ws.CreateWork(taskID, source)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		if err := ws.SaveWork(work); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark_CreateWork benchmarks creating a new work directory.
func Benchmark_CreateWork(b *testing.B) {
	tmpDir := b.TempDir()
	ws, err := OpenWorkspace(tmpDir, nil)
	if err != nil {
		b.Fatal(err)
	}
	source := SourceInfo{Type: "file", Ref: "test"}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		taskID := GenerateTaskID()
		_, err := ws.CreateWork(taskID, source)
		if err != nil {
			b.Fatal(err)
		}
		// Cleanup for next iteration
		_ = ws.DeleteWork(taskID)
	}
}

// Benchmark_GenerateTaskID benchmarks task ID generation.
func Benchmark_GenerateTaskID(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		_ = GenerateTaskID()
	}
}

// Benchmark_WorkPath benchmarks work path generation.
func Benchmark_WorkPath(b *testing.B) {
	tmpDir := b.TempDir()
	ws, err := OpenWorkspace(tmpDir, nil)
	if err != nil {
		b.Fatal(err)
	}
	taskID := "test-task-abc123"

	b.ReportAllocs()
	for range b.N {
		_ = ws.WorkPath(taskID)
	}
}

// Benchmark_OpenWorkspace benchmarks workspace opening.
func Benchmark_OpenWorkspace(b *testing.B) {
	tmpDir := b.TempDir()

	b.ReportAllocs()
	for range b.N {
		_, err := OpenWorkspace(tmpDir, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark_AddUsage_WithoutBuffering simulates the old behavior
// for comparison - direct LoadWork -> Modify -> SaveWork.
func Benchmark_AddUsage_WithoutBuffering(b *testing.B) {
	tmpDir := b.TempDir()
	ws, err := OpenWorkspace(tmpDir, nil)
	if err != nil {
		b.Fatal(err)
	}
	taskID := "test-task"
	source := SourceInfo{Type: "file", Ref: "test"}
	work, err := ws.CreateWork(taskID, source)
	if err != nil {
		b.Fatal(err)
	}
	_ = work

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		// Simulate old behavior: load, modify, save
		work, err := ws.LoadWork(taskID)
		if err != nil {
			b.Fatal(err)
		}
		if work.Costs.ByStep == nil {
			work.Costs.ByStep = make(map[string]StepCostStats)
		}
		work.Costs.TotalInputTokens += 1000
		work.Costs.TotalOutputTokens += 500
		work.Costs.TotalCachedTokens += 100
		work.Costs.TotalCostUSD += 0.01
		stepStats := work.Costs.ByStep["planning"]
		stepStats.InputTokens += 1000
		stepStats.OutputTokens += 500
		stepStats.CachedTokens += 100
		stepStats.CostUSD += 0.01
		stepStats.Calls++
		work.Costs.ByStep["planning"] = stepStats
		if err := ws.SaveWork(work); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark_YAMLMarshal benchmarks YAML marshaling of work metadata.
func Benchmark_YAMLMarshal(b *testing.B) {
	source := SourceInfo{Type: "file", Ref: "test"}
	work := NewTaskWork("test-id", source)
	// Add some realistic data
	for range 50 {
		work.Costs.ByStep["step"] = StepCostStats{
			InputTokens:  1000,
			OutputTokens: 500,
			CachedTokens: 100,
			CostUSD:      0.01,
			Calls:        10,
		}
	}

	b.ReportAllocs()
	for range b.N {
		_, err := yaml.Marshal(work)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark_WorkPath_Join benchmarks using filepath.Join vs string concatenation.
func Benchmark_WorkPath_Join(b *testing.B) {
	tmpDir := b.TempDir()
	ws, err := OpenWorkspace(tmpDir, nil)
	if err != nil {
		b.Fatal(err)
	}
	taskID := "test-task-abc123"

	b.ReportAllocs()
	for range b.N {
		_ = filepath.Join(ws.WorkRoot(), taskID)
	}
}
