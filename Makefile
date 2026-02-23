# Modeltunnel Makefile
# Provides common development and build tasks

.PHONY: all build build-all test test-integration clean install uninstall fmt lint vet run docker help

# Variables
BINARY_NAME=modeltunnel
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DIR=./build
INSTALL_PATH=/usr/local/bin
GO_FILES=$(shell find . -name '*.go' -not -path './vendor/*' -not -path './venv/*')

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -s -w"

# Default target
all: build

## help: Show this help message
help:
	@echo "Modeltunnel - Available Commands:"
	@echo ""
	@echo "  make build          - Build the binary for current platform"
	@echo "  make build-all      - Build for all platforms (Linux, macOS, Windows)"
	@echo "  make test           - Run unit tests"
	@echo "  make test-integration - Run integration tests"
	@echo "  make test-all       - Run all tests"
	@echo "  make fmt            - Format Go code"
	@echo "  make lint           - Run linter"
	@echo "  make vet            - Run go vet"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make install        - Install binary to $(INSTALL_PATH)"
	@echo "  make uninstall      - Remove installed binary"
	@echo "  make run            - Build and run the server"
	@echo "  make docker-build   - Build Docker image"
	@echo "  make docker-run     - Run with Docker Compose"
	@echo "  make release        - Prepare release build"
	@echo "  make deps           - Download dependencies"
	@echo "  make generate       - Generate any needed code"
	@echo ""

## build: Build the binary for current platform
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/modeltunnel/main.go
	@echo "✅ Built: $(BUILD_DIR)/$(BINARY_NAME)"

## build-all: Build binaries for all platforms
build-all: clean
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	
	# Linux AMD64
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/modeltunnel/main.go
	@echo "✅ Linux AMD64"
	
	# Linux ARM64
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/modeltunnel/main.go
	@echo "✅ Linux ARM64"
	
	# macOS AMD64
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/modeltunnel/main.go
	@echo "✅ macOS AMD64"
	
	# macOS ARM64 (Apple Silicon)
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/modeltunnel/main.go
	@echo "✅ macOS ARM64"
	
	# Windows AMD64
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/modeltunnel/main.go
	@echo "✅ Windows AMD64"
	
	@echo ""
	@echo "All binaries built in $(BUILD_DIR)/"

## test: Run unit tests
test:
	@echo "Running unit tests..."
	go test -v -race ./...

## test-integration: Run integration tests (requires running server)
test-integration:
	@echo "Running integration tests..."
	@which python3 > /dev/null || (echo "❌ Python3 required for integration tests" && exit 1)
	@cd tests && python3 run_all_tests.py

## test-all: Run all tests
test-all: test test-integration

## fmt: Format Go code
fmt:
	@echo "Formatting Go code..."
	@gofmt -w $(GO_FILES)
	@echo "✅ Code formatted"

## lint: Run linter (requires golangci-lint)
lint:
	@which golangci-lint > /dev/null || (echo "❌ golangci-lint not installed. Run: brew install golangci-lint" && exit 1)
	@echo "Running linter..."
	golangci-lint run ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

## clean: Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)
	@echo "✅ Cleaned"

## install: Install binary to system (requires sudo)
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_PATH)..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@sudo chmod +x $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "✅ Installed to $(INSTALL_PATH)/$(BINARY_NAME)"
	@echo "Run '$(BINARY_NAME) --version' to verify"

## uninstall: Remove installed binary (requires sudo)
uninstall:
	@echo "Removing $(BINARY_NAME) from $(INSTALL_PATH)..."
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "✅ Uninstalled"

## run: Build and run the server with Ollama
run: build
	@echo "Starting Modeltunnel with Ollama..."
	$(BUILD_DIR)/$(BINARY_NAME) up --ollama --model mistral

## run-tunnel: Build and run with public tunnel
run-tunnel: build
	@echo "Starting Modeltunnel with public tunnel..."
	$(BUILD_DIR)/$(BINARY_NAME) up --ollama --model mistral --tunnel

## deps: Download and verify dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod verify
	@echo "✅ Dependencies ready"

## generate: Generate any needed code (mocks, etc.)
generate:
	@echo "Generating code..."
	go generate ./...

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) -t $(BINARY_NAME):latest .
	@echo "✅ Docker image built"

## docker-run: Run with Docker Compose
docker-run:
	@echo "Starting with Docker Compose..."
	docker-compose up -d
	@echo "✅ Services started"
	@echo "Dashboard: http://localhost:8080/admin"

## docker-stop: Stop Docker Compose
docker-stop:
	@echo "Stopping Docker Compose..."
	docker-compose down
	@echo "✅ Services stopped"

## release: Prepare release build
release: clean build-all
	@echo "Creating release $(VERSION)..."
	@mkdir -p $(BUILD_DIR)/release
	
	# Copy LICENSE and README to build directory
	@cp LICENSE README.md $(BUILD_DIR)/
	
	# Create archives for Linux/macOS
	@tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-linux-amd64 LICENSE README.md
	@tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-linux-arm64 LICENSE README.md
	@tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-darwin-amd64 LICENSE README.md
	@tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-darwin-arm64 LICENSE README.md
	
	# Create Windows archive
	@cd $(BUILD_DIR) && zip -j release/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe LICENSE README.md
	
	# Generate checksums
	@cd $(BUILD_DIR)/release && \
	sha256sum * > SHA256SUMS && \
	sha256sum -c SHA256SUMS && \
	rm -f SHA256SUMSUMSUMSUMS  # Remove duplicate names if any
	
	@echo "✅ Release archives created in $(BUILD_DIR)/release/"
	@ls -lh $(BUILD_DIR)/release/

## dev-setup: Setup development environment
dev-setup: deps
	@echo "Setting up development environment..."
	@echo "✅ Development environment ready"
	@echo ""
	@echo "Quick start:"
	@echo "  make build    - Build the binary"
	@echo "  make run      - Run with Ollama"
	@echo "  make test     - Run tests"

## check: Run all checks (fmt, vet, test)
check: fmt vet test
	@echo "✅ All checks passed"

# Catch-all target to prevent errors
%:
	@echo "❌ Unknown target: $@"
	@echo "Run 'make help' to see available commands"
