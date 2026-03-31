.PHONY: build build-go test test-cover test-race test-e2e quality ci-quality ci-test types \
        web-build web-dev web-test web-e2e web-e2e-ui \
        run run-dev install release man-pages \
        desktop-dev desktop-build desktop-sidecar desktop-sidecar-all desktop-clean tauri-install \
        prototype-lock prototype-unlock \
        clean tidy deps ci dev version help all

# Build variables
BINARY_NAME := kvelmo
BUILD_DIR := ./build
CMD_DIR := ./cmd/kvelmo
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-s -w -X github.com/valksor/kvelmo/pkg/meta.Version=$(VERSION) -X github.com/valksor/kvelmo/pkg/meta.Commit=$(COMMIT) -X github.com/valksor/kvelmo/pkg/meta.BuildTime=$(BUILD_TIME)"

# E2E suite selection (SUITE=provider|workflow|cli|all, default: all)
SUITE ?= all

# Default target
all: build

# ──────────────────────────────────────────────────────────────────────────────
# Quality & Testing
# ──────────────────────────────────────────────────────────────────────────────

## Full-stack quality checks (Go fmt/vet/lint + frontend lint/typecheck + Tauri fmt/clippy)
quality:
	@command -v goimports >/dev/null && find . -name '*.go' -not -path './.claude/*' -not -path './prototype/*' -not -path './vendor/*' -exec goimports -w {} + || true
	@command -v gofumpt >/dev/null && find . -name '*.go' -not -path './.claude/*' -not -path './prototype/*' -not -path './vendor/*' -exec gofumpt -l -w {} + || true
	go vet ./...
	@alias_issues="$$(./.github/alias.sh || true)"; \
	if [ -n "$$alias_issues" ]; then \
		echo "Unnecessary import alias detected:"; \
		echo "$$alias_issues"; \
		exit 1; \
	fi
	golangci-lint run ./... --fix
	cd web && bun install --frozen-lockfile && bun run lint && bun run typecheck
	cd web/src-tauri && cargo fmt --check && cargo clippy -- -D warnings

## Full-stack tests (Go + frontend + Tauri unit tests)
test: quality
	go test -p 4 ./pkg/... ./cmd/...
	cd web && bun run test:run
	cd web/src-tauri && cargo test --lib

## CI quality checks (Go + frontend only, Tauri checked in dedicated CI job)
ci-quality:
	go fmt ./...
	@command -v goimports >/dev/null && find . -name '*.go' -not -path './.claude/*' -not -path './prototype/*' -not -path './vendor/*' -exec goimports -w {} + || true
	@command -v gofumpt >/dev/null && find . -name '*.go' -not -path './.claude/*' -not -path './prototype/*' -not -path './vendor/*' -exec gofumpt -l -w {} + || true
	go vet ./...
	@alias_issues="$$(./.github/alias.sh || true)"; \
	if [ -n "$$alias_issues" ]; then \
		echo "Unnecessary import alias detected:"; \
		echo "$$alias_issues"; \
		exit 1; \
	fi
	golangci-lint run ./... --fix
	cd web && bun install --frozen-lockfile && bun run lint && bun run typecheck

## CI tests (Go + frontend only, Tauri tested in dedicated CI job)
ci-test: ci-quality
	go test -p 4 ./pkg/... ./cmd/...
	cd web && bun run test:run

## Go tests with coverage report
test-cover:
	go test -p 4 -coverprofile=coverage.out ./pkg/... ./cmd/...
	go tool cover -html=coverage.out -o coverage.html

## Go tests with race detector
test-race:
	go test -race -p 4 ./pkg/... ./cmd/...

## E2E tests (SUITE=provider|workflow|cli|all, default: all provider+workflow tests; cli excluded due to 30m timeout)
test-e2e:
ifeq ($(SUITE),provider)
	go test -tags=e2e -v ./pkg/provider/... -run TestE2E
else ifeq ($(SUITE),workflow)
	go test -tags=e2e -v ./pkg/conductor/... -run TestE2E
else ifeq ($(SUITE),cli)
	go test -tags=e2e -v -timeout=30m ./e2e/... -run TestCLIFullCycle
else
	go test -tags=e2e -v ./pkg/provider/... ./pkg/conductor/... -run TestE2E
endif

# ──────────────────────────────────────────────────────────────────────────────
# Build
# ──────────────────────────────────────────────────────────────────────────────

## Full build (web + Go binary)
build: web-build
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

## Go-only build (faster, skips web)
build-go:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

## Build for release (all platforms)
release: web-build
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	GOOS=darwin GOARCH=arm64 go build -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)
	GOOS=linux GOARCH=amd64 go build -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)
	GOOS=linux GOARCH=arm64 go build -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)
	@echo "Release binaries in $(BUILD_DIR)/"

## Install to ~/.local/bin
install: build
	@INSTALL_DIR="$$HOME/.local/bin"; \
	mkdir -p "$$INSTALL_DIR"; \
	cp $(BUILD_DIR)/$(BINARY_NAME) "$$INSTALL_DIR/$(BINARY_NAME)"; \
	echo "Installed to $$INSTALL_DIR/$(BINARY_NAME)"

# ──────────────────────────────────────────────────────────────────────────────
# Run
# ──────────────────────────────────────────────────────────────────────────────

## Build and run (sockets + web UI)
run: build
	$(BUILD_DIR)/$(BINARY_NAME) serve

## Run without rebuilding (uses existing binary)
run-dev:
	$(BUILD_DIR)/$(BINARY_NAME) serve

## Show version info
version: build-go
	$(BUILD_DIR)/$(BINARY_NAME) version

# ──────────────────────────────────────────────────────────────────────────────
# Web Frontend
# ──────────────────────────────────────────────────────────────────────────────

## Generate TypeScript types from Go structs
types:
	@mkdir -p web/src/types
	tygo generate
	@# tygo emits `any` for Go function types it cannot convert; replace with `unknown`
	@sed -i 's/= any;/= unknown;/g' web/src/types/conductor.ts

## Build web UI
web-build: types
	cd web && bun install --frozen-lockfile && bun run build
	@echo "Copying web assets for embedding..."
	@rm -rf pkg/web/static/dist
	@cp -r web/dist pkg/web/static/dist

## Frontend dev server with hot reload
web-dev:
	cd web && bun run dev

## Frontend unit tests
web-test:
	cd web && bun run test:run

## Frontend e2e tests (demo mode, no backend needed)
web-e2e:
	cd web && bun run test:e2e

## Frontend e2e tests with UI (interactive debugging)
web-e2e-ui:
	cd web && bun run test:e2e:ui

# ──────────────────────────────────────────────────────────────────────────────
# Desktop (Tauri)
# ──────────────────────────────────────────────────────────────────────────────

## Install Tauri CLI
tauri-install:
	cargo install tauri-cli --locked

## Desktop app dev mode
desktop-dev: build-go desktop-sidecar
	./.github/desktop-dev.sh

## Desktop app production build
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
	cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 web/src-tauri/binaries/$(BINARY_NAME)-x86_64-apple-darwin
	cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 web/src-tauri/binaries/$(BINARY_NAME)-aarch64-apple-darwin
	cp $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 web/src-tauri/binaries/$(BINARY_NAME)-x86_64-unknown-linux-gnu
	cp $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 web/src-tauri/binaries/$(BINARY_NAME)-aarch64-unknown-linux-gnu
	cp $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 web/src-tauri/resources/$(BINARY_NAME)-wsl-x64
	cp $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 web/src-tauri/resources/$(BINARY_NAME)-wsl-arm64
	@echo "Prepared sidecars for all platforms"

## Clean desktop build artifacts
desktop-clean:
	rm -rf web/src-tauri/target
	rm -rf web/src-tauri/binaries/*
	rm -rf web/src-tauri/resources/*

# ──────────────────────────────────────────────────────────────────────────────
# Misc
# ──────────────────────────────────────────────────────────────────────────────

## Generate man pages
man-pages:
	@mkdir -p man
	@go run ./cmd/kvelmo gen-man-pages man
	@echo "Man pages generated in man/"

## Lock prototype/ directory (remove write permissions)
prototype-lock:
	chmod -R a-w prototype/

## Unlock prototype/ directory (restore write permissions for owner)
prototype-unlock:
	chmod -R u+w prototype/

## Clean all build artifacts
clean: desktop-clean
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	rm -rf web/dist web/node_modules

## Download Go dependencies
deps:
	go mod download

## Clean and tidy dependencies
tidy: clean
	go mod tidy -e
	go get -d -v ./...

## CI checks (ci-quality + ci-test + build; Tauri checked separately)
ci: ci-quality ci-test build

## Full dev workflow (quality + test + run)
dev: quality test run

## Show available targets
help:
	@echo "Development:"
	@echo "  make dev            Quality + test + run"
	@echo "  make run            Build and run (sockets + web UI)"
	@echo "  make run-dev        Run without rebuilding"
	@echo "  make web-dev        Frontend dev server with hot reload"
	@echo ""
	@echo "Quality & Testing:"
	@echo "  make quality        Full-stack: Go fmt/vet/lint + frontend lint/typecheck + Tauri fmt/clippy"
	@echo "  make test           Full-stack: Go tests + frontend unit tests + Tauri tests"
	@echo "  make test-e2e       E2E tests (SUITE=provider|workflow|cli|all)"
	@echo "  make test-cover     Go tests with coverage report"
	@echo "  make test-race      Go tests with race detector"
	@echo "  make web-test       Frontend unit tests only"
	@echo "  make web-e2e        Frontend e2e tests (demo mode)"
	@echo ""
	@echo "Build & Release:"
	@echo "  make build          Full build (web + Go binary)"
	@echo "  make build-go       Go-only build (faster)"
	@echo "  make release        Release binaries for all platforms"
	@echo "  make install        Install to ~/.local/bin"
	@echo "  make ci             quality + test + build"
	@echo ""
	@echo "Desktop:"
	@echo "  make desktop-dev    Desktop app dev mode"
	@echo "  make desktop-build  Desktop app production build"
	@echo ""
	@echo "Prototype:"
	@echo "  make prototype-lock   Remove write permissions on prototype/"
	@echo "  make prototype-unlock Restore write permissions on prototype/"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean          Remove all build artifacts"
	@echo "  make tidy           Clean and tidy dependencies"
