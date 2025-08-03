.PHONY: help test test-race test-cover lint build clean example install-tools

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: ## Run tests
	go test -v ./...

test-race: ## Run tests with race detector
	go test -v -race ./...

test-cover: ## Run tests with coverage
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

bench: ## Run benchmarks
	go test -bench=. -benchmem ./...

lint: ## Run linter
	golangci-lint run

lint-fix: ## Run linter with auto-fix
	golangci-lint run --fix

build: ## Build the library
	go build -v ./...

example: ## Build the examples
	cd example && go build -v .
	cd example-sighup && go build -v .
	cd example-simple && go build -v .
	cd example-multi && go build -v .

clean: ## Clean build artifacts
	go clean
	rm -f coverage.out coverage.html
	find . -name "*.test" -delete
	find ./example* -type f -executable -delete

install-tools: ## Install development tools
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

deps: ## Download dependencies
	go mod download
	go mod tidy

fmt: ## Format code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

check: fmt vet lint test ## Run all checks

ci: check ## Run CI pipeline locally

# Development workflow
dev: clean deps fmt vet lint test ## Full development workflow
