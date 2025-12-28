.PHONY: build test lint install clean run

# Build variables
BINARY_NAME := mehr
BUILD_DIR := ./build
CMD_DIR := ./cmd/mehr
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-X github.com/valksor/go-mehrhof/cmd/mehr/commands.Version=$(VERSION) -X github.com/valksor/go-mehrhof/cmd/mehr/commands.Commit=$(COMMIT) -X github.com/valksor/go-mehrhof/cmd/mehr/commands.BuildTime=$(BUILD_TIME)"

# Default target
all: build

# Build the binary
build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

test:
	go test -v -cover ./...

coverage:
	go test -race -covermode atomic -coverprofile=covprofile ./...

coverage-html: coverage
	go tool cover -html=covprofile -o .coverage/coverage.html

# Run linter
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed"; exit 1; }
	golangci-lint run ./...

# Format code
fmt:
	go fmt ./...
	goimports -w .
	gofumpt -l -w .

# Install locally
install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
	@echo "Installed to $(GOPATH)/bin/$(BINARY_NAME)"

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -rf .coverage covprofile

# Run the binary (for development)
run: build
	$(BUILD_DIR)/$(BINARY_NAME)

# Run with arguments
run-args: build
	$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

# Tidy dependencies
tidy:
	go mod tidy

# Download dependencies
deps:
	go mod download

# Show version info
version: build
	$(BUILD_DIR)/$(BINARY_NAME) version
