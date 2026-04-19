include config.mk

.PHONY: install-deps
install-deps: ## Installs Dependencies
	@echo "--->  Installing Dependencies"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.PHONY: lint
lint: ## Linting
	@echo "--->  Linting"
	@golangci-lint run -v

.PHONY: lint-fix
lint-fix: ## Lint-Fixing code
	@echo "---> Lint-Fixing code"
	@golangci-lint run --fix

.PHONY: test
test: ## Test code
	@echo "--->  Running tests"
	@go test -v -count=1 ./...

.PHONY: test-cov
test-cov: ## Test with coverage
	@echo "--->  Running tests with coverage"
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out

# Build target
build: ## Build code
	@echo "---> Building for $(GOOS)/$(GOARCH) with binary name $(BINARY_NAME)"
	@mkdir -p $(OUTPUT_DIR)
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-s -w" -buildvcs=true -o $(OUTPUT_DIR)/$(BINARY_NAME) ./

# Target for Linux (amd64)
build-linux: ## Build for Linux (amd64)
	@$(MAKE) build GOOS=linux GOARCH=amd64

# Target for macOS (arm64)
build-macos-arm64: ## Build for macOS (arm64, Apple Silicon)
	@$(MAKE) build GOOS=darwin GOARCH=arm64

# Target for Windows (amd64)
build-windows: ## Build for Windows (amd64)
	@$(MAKE) build GOOS=windows GOARCH=amd64 BINARY_NAME=$(BINARY_NAME).exe

build-all: ## Build for all platforms
	@$(MAKE) build-linux
	@$(MAKE) build-macos-arm64
	@$(MAKE) build-windows

.PHONY: clean
clean: ## Clean bin and coverage files
	@echo "--->  Cleaning bin and coverage files"
	@rm -rf bin/
	@rm -f coverage.out

.PHONY: help
help: ## Help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sed 's/Makefile://' | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
