.PHONY: build install update run test clean help default

# Variables
BIN_NAME := aws-creds-updater
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
VERSION := $(shell cat ./VERSION 2>/dev/null || echo "0.0.0")
REVISION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GOLDFLAGS := -s -w -X main.binName=$(BIN_NAME) -X main.version=${VERSION}-$(REVISION)
GOVERSION := 1.27

# Default target
default: build

# Build the Go binary
build:
	@echo "Building $(BIN_NAME) for $(GOOS)/$(GOARCH)..."
	CGO_ENABLED=0 go build -ldflags="$(GOLDFLAGS)" -o $(BIN_NAME) cmd/aws-creds-updater/main.go

# Alias for build
b: build

# Update Go packages and dependencies
update:
	@echo "Updating Go packages..."
	go get -u ./...
	go mod tidy

# Alias for update
u: update

# Install the binary to ~/go/bin/
install: build
	@echo "Installing $(BIN_NAME) to ~/go/bin/..."
	go install -ldflags="$(GOLDFLAGS)" ./cmd/aws-creds-updater
	@if [ -f $(BIN_NAME) ]; then \
		mv $(BIN_NAME) ~/go/bin/$(BIN_NAME); \
		echo "Installed to ~/go/bin/$(BIN_NAME)"; \
	fi

# Alias for install
i: install

# Compile and run the code
run:
	@go run -ldflags="$(GOLDFLAGS)" ./cmd/aws-creds-updater

# Alias for run
r: run

# Run tests
test:
	@echo "Running tests..."
	go test ./cmd/...

# Alias for test
t: test

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BIN_NAME)
	@echo "Cleaned!"

# Show help
help:
	@echo "Available targets:"
	@echo "  build, b    - Build go binary ($(GOOS)/$(GOARCH))"
	@echo "  install, i  - Install go binary to ~/go/bin/"
	@echo "  update, u   - Update go packages and dependencies"
	@echo "  run, r      - Compile and run go code"
	@echo "  test, t     - Run tests"
	@echo "  clean       - Remove build artifacts"
	@echo "  help        - Show this help message"
	@echo ""
	@echo "Variables:"
	@echo "  GOOS        - Target OS (default: $(GOOS))"
	@echo "  GOARCH      - Target ARCH (default: $(GOARCH))"
	@echo "  REVISION    - Git revision (default: $(REVISION))"
	@echo ""
	@echo "Examples:"
	@echo "  make build           - Build for current platform"
	@echo "  make build GOOS=linux GOARCH=amd64 - Build for Linux AMD64"
	@echo "  make install         - Build and install to ~/go/bin/"
	@echo "  make run             - Run the application"

