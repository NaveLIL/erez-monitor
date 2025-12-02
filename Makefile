# Build targets for EREZMonitor

# Variables
APP_NAME := EREZMonitor
OUTPUT_DIR := build
OUTPUT := $(OUTPUT_DIR)/$(APP_NAME).exe
VERSION := 1.0.0
BUILD_TIME := $(shell date -u +"%Y-%m-%d %H:%M:%S")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOFMT := $(GOCMD) fmt

# Build flags
LDFLAGS := -s -w
LDFLAGS += -X 'main.Version=$(VERSION)'
LDFLAGS += -X 'main.BuildTime=$(BUILD_TIME)'
LDFLAGS += -X 'main.GitCommit=$(GIT_COMMIT)'

# Windows GUI flag (no console window)
LDFLAGS_RELEASE := $(LDFLAGS) -H=windowsgui

.PHONY: all build release test clean deps fmt lint run help

all: deps build

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) tidy
	$(GOMOD) download

# Build debug version (with console)
build: deps
	@echo "Building $(APP_NAME) (debug)..."
	@mkdir -p $(OUTPUT_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags="$(LDFLAGS)" -o $(OUTPUT) .
	@echo "Built: $(OUTPUT)"

# Build release version (no console)
release: deps
	@echo "Building $(APP_NAME) (release)..."
	@mkdir -p $(OUTPUT_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags="$(LDFLAGS_RELEASE)" -o $(OUTPUT) .
	@echo "Built: $(OUTPUT)"

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(OUTPUT_DIR)
	rm -f coverage.out coverage.html
	rm -f rsrc.syso rsrc_windows_amd64.syso

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run

# Run the application
run: build
	@echo "Running $(APP_NAME)..."
	$(OUTPUT) --debug

# Generate Windows resources (icon, manifest)
winres:
	@echo "Generating Windows resources..."
	go-winres make --in winres.json --product-version="$(VERSION).0" --file-version="$(VERSION).0"

# Build release with resources
release-full: winres release
	@echo "Release build with resources complete"

# Install development tools
tools:
	@echo "Installing development tools..."
	go install github.com/tc-hib/go-winres@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install go.uber.org/goleak/goleak@latest

# Help
help:
	@echo "Available targets:"
	@echo "  all        - Download deps and build (default)"
	@echo "  build      - Build debug version"
	@echo "  release    - Build release version (no console)"
	@echo "  test       - Run tests"
	@echo "  coverage   - Run tests with coverage"
	@echo "  clean      - Remove build artifacts"
	@echo "  fmt        - Format code"
	@echo "  lint       - Lint code"
	@echo "  run        - Build and run"
	@echo "  winres     - Generate Windows resources"
	@echo "  tools      - Install development tools"
	@echo "  help       - Show this help"
