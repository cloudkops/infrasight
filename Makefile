# Variables
BINARY_NAME := infrasight
BUILD_DIR   := bin
GO          := go
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)


# Default target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build              Build the binary into $(BUILD_DIR)/$(BINARY_NAME)"
	@echo "  run                Build and run (pass ARGS=\"...\" for CLI args)"
	@echo "  install            Install the binary into \$$GOBIN or \$$GOPATH/bin"
	@echo "  test               Run unit tests"
	@echo "  coverage           Run tests with coverage report"
	@echo "  lint               Run golangci-lint"
	@echo "  tidy               Add missing and remove unused modules"
	@echo "  clean              Remove build artifacts"

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: build
build:
	CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)

.PHONY: run
run: build
	@ARGS="$(filter-out $@,$(MAKECMDGOALS))"; \
	./$(BUILD_DIR)/$(BINARY_NAME) $$ARGS

.PHONY: install
install: build
	$(GO) install -trimpath -ldflags "-s -w" .

.PHONY: test
test:
	$(GO) test ./...

.PHONY: coverage
coverage:
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report written to coverage.html"

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: clean
clean:
	@rm -rf $(BUILD_DIR) coverage.out coverage.html
