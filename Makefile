.PHONY: build test lint install clean run hooks lefthook

help: ## Outputs this help screen
	@grep -E '(^[a-zA-Z0-9_-]+:.*?##.*$$)|(^##)' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}{printf "\033[32m%-30s\033[0m %s\n", $$1, $$2}' | sed -e 's/\[32m##/[33m/'

# Build variables
BINARY_NAME := mehr
BUILD_DIR := ./build
CMD_DIR := ./cmd/mehr
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-s -w -v -X github.com/valksor/go-mehrhof/cmd/mehr/commands.Version=$(VERSION) -X github.com/valksor/go-mehrhof/cmd/mehr/commands.Commit=$(COMMIT) -X github.com/valksor/go-mehrhof/cmd/mehr/commands.BuildTime=$(BUILD_TIME)"

# Default target
all: build ## Build the binary (default target)

build: ## Compile the binary
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

test: ## Run tests with coverage
	go test -v -cover ./...

coverage: ## Run tests with race detection and coverage profile
	@go test -race -covermode atomic -coverprofile=covprofile.tmp ./...
	@grep -v testutil covprofile.tmp > covprofile || true
	@rm covprofile.tmp

coverage-html: coverage ## Generate HTML coverage report
	@mkdir -p .coverage
	go tool cover -html=covprofile -o .coverage/coverage.html

lint: ## Run linter (golangci-lint)
	${MAKE} fmt
	golangci-lint run ./...
	govulncheck ./...

fmt: ## Format code with go fmt, goimports, and gofumpt
	go fmt ./...
	goimports -w .
	gofumpt -l -w .

build-cross: ## Build cross-platform binary (use GOOS, GOARCH, OUTPUT, VERSION)
	@mkdir -p $(BUILD_DIR)
	$(eval CROSS_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none"))
	$(eval CROSS_BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ'))
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	go build -ldflags="-s -w -v -X github.com/valksor/go-mehrhof/cmd/mehr/commands.Version=$(VERSION) -X github.com/valksor/go-mehrhof/cmd/mehr/commands.Commit=$(CROSS_COMMIT) -X github.com/valksor/go-mehrhof/cmd/mehr/commands.BuildTime=$(CROSS_BUILD_TIME)" \
	-o $(OUTPUT) $(CMD_DIR)

install: build ## Install binary locally to GOPATH/bin
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
	@echo "Installed to $(GOPATH)/bin/$(BINARY_NAME)"

clean: ## Clean build artifacts
	rm -rf $(BUILD_DIR)
	rm -rf .coverage covprofile

run: build ## Run the binary (for development)
	$(BUILD_DIR)/$(BINARY_NAME)

run-args: build ## Run the binary with arguments (use ARGS=...)
	$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

tidy: clean ## Clean and tidy dependencies
	go mod tidy -e
	go get -d -v ./...

deps: ## Download dependencies
	go mod download

version: build ## Show version info
	$(BUILD_DIR)/$(BINARY_NAME) version

hooks: ## Configure git to use versioned hooks
	git config core.hooksPath .githooks
	@echo "Git hooks configured to use .githooks/"

lefthook: ## Install and configure Lefthook pre-commit hooks
	go install github.com/evilmartians/lefthook@latest
	lefthook install
	@echo "Lefthook installed. Pre-commit hooks active."
