package discovery

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestDiscoverTools_Makefile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(`
build:
	go build ./...

test:
	go test ./...

.PHONY: build test

_internal:
	echo hidden

CC := gcc
`), 0o644); err != nil {
		t.Fatal(err)
	}

	tools := DiscoverTools(dir)

	if !slices.Contains(tools, "make build") {
		t.Error("expected 'make build'")
	}
	if !slices.Contains(tools, "make test") {
		t.Error("expected 'make test'")
	}
	if slices.Contains(tools, "make _internal") {
		t.Error("should not include underscore targets")
	}
	if slices.Contains(tools, "make CC") {
		t.Error("should not include variable assignments")
	}
}

func TestDiscoverTools_PackageJSON_Bun(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
		"scripts": {
			"dev": "vite",
			"build": "vite build",
			"test": "vitest"
		}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create bun.lockb to signal bun
	if err := os.WriteFile(filepath.Join(dir, "bun.lockb"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	tools := DiscoverTools(dir)

	if !slices.Contains(tools, "bun run dev") {
		t.Error("expected 'bun run dev'")
	}
	if !slices.Contains(tools, "bun run build") {
		t.Error("expected 'bun run build'")
	}
}

func TestDiscoverTools_PackageJSON_NPM(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
		"scripts": {"start": "node index.js"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	tools := DiscoverTools(dir)

	if !slices.Contains(tools, "npm run start") {
		t.Error("expected 'npm run start'")
	}
}

func TestDiscoverTools_Taskfile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Taskfile.yml"), []byte(`
version: '3'
tasks:
  build:
    cmds:
      - go build ./...
  lint:
    cmds:
      - golangci-lint run
`), 0o644); err != nil {
		t.Fatal(err)
	}

	tools := DiscoverTools(dir)

	if !slices.Contains(tools, "task build") {
		t.Error("expected 'task build'")
	}
	if !slices.Contains(tools, "task lint") {
		t.Error("expected 'task lint'")
	}
}

func TestDiscoverTools_BinExecutables(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "deploy"), []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Non-executable should be excluded
	if err := os.WriteFile(filepath.Join(binDir, "README.md"), []byte("docs"), 0o644); err != nil {
		t.Fatal(err)
	}

	tools := DiscoverTools(dir)

	if !slices.Contains(tools, "./bin/deploy") {
		t.Error("expected './bin/deploy'")
	}
	if slices.Contains(tools, "./bin/README.md") {
		t.Error("should not include non-executable files")
	}
}

func TestDiscoverTools_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	tools := DiscoverTools(dir)
	if len(tools) != 0 {
		t.Errorf("expected no tools for empty dir, got %v", tools)
	}
}
