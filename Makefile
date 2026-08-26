# Variables
BIN_NAME := aws-creds-updater
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
VERSION := $(shell cat ./VERSION 2>/dev/null || echo "0.0.0")
REVISION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GOLDFLAGS := -s -w -X main.binName=$(BIN_NAME) -X main.version=${VERSION}-$(REVISION)
GOVERSION := 1.26
SRC := cmd/aws-creds-updater/main.go

.PHONY: default
default: build

.PHONY: build
build:
	@echo "Building $(BIN_NAME) for $(GOOS)/$(GOARCH)..."
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="$(GOLDFLAGS)" -o $(BIN_NAME) $(SRC)

.PHONY: b
b: build

.PHONY: build\:all
build\:all:
	@make build:macos
	@make build:linux

.PHONY: build\:macos
build\:macos:
	@make build:macos:arm64
	@make build:macos:amd64

.PHONY: build\:macos\:arm64
build\:macos\:arm64:
	@echo "Building $(BIN_NAME) for macOS ARM64..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$(GOLDFLAGS)" -o $(BIN_NAME)-$(VERSION)-macos-arm64 $(SRC)

.PHONY: build\:macos\:amd64
build\:macos\:amd64:
	@echo "Building $(BIN_NAME) for macOS Intel (amd64)..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="$(GOLDFLAGS)" -o $(BIN_NAME)-$(VERSION)-macos-amd64 $(SRC)

.PHONY: build\:linux
build\:linux:
	@make build:linux:arm64
	@make build:linux:amd64

.PHONY: build\:linux\:arm64
build\:linux\:arm64:
	@echo "Building $(BIN_NAME) for Linux ARM64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$(GOLDFLAGS)" -o $(BIN_NAME)-$(VERSION)-linux-arm64 $(SRC)

.PHONY: build\:linux\:amd64
build\:linux\:amd64:
	@echo "Building $(BIN_NAME) for Linux AMD64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(GOLDFLAGS)" -o $(BIN_NAME)-$(VERSION)-linux-amd64 $(SRC)

.PHONY: update
update:
	@echo "Updating Go packages..."
	go get -u ./...
	go mod tidy

.PHONY: u
u: update

.PHONY: install
install: build
	@echo "Installing $(BIN_NAME) to ~/go/bin/..."
	go install -ldflags="$(GOLDFLAGS)" ./cmd/aws-creds-updater
	@if [ -f $(BIN_NAME) ]; then \
		mv $(BIN_NAME) ~/go/bin/$(BIN_NAME); \
		echo "Installed to ~/go/bin/$(BIN_NAME)"; \
	fi

.PHONY: i
i: install

.PHONY: run
run:
	@go run -ldflags="$(GOLDFLAGS)" ./cmd/aws-creds-updater

.PHONY: r
r: run

.PHONY: test
test:
	@echo "Running tests..."
	go test ./cmd/...

.PHONY: t
t: test

.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BIN_NAME) \
		$(BIN_NAME)-*-macos-arm64 $(BIN_NAME)-*-macos-amd64 \
		$(BIN_NAME)-*-linux-arm64 $(BIN_NAME)-*-linux-amd64
	@echo "Cleaned!"

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build, b              - Build go binary ($(GOOS)/$(GOARCH))"
	@echo "  build:all             - Build for macOS and Linux (amd64 and arm64)"
	@echo "  build:macos           - Build for macOS (Intel amd64 and ARM64)"
	@echo "  build:macos:arm64     - Build for macOS ARM64"
	@echo "  build:macos:amd64     - Build for macOS Intel (amd64)"
	@echo "  build:linux           - Build for Linux (amd64 and ARM64)"
	@echo "  build:linux:arm64     - Build for Linux ARM64"
	@echo "  build:linux:amd64     - Build for Linux AMD64"
	@echo "  install, i            - Install go binary to ~/go/bin/"
	@echo "  update, u             - Update go packages and dependencies"
	@echo "  run, r                - Compile and run go code"
	@echo "  test, t               - Run tests"
	@echo "  clean                 - Remove build artifacts"
	@echo "  help                  - Show this help message"
	@echo ""
	@echo "Variables:"
	@echo "  GOOS        - Target OS (default: $(GOOS))"
	@echo "  GOARCH      - Target ARCH (default: $(GOARCH))"
	@echo "  REVISION    - Git revision (default: $(REVISION))"
	@echo ""
	@echo "Examples:"
	@echo "  make build                    - Build for current platform"
	@echo "  make build:macos:arm64        - Build for macOS ARM64"
	@echo "  make build GOOS=linux GOARCH=amd64 - Build for Linux AMD64"
	@echo "  make install                  - Build and install to ~/go/bin/"
	@echo "  make run                      - Run the application"
