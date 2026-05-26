.PHONY: help build test docker-build docker-push clean run

# Variables
BINARY_NAME=webhook
DOCKER_IMAGE=external-dns-webhook-ipv64
DOCKER_TAG=latest
GO_FILES=$(shell find . -name '*.go' -type f)

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building..."
	go build -o bin/$(BINARY_NAME) ./cmd/webhook

test: ## Run tests
	@echo "Running tests..."
	go test -v ./...

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-push: ## Push Docker image
	@echo "Pushing Docker image..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	go clean

run: ## Run the webhook locally
	@echo "Running webhook..."
	go run ./cmd/webhook

fmt: ## Format Go code
	@echo "Formatting code..."
	go fmt ./...

lint: ## Run linter
	@echo "Running linter..."
	golangci-lint run

tidy: ## Tidy Go modules
	@echo "Tidying modules..."
	go mod tidy

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download

.DEFAULT_GOAL := help
