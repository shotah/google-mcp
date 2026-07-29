# google-mcp Makefile — Go training wheels for the Python-brained
#
# Run `make` or `make help` to see everything.
# Tip: Go already has great CLI ergonomics; these targets just wrap the common ones.

.DEFAULT_GOAL := help

.PHONY: help fmt vet lint test test-race test-short coverage check \
	build cli install tidy deps clean install-hooks tools run \
	release version

# Build-time version stamp (git describe). Release tags are tracked in ./VERSION.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/shotah/google-mcp/server.ServerVersion=$(VERSION)

# Release bump: patch (default), minor, or major. Or set TAG=v0.2.0 explicitly.
BUMP ?= patch

# Optional: `make test PKG=./tools/...` or `make coverage PKG=./...`
PKG ?= ./...

BINARY := google-mcp
ifeq ($(OS),Windows_NT)
EXE := .exe
else
EXE :=
endif
GOBIN_DIR := $(shell go env GOBIN)
ifeq ($(GOBIN_DIR),)
GOBIN_DIR := $(shell go env GOPATH)/bin
endif

##@ Getting oriented

help: ## Show this help
	@echo.
	@echo Usage:  make ^<target^>
	@echo.
	@echo Getting oriented
	@echo   help                   Show this help
	@echo.
	@echo Daily loop (format -^> lint -^> test)
	@echo   fmt                    Format imports/code (goimports-reviser)
	@echo   vet                    Static analysis (go vet)
	@echo   lint                   Full lint suite (golangci-lint)
	@echo   test                   Unit tests
	@echo   test-short             Unit tests with -short
	@echo   test-race              Unit tests with the race detector
	@echo   coverage               Library coverage report (excludes main)
	@echo   check                  Autofix, lint, and unit tests
	@echo.
	@echo Build ^& run
	@echo   build                  Compile all packages (sanity check)
	@echo   cli                    Build the MCP binary into ./bin/$(BINARY)
	@echo   install                Install binary into GOPATH/bin
	@echo   run                    go run CLI  (make run ARGS="--help")
	@echo.
	@echo Modules ^& cleanup
	@echo   tidy                   Sync go.mod / go.sum with imports
	@echo   deps                   Download module deps
	@echo   clean                  Remove binaries and coverage artifacts
	@echo.
	@echo Project-specific
	@echo   install-hooks          Install git pre-commit (autofix + lint + test)
	@echo   version                Show current VERSION file + latest git tag
	@echo   release                Bump tag, update VERSION, push (BUMP=patch^|minor^|major)
	@echo.
	@echo Tooling
	@echo   tools                  Install goimports-reviser + golangci-lint v2
	@echo.

##@ Daily loop (format → lint → test)

fmt: ## Autofix imports/code (goimports-reviser + golangci-lint fmt/fix)
	goimports-reviser -format -recursive .
	-golangci-lint fmt ./...
	-golangci-lint run --fix ./...

vet: ## Static analysis (go vet) — catches bugs gofmt won't
	go vet ./...

lint: ## Full lint suite (golangci-lint; no write)
	golangci-lint run ./...

test: ## Unit tests (PKG=./path/... for one package)
	go test $(PKG)

test-short: ## Unit tests with -short
	go test -short $(PKG)

test-race: ## Unit tests with the race detector (slower, worth it)
	go test -race $(PKG)

# Default coverage scope excludes CLI mains (cmd/google-mcp, cmd/release).
# Override: make coverage PKG=./...
COVERAGE_PKG ?= ./auth/... ./internal/... ./server/... ./tools/...

coverage: ## Tests + coverage report for library packages (writes coverage.out)
	go test -cover "-coverprofile=coverage.out" -covermode=atomic $(COVERAGE_PKG)
	go tool cover "-func=coverage.out"

check: fmt lint test ## Autofix, lint, test (matches pre-commit)

##@ Build & run

build: ## Compile all packages (sanity check; no binary kept)
	go build ./...

cli: ## Build the MCP binary into ./bin/google-mcp
	mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY)$(EXE) .

install: ## Install binary into $$GOPATH/bin (or $$GOBIN) as google-mcp
	go build -ldflags "$(LDFLAGS)" -o "$(GOBIN_DIR)/$(BINARY)$(EXE)" .

run: ## Build & run CLI — e.g. make run ARGS="--help"
	go run -ldflags "$(LDFLAGS)" . $(ARGS)

##@ Modules & cleanup

tidy: ## Sync go.mod / go.sum with imports (python: pip freeze vibes)
	go mod tidy

deps: ## Download module deps into the module cache
	go mod download

clean: ## Remove built binaries and coverage artifacts
	go clean ./...
ifeq ($(OS),Windows_NT)
	-cmd /C "rmdir /S /Q bin 2>NUL & del /Q $(BINARY) $(BINARY).exe coverage coverage.out coverage.txt 2>NUL"
else
	rm -rf bin
	rm -f $(BINARY) $(BINARY).exe coverage coverage.out coverage.txt
endif

##@ Project-specific

install-hooks: ## Install git pre-commit hook (autofix + lint + test)
ifeq ($(OS),Windows_NT)
	copy /Y scripts\pre-commit .git\hooks\pre-commit
else
	cp scripts/pre-commit .git/hooks/pre-commit
	chmod +x .git/hooks/pre-commit
endif
	@echo "Installed .git/hooks/pre-commit"

version: ## Show VERSION file and latest git tag / next patch
	@go run ./cmd/release -dry-run

# Bump semver, commit VERSION, annotated-tag, push HEAD + tag (triggers GoReleaser).
# Examples:
#   make release
#   make release BUMP=minor
#   make release BUMP=major
#   make release TAG=v0.2.0
#   make release DRY_RUN=1
release: ## Bump version tag, update VERSION, push (BUMP=patch|minor|major)
	go run ./cmd/release \
		$(if $(TAG),-version=$(TAG),-bump=$(BUMP)) \
		$(if $(DRY_RUN),-dry-run,) \
		$(if $(SKIP_PUSH),-skip-push,) \
		$(if $(ALLOW_DIRTY),-allow-dirty,)

##@ Tooling (skip if you use nix develop / direnv)

tools: ## Install goimports-reviser + golangci-lint v2 into $$GOBIN
	go install github.com/incu6us/goimports-reviser/v3@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@echo Installed tools. Ensure GOPATH/bin is on PATH, then: golangci-lint version
