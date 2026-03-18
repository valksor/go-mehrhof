.PHONY: build test clean install run web-build web-dev fmt vet all help \
        desktop-dev desktop-build desktop-sidecar desktop-sidecar-all desktop-clean tauri-install \
        tidy deps version run-args check-alias web-test web-test-coverage \
        test-e2e test-e2e-provider test-e2e-gitlab test-e2e-workflow test-e2e-cli \
        man-pages

# Build variables
BINARY_NAME := kvelmo
BUILD_DIR := ./build
CMD_DIR := ./cmd/kvelmo
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-s -w -X github.com/valksor/kvelmo/pkg/meta.Version=$(VERSION) -X github.com/valksor/kvelmo/pkg/meta.Commit=$(COMMIT) -X github.com/valksor/kvelmo/pkg/meta.BuildTime=$(BUILD_TIME)"

# Default target
all: build

## Build the binary
build: web-build
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

## Build without web (faster for Go-only changes)
build-go:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

## Run kvelmo (sockets + web UI)
run: build
	$(BUILD_DIR)/$(BINARY_NAME) serve

## Run in development mode (no build, uses existing binary)
run-dev:
	$(BUILD_DIR)/$(BINARY_NAME) serve

## Run with arguments (use ARGS=...)
run-args: build
	$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

## Show version info
version: build-go
	$(BUILD_DIR)/$(BINARY_NAME) version

## Install binary locally
install: build
	@INSTALL_DIR="$$HOME/.local/bin"; \
	mkdir -p "$$INSTALL_DIR"; \
	cp $(BUILD_DIR)/$(BINARY_NAME) "$$INSTALL_DIR/$(BINARY_NAME)"; \
	echo "Installed to $$INSTALL_DIR/$(BINARY_NAME)"

## Run all tests (limit parallel packages to avoid resource exhaustion)
test: quality
	go test -p 4 ./pkg/... ./cmd/...

## Run tests with verbose output
test-v:
	go test -v -p 4 ./pkg/... ./cmd/...

## Run tests with coverage
test-cover:
	go test -p 4 -coverprofile=coverage.out ./pkg/... ./cmd/...
	go tool cover -html=coverage.out -o coverage.html

## Run tests with race detector
test-race:
	go test -race -p 4 ./pkg/... ./cmd/...

## Run all E2E tests (requires GITHUB_TOKEN and E2E_GITHUB_REPO)
test-e2e:
	go test -tags=e2e -v ./pkg/provider/... ./pkg/conductor/... -run TestE2E

## Run E2E provider tests only
test-e2e-provider:
	go test -tags=e2e -v ./pkg/provider/... -run TestE2E

## Run E2E GitLab provider tests only
test-e2e-gitlab:
	go test -tags=e2e -v -timeout=5m ./pkg/provider/... -run TestE2E_GitLab

## Run E2E workflow tests only
test-e2e-workflow:
	go test -tags=e2e -v ./pkg/conductor/... -run TestE2E

## Run full CLI E2E cycle test (requires GITHUB_TOKEN and E2E_GITHUB_REPO)
test-e2e-cli:
	go test -tags=e2e -v -timeout=30m ./e2e/... -run TestCLIFullCycle

## Format code
fmt:
	go fmt ./...
	@command -v goimports >/dev/null && find . -name '*.go' -not -path './.claude/*' -not -path './prototype/*' -not -path './vendor/*' -exec goimports -w {} + || true
	@command -v gofumpt >/dev/null && find . -name '*.go' -not -path './.claude/*' -not -path './prototype/*' -not -path './vendor/*' -exec gofumpt -l -w {} + || true

## Vet code
vet:
	go vet ./...

## Check for unnecessary import aliases
check-alias:
	@alias_issues="$$(./.github/alias.sh || true)"; \
	if [ -n "$$alias_issues" ]; then \
		echo "Unnecessary import alias detected:"; \
		echo "$$alias_issues"; \
		exit 1; \
	fi

## Quality checks (fmt + vet + lint + alias check)
quality: fmt vet check-alias
	golangci-lint run ./... --fix

# ──────────────────────────────────────────────────────────────────────────────
# Web Frontend
# ──────────────────────────────────────────────────────────────────────────────

## Install web dependencies
web-install:
	cd web && bun install

## Generate TypeScript types from Go structs
types:
	@mkdir -p web/src/types
	tygo generate

## Build web UI (generates types first)
web-build: types
	cd web && bun install && bun run build
	@echo "Copying web assets for embedding..."
	@rm -rf pkg/web/static/dist
	@cp -r web/dist pkg/web/static/dist

## Run web dev server (with hot reload, proxies to backend)
web-dev:
	cd web && bun run dev

## Run web tests
web-test:
	cd web && bun run test:run

## Run web tests with coverage
web-test-coverage:
	cd web && bun run test:coverage

## Run web e2e tests (demo mode, safe - no backend needed)
web-e2e:
	cd web && bun run test:e2e

## Run web e2e tests with UI (interactive debugging)
web-e2e-ui:
	cd web && bun run test:e2e:ui

# ──────────────────────────────────────────────────────────────────────────────
# Documentation
# ──────────────────────────────────────────────────────────────────────────────

## Generate man pages
man-pages:
	@mkdir -p man
	@go run ./cmd/kvelmo gen-man-pages man
	@echo "Man pages generated in man/"

# ──────────────────────────────────────────────────────────────────────────────
# Release
# ──────────────────────────────────────────────────────────────────────────────

## Build for release (all platforms)
release: web-build
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	GOOS=darwin GOARCH=arm64 go build -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)
	GOOS=linux GOARCH=amd64 go build -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)
	GOOS=linux GOARCH=arm64 go build -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)
	@echo "Release binaries in $(BUILD_DIR)/"

# ──────────────────────────────────────────────────────────────────────────────
# Desktop (Tauri)
# ──────────────────────────────────────────────────────────────────────────────

## Install Tauri CLI
tauri-install:
	cargo install tauri-cli --locked

## Run desktop app in development mode
desktop-dev: build-go desktop-sidecar
	./.github/desktop-dev.sh

## Build desktop app for production
desktop-build: desktop-sidecar
	cd web && bun tauri build

## Prepare sidecar binary for current platform
desktop-sidecar: build-go
	@mkdir -p web/src-tauri/binaries
	@TARGET=$$(rustc -vV | grep host | cut -d' ' -f2); \
	cp $(BUILD_DIR)/$(BINARY_NAME) web/src-tauri/binaries/$(BINARY_NAME)-$$TARGET; \
	echo "Prepared sidecar for $$TARGET"

## Prepare sidecar binaries for all platforms (for CI)
desktop-sidecar-all: release
	@mkdir -p web/src-tauri/binaries web/src-tauri/resources
	# macOS: sidecar binaries (Tauri auto-selects by target triple)
	cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 web/src-tauri/binaries/$(BINARY_NAME)-x86_64-apple-darwin
	cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 web/src-tauri/binaries/$(BINARY_NAME)-aarch64-apple-darwin
	# Linux: sidecar binaries
	cp $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 web/src-tauri/binaries/$(BINARY_NAME)-x86_64-unknown-linux-gnu
	cp $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 web/src-tauri/binaries/$(BINARY_NAME)-aarch64-unknown-linux-gnu
	# Windows: Linux binaries as resources (for WSL deployment, not sidecar)
	cp $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 web/src-tauri/resources/$(BINARY_NAME)-wsl-x64
	cp $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 web/src-tauri/resources/$(BINARY_NAME)-wsl-arm64
	@echo "Prepared sidecars for all platforms"

## Clean desktop build artifacts
desktop-clean:
	rm -rf web/src-tauri/target
	rm -rf web/src-tauri/binaries/*
	rm -rf web/src-tauri/resources/*

# ──────────────────────────────────────────────────────────────────────────────
# Cleanup
# ──────────────────────────────────────────────────────────────────────────────

## Clean build artifacts
clean: desktop-clean
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	rm -rf web/dist web/node_modules

## Download dependencies
deps:
	go mod download

## Clean and tidy dependencies
tidy: clean
	go mod tidy -e
	go get -d -v ./...

## CI checks (quality + test + build)
ci: quality test build

## Development workflow (quality + test + build + run)
dev: quality test run

## Show help
help:
	@echo "kvelmo Makefile"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | while read line; do \
		target=$$(echo "$$line" | head -1); \
		echo "  $$target"; \
	done
	@echo ""
	@echo "Common workflows:"
	@echo "  make run          - Build and run (sockets + web)"
	@echo "  make dev          - Quality + test + run"
	@echo "  make web-dev      - Frontend dev with hot reload"
	@echo "  make desktop-dev  - Desktop app dev with hot reload"
	@echo "  make desktop-build - Build desktop app for distribution"
