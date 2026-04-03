BINARY=sekd
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-s -w -X github.com/jefrnc/sekd/cmd.Version=$(VERSION)"

.PHONY: help build run test test-coverage vet lint fmt clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

build: ## Build binary
	go build $(LDFLAGS) -o $(BINARY) .

run: build ## Build and run interactive mode
	./$(BINARY)

test: ## Run tests
	go test -race -count=1 ./internal/...

test-coverage: ## Run tests with coverage report
	go test -race -coverprofile=coverage.out ./internal/...
	go tool cover -func=coverage.out
	@echo "HTML report: coverage.html"
	go tool cover -html=coverage.out -o coverage.html

vet: ## Run go vet
	go vet ./...

lint: ## Run linter (requires golangci-lint)
	golangci-lint run ./...

fmt: ## Format code
	go fmt ./...

clean: ## Remove build artifacts and cache
	rm -f $(BINARY) coverage.out coverage.html
	rm -rf dist/
