GO := go
PKG := ./...
COVERAGE_FILE := coverage.out
COVERAGE_HTML := coverage.html

# Default goal
.PHONY: all
all: clean build test

# Build the project
.PHONY: build
build:
	$(GO) build -v $(PKG)

# Run tests with coverage
.PHONY: test
test:
	$(GO) test -v -coverprofile=$(COVERAGE_FILE) $(PKG)
	$(GO) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report generated at $(COVERAGE_HTML)"

# Clean up generated files
.PHONY: clean
clean:
	$(GO) clean
	rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)

# Format the code
.PHONY: fmt
fmt:
	$(GO) fmt $(PKG)

# Lint the code
.PHONY: lint
lint:
	golangci-lint run

# Run all checks
.PHONY: check
check: fmt lint test

# Install dependencies
.PHONY: deps
deps:
	$(GO) mod tidy
	$(GO) mod vendor

.PHONY: start
start:
	$(GO) run cmd/server/main.go
