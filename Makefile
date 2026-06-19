BIN := alpakr
BUILD_DIR := ./bin

.PHONY: help build run test clean install lint

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*##"}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Build binary to ./bin/alpakr
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BIN) .

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

lint: ## Run go vet
	go vet ./...

tidy: ## Tidy go.mod and go.sum
	go mod tidy
