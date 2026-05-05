BINARY       := ccs-cli
PKG          := github.com/mingyuans/claude-profile-switch
BIN_DIR      := bin
DIST_DIR     := dist
INSTALL_DIR  ?= /usr/local/bin
VERSION      ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS      := -s -w -X $(PKG)/cmd.version=$(VERSION)
RELEASE_TARGETS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64

.PHONY: help install dev build test lint fmt vet clean run setup uninstall release release-clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Install/refresh Go module dependencies
	go mod tidy

dev: build ## Build then run `list` (quick smoke)
	./$(BIN_DIR)/$(BINARY) list

build: $(BIN_DIR) ## Build binary into bin/
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) ./

$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

test: ## Run all unit tests
	go test ./...

lint: fmt vet ## Run gofmt + go vet

fmt: ## Format Go source files
	gofmt -s -w .

vet: ## Run go vet
	go vet ./...

clean: ## Remove build and dist artifacts
	rm -rf $(BIN_DIR) $(DIST_DIR)

release: release-clean ## Cross-compile release tarballs into dist/ (darwin/linux × amd64/arm64)
	@mkdir -p $(DIST_DIR)
	@for target in $(RELEASE_TARGETS); do \
		os=$${target%%/*}; arch=$${target##*/}; \
		echo "  ▸ building $$os-$$arch"; \
		GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY) ./; \
		tar -czf $(DIST_DIR)/$(BINARY)-$$os-$$arch.tar.gz -C $(DIST_DIR) $(BINARY); \
		rm $(DIST_DIR)/$(BINARY); \
	done
	@echo ""
	@echo "  ✓ artifacts:"
	@ls -1 $(DIST_DIR)

release-clean: ## Remove dist/ artifacts
	rm -rf $(DIST_DIR)

run: ## Run via `go run` (forwards ARGS, e.g. `make run ARGS="list"`)
	go run . $(ARGS)

setup: build ## Build and print shell-integration setup hint
	@echo ""
	@echo "Built $(BIN_DIR)/$(BINARY)"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Copy to PATH:    sudo cp $(BIN_DIR)/$(BINARY) $(INSTALL_DIR)/"
	@echo "  2. Source in shell: echo 'eval \"\$$($(BINARY) init zsh)\"' >> ~/.zshrc"
	@echo "  3. Reload shell:    exec zsh"
	@echo ""

uninstall: ## Remove installed binary
	rm -f $(INSTALL_DIR)/$(BINARY)
