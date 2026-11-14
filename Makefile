BIN := alpakr
BUILD_DIR := ./bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: help build run test clean install fmt lint tidy release

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*##"}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Build binary to ./bin/alpakr
	@mkdir -p $(BUILD_DIR)
	go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BIN) .

run: build ## Build and run with CONFIG= (e.g. make run CONFIG=examples/outings/alpakr.yaml)
	$(BUILD_DIR)/$(BIN) run -c $(CONFIG)

test: ## Run all tests
	go test ./...

test-v: ## Run all tests with verbose output
	go test -v ./...

clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)

install: ## Install binary to GOPATH/bin
	go install .

fmt: ## Format code with gofmt
	gofmt -w .

lint: ## Run gofmt check, go vet and golangci-lint (if installed)
	@test -z "$$(gofmt -l .)" || { echo "Files need gofmt:"; gofmt -l .; exit 1; }
	go vet ./...
	@if command -v golangci-lint >/dev/null; then golangci-lint run ./...; else echo "golangci-lint not installed; skipping (CI runs it)"; fi

tidy: ## Tidy go.mod and go.sum
	go mod tidy

# Verifies, tags, and pushes; the Release workflow (release.yaml) then builds
# the binaries and publishes the GitHub release with generated notes.
release: ## Cut a new release (prompts for version, tags, pushes)
	@set -e; \
	git diff --quiet && git diff --cached --quiet || { echo "error: working tree dirty — commit or stash first"; exit 1; }; \
	current=$$(git describe --tags --abbrev=0 2>/dev/null || echo "(none)"); \
	echo "Current version: $$current"; \
	printf "New version (vX.Y.Z): "; read -r version; \
	echo "$$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$$' || { echo "error: invalid version '$$version' (expected vX.Y.Z)"; exit 1; }; \
	if git rev-parse -q --verify "refs/tags/$$version" >/dev/null; then echo "error: tag $$version already exists"; exit 1; fi; \
	$(MAKE) test lint; \
	git tag -a "$$version" -m "$$version"; \
	git push origin HEAD "$$version"; \
	url=$$(git remote get-url origin | sed -E 's#^git@github\.com:#https://github.com/#; s#\.git$$##'); \
	echo "Pushed $$version — the Release workflow is publishing it:"; \
	echo "  $$url/actions/workflows/release.yaml"
