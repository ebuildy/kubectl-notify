BINARY      := kubectl-notify
BIN_DIR     := bin
INSTALL_DIR := $(HOME)/bin
PKG         := github.com/ebuildy/kubectl-notify
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -ldflags "-s -w -X $(PKG)/cmd.version=$(VERSION)"

.DEFAULT_GOAL := build

.PHONY: build
build: ## Build the plugin binary into ./bin
	go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY) .

.PHONY: install
install: build ## Install the plugin to $(INSTALL_DIR) (must be on PATH)
	install -d $(INSTALL_DIR)
	install -m 0755 $(BIN_DIR)/$(BINARY) $(INSTALL_DIR)/$(BINARY)

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run ./...

.PHONY: test
test: ## Run unit tests
	go test ./... -race -count=1

.PHONY: e2e
e2e: ## Run integration/e2e tests
	go test ./test/integration/... -race -count=1

.PHONY: coverage
coverage: ## Generate and open an HTML coverage report
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

.PHONY: tidy
tidy: ## Tidy go.mod / go.sum
	go mod tidy

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) coverage.out

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
