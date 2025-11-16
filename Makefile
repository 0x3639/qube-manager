.PHONY: build build-all clean test vet version help

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS := -X 'main.Version=$(VERSION)' \
           -X 'main.GitCommit=$(COMMIT)' \
           -X 'main.BuildDate=$(BUILD_DATE)'

# Binary name
BINARY := qube-manager

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the application for current platform
	@echo "Building $(BINARY) $(VERSION) ($(COMMIT))..."
	go build -v -ldflags="$(LDFLAGS)" -o $(BINARY) .
	@echo "Build complete: ./$(BINARY)"

build-all: ## Build for all supported platforms
	@echo "Building for all platforms..."
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe .
	@echo "Build complete. Binaries in dist/"
	@ls -lh dist/

clean: ## Remove built binaries and artifacts
	@echo "Cleaning..."
	rm -f $(BINARY)
	rm -rf dist/
	rm -f coverage.txt
	@echo "Clean complete."

test: ## Run tests with coverage
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

vet: ## Run go vet
	go vet ./...

version: ## Show version information
	@echo "Version:    $(VERSION)"
	@echo "Commit:     $(COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

run: build ## Build and run the application
	./$(BINARY)

install: build ## Install the binary to $GOPATH/bin
	go install -ldflags="$(LDFLAGS)" .
