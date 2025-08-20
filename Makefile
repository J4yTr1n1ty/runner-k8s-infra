.PHONY: build build-controller build-deploy-runner clean test test-coverage test-race fmt vet lint install-deps docker-build ci-setup

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet
GOLINT=golangci-lint

# Binary names
CONTROLLER_BINARY_NAME=controller
DEPLOY_RUNNER_BINARY_NAME=deploy-runner
CONTROLLER_BINARY_PATH=bin/$(CONTROLLER_BINARY_NAME)
DEPLOY_RUNNER_BINARY_PATH=bin/$(DEPLOY_RUNNER_BINARY_NAME)

# Build targets
build: build-controller build-deploy-runner

build-controller:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux $(GOBUILD) -o $(CONTROLLER_BINARY_PATH) ./cmd/controller

build-deploy-runner:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux $(GOBUILD) -o $(DEPLOY_RUNNER_BINARY_PATH) ./cmd/deploy-runner

# Development targets
test:
	$(GOTEST) -v ./...

test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

test-race:
	$(GOTEST) -v -race ./...

fmt:
	$(GOFMT) ./...

vet:
	$(GOVET) ./...

lint:
	$(GOLINT) run

ci-setup:
	$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Docker targets
docker-build:
	docker build -t runner-k8s-infra/controller:latest -f Dockerfile.controller .
	docker build -t runner-k8s-infra/deploy-runner:latest -f Dockerfile.deploy-runner .

# Kubernetes targets
apply-namespace:
	kubectl apply -f deploy/namespace.yaml

deploy-controller: build-controller
	kubectl create configmap controller-config --from-file=config/config.yaml -n github-runners --dry-run=client -o yaml | kubectl apply -f -
	# Note: You'll need to create a controller deployment manifest

# Release targets
release-dry-run:
	goreleaser release --snapshot --rm-dist

release:
	goreleaser release --rm-dist

# Cleanup
clean:
	$(GOCLEAN)
	rm -f $(CONTROLLER_BINARY_PATH)
	rm -f $(DEPLOY_RUNNER_BINARY_PATH)

# Dependencies
install-deps:
	$(GOGET) -u ./...
	$(GOCMD) mod tidy

# Help
help:
	@echo "Available targets:"
	@echo "  build              - Build all binaries"
	@echo "  build-controller   - Build controller binary"
	@echo "  build-deploy-runner - Build deploy-runner binary"
	@echo "  test               - Run tests"
	@echo "  test-coverage      - Run tests with coverage"
	@echo "  test-race          - Run tests with race detection"
	@echo "  fmt                - Format code"
	@echo "  vet                - Run go vet"
	@echo "  lint               - Run golangci-lint"
	@echo "  ci-setup           - Install CI dependencies"
	@echo "  docker-build       - Build Docker images"
	@echo "  apply-namespace    - Create kubernetes namespace"
	@echo "  release-dry-run    - Test release build"
	@echo "  release            - Create release"
	@echo "  clean              - Clean build artifacts"
	@echo "  install-deps       - Install/update dependencies"
	@echo "  help               - Show this help message"