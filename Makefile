.PHONY: build build-controller build-deploy-runner clean test fmt vet install-deps

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

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

fmt:
	$(GOFMT) ./...

vet:
	$(GOVET) ./...

# Kubernetes targets
apply-namespace:
	kubectl apply -f deploy/namespace.yaml

deploy-controller: build-controller
	kubectl create configmap controller-config --from-file=config/config.yaml -n github-runners --dry-run=client -o yaml | kubectl apply -f -
	# Note: You'll need to create a controller deployment manifest

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
	@echo "  fmt                - Format code"
	@echo "  vet                - Run go vet"
	@echo "  apply-namespace    - Create kubernetes namespace"
	@echo "  clean              - Clean build artifacts"
	@echo "  install-deps       - Install/update dependencies"
	@echo "  help               - Show this help message"