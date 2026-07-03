MODULE      := github.com/cloudkops/infrasight
BINARY_NAME := infrasight
BUILD_DIR   := bin
MAIN_PKG    := ./cmd/infrasight
GO          := go

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -s -w \
	-X '$(MODULE)/internal/app.version=$(VERSION)' \
	-X '$(MODULE)/internal/app.commit=$(COMMIT)'

.DEFAULT_GOAL := help

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build       Build $(BUILD_DIR)/$(BINARY_NAME)"
	@echo "  run         Build and run (make run ARGS=\"scan kubernetes -n default\")"
	@echo "  install     Install into \$$GOBIN or \$$GOPATH/bin"
	@echo "  fmt         Run gofmt -w on the whole tree"
	@echo "  fmt-check   Fail if gofmt would change any file"
	@echo "  vet         Run go vet"
	@echo "  test        Run unit tests"
	@echo "  test-race   Run unit tests with the race detector"
	@echo "  coverage    Run tests with a coverage report (coverage.html)"
	@echo "  lint        Run golangci-lint (must be installed separately)"
	@echo "  tidy        Run go mod tidy"
	@echo "  ci          fmt-check + vet + test-race + build (what CI should run)"
	@echo "  clean       Remove build artifacts and coverage output"

.PHONY: build
build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PKG)

.PHONY: run
run: build
	./$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

.PHONY: install
install:
	$(GO) install -trimpath -ldflags "$(LDFLAGS)" $(MAIN_PKG)

.PHONY: fmt
fmt:
	gofmt -w .

.PHONY: fmt-check
fmt-check:
	@files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then \
		echo "gofmt needed on:"; echo "$$files"; exit 1; \
	fi

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: test
test:
	$(GO) test ./...

.PHONY: test-race
test-race:
	$(GO) test -race ./...

.PHONY: coverage
coverage:
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report written to coverage.html"

.PHONY: lint
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not found — install: https://golangci-lint.run/welcome/install/"; exit 1; \
	}
	golangci-lint run ./...

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: ci
ci: fmt-check vet test-race build

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html
