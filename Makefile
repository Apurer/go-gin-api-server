.PHONY: all build run test test-unit test-integration lint fmt clean deps tidy help pact-consumer pact-provider pact-contracts

# Build variables
BINARY_NAME=petstore-api
WORKER_BINARY=petstore-worker
SESSION_PURGER_BINARY=session-purger
BUILD_DIR=bin
GO=go
GOFLAGS=-v

# Go module cache location
GO_MOD_CACHE=$(shell go env GOMODCACHE 2>/dev/null || echo $(HOME)/go/pkg/mod)

# Git repo info used by OpenAPI generator (can be overridden when calling make)
# Defaults set to the repository owner and name you requested.
GIT_USER_ID ?= Apurer
GIT_REPO_ID ?= go-gin-api-server

# Default target
all: lint test build

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## build: Build all binaries
build: build-api build-worker build-session-purger

## build-api: Build the API server
build-api:
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/api

## build-worker: Build the Temporal worker
build-worker:
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(WORKER_BINARY) ./cmd/worker

## build-session-purger: Build the Temporal worker
build-session-purger:
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(SESSION_PURGER_BINARY) ./cmd/session-purger

## run: Run the API server
run:
	$(GO) run ./cmd/api

## run-worker: Run the Temporal worker
run-worker:
	$(GO) run ./cmd/worker

## test: Run all tests
test: test-unit

## test-unit: Run unit tests only
test-unit:
	$(GO) test -v -short ./...

## test-integration: Run integration tests (requires Docker)
test-integration:
	$(GO) test -v -tags=integration ./...

## test-coverage: Run tests with coverage report
test-coverage:
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## pact-consumer: Run Pact consumer contract tests (generates ./pacts)
pact-consumer:
	$(GO) test -v -tags=pact ./test/pact/consumer -count=1

## pact-provider: Verify provider against generated Pact files
pact-provider:
	$(GO) test -v -tags=pact ./test/pact/provider -count=1

## pact-contracts: Generate and verify Pact contracts
pact-contracts: pact-consumer pact-provider

pact-publish:
	docker run --rm \
	-v "$$PWD/pacts:/pacts" \
	pactfoundation/pact-cli:latest \
	pact-broker publish /pacts \
	--consumer-app-version 1.0.0 \
	--tag local \
	--broker-base-url http://host.docker.internal:9292


## lint: Run all linters
lint: lint-go lint-depguard

## lint-go: Run golangci-lint
lint-go:
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

## lint-depguard: Run depguard dependency checker
lint-depguard:
	@which depguard > /dev/null || (echo "Installing depguard..." && go install github.com/OpenPeeDeeP/depguard/cmd/depguard@latest)
	@if [ -f depguard.json ]; then depguard -c depguard.json ./...; else echo "depguard.json not found, skipping"; fi

## fmt: Format Go code
fmt:
	$(GO) fmt ./...
	@which goimports > /dev/null || (echo "Installing goimports..." && go install golang.org/x/tools/cmd/goimports@latest)
	goimports -w .

## vet: Run go vet
vet:
	$(GO) vet ./...

## deps: Download dependencies
deps:
	$(GO) mod download

## tidy: Tidy and verify dependencies
tidy:
	$(GO) mod tidy
	$(GO) mod verify

## clean: Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

## docker-build: Build Docker image
docker-build:
	docker build -t $(BINARY_NAME):latest .

## docker-run: Run the API in Docker
docker-run:
	docker run -p 8080:8080 $(BINARY_NAME):latest

## generate: Run code generation
## openapi-gen: Generate OpenAPI server code (requires Docker)
openapi-gen:
	@which docker > /dev/null || (echo "Docker not found: please install Docker"; exit 1)
	docker run --rm -v "$(shell pwd)":/local openapitools/openapi-generator-cli generate -i /local/api/openapi.yaml -g go-gin-server -o /local/generated --git-user-id $(GIT_USER_ID) --git-repo-id $(GIT_REPO_ID)

## partner-client-gen: Generate the partner HTTP client (requires oapi-codegen)
partner-client-gen:
	$(GO) generate ./internal/clients/http/partner

## generate: Run code generation (OpenAPI + partner client)
generate: openapi-gen partner-client-gen

## openapi-validate: Validate OpenAPI spec
openapi-validate:
	@which swagger > /dev/null || echo "swagger CLI not installed, skipping validation"
	@which swagger > /dev/null && swagger validate api/openapi.yaml || true

## install-tools: Install development tools
install-tools:
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install github.com/OpenPeeDeeP/depguard/cmd/depguard@latest
	$(GO) install golang.org/x/tools/cmd/goimports@latest

## watch: Run with file watching (requires air)
watch:
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	air

# CI targets
## ci: Run CI checks (lint + test)
ci: lint test-unit

## ci-full: Run full CI checks including integration tests
ci-full: lint test-unit test-integration
