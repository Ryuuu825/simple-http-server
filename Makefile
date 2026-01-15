# Simple HTTP Server - Build Makefile

# Binary name
BINARY_NAME=simple-http-server

# Version
VERSION?=1.0.0

# Build directory
BUILD_DIR=build

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-s -w"

.PHONY: all build clean test deps build-all build-linux build-darwin build-windows help

# Default target
all: clean build-all

# Build for current platform
build:
	@echo "Building for current platform..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) .
	@echo "Build complete: $(BINARY_NAME)"

# Build for all platforms
build-all: build-linux build-darwin build-windows
	@echo "All builds complete!"

# Build for Linux (amd64)
build-linux:
	@echo "Building for Linux (amd64)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	@echo "Linux build complete: $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64"

# Build for Linux (arm64)
build-linux-arm64:
	@echo "Building for Linux (arm64)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 .
	@echo "Linux ARM64 build complete: $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64"

# Build for macOS (amd64 - Intel)
build-darwin:
	@echo "Building for macOS (amd64 - Intel)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	@echo "macOS Intel build complete: $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64"

# Build for macOS (arm64 - Apple Silicon)
build-darwin-arm64:
	@echo "Building for macOS (arm64 - Apple Silicon)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .
	@echo "macOS Apple Silicon build complete: $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64"

# Build for Windows (amd64)
build-windows:
	@echo "Building for Windows (amd64)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .
	@echo "Windows build complete: $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe"

# Build for all platforms with ARM64 support
build-all-platforms: build-linux build-linux-arm64 build-darwin build-darwin-arm64 build-windows
	@echo "All platform builds complete!"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)
	@rm -f server
	@echo "Clean complete!"

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "Dependencies updated!"

# Run the server
run: build
	@echo "Starting server..."
	./$(BINARY_NAME)

# Display help
help:
	@echo "Simple HTTP Server - Makefile commands:"
	@echo ""
	@echo "  make build              - Build for current platform"
	@echo "  make build-all          - Build for Linux, macOS, and Windows (amd64)"
	@echo "  make build-all-platforms - Build for all platforms including ARM64"
	@echo "  make build-linux        - Build for Linux (amd64)"
	@echo "  make build-linux-arm64  - Build for Linux (arm64)"
	@echo "  make build-darwin       - Build for macOS Intel (amd64)"
	@echo "  make build-darwin-arm64 - Build for macOS Apple Silicon (arm64)"
	@echo "  make build-windows      - Build for Windows (amd64)"
	@echo "  make clean              - Remove build artifacts"
	@echo "  make test               - Run tests"
	@echo "  make deps               - Download and tidy dependencies"
	@echo "  make run                - Build and run the server"
	@echo "  make help               - Show this help message"
	@echo ""
