APP      := vault-note-cli
MODULE   := github.com/hmanduca/vault-note-cli
GO       ?= go
PREFIX   ?= $(HOME)/.local
SKILL_DIR ?= $(HOME)/.gemini/antigravity-cli/skills/$(APP)

VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE     := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS  := -s -w \
	-X '$(MODULE)/internal/buildinfo.Version=$(VERSION)' \
	-X '$(MODULE)/internal/buildinfo.Commit=$(COMMIT)' \
	-X '$(MODULE)/internal/buildinfo.Date=$(DATE)'

BUILD    := CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)"

.DEFAULT_GOAL := help

.PHONY: help build build-all checksums run test test-cover fmt vet lint check clean install uninstall install-skill

help: ## Lista os alvos disponíveis
	@awk 'BEGIN {FS = ":.*## "; printf "\nUso:\n  make <alvo>\n\nAlvos:\n"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Compila binário estático em bin/vault-note-cli
	@mkdir -p bin
	$(BUILD) -o bin/$(APP) ./cmd/$(APP)

build-all: ## Cross-compilation de release para Linux, macOS e Windows em dist/
	@mkdir -p dist
	GOOS=linux   GOARCH=amd64          $(BUILD) -o dist/$(APP)-linux-amd64   ./cmd/$(APP)
	GOOS=linux   GOARCH=arm64          $(BUILD) -o dist/$(APP)-linux-arm64   ./cmd/$(APP)
	GOOS=linux   GOARCH=arm   GOARM=7  $(BUILD) -o dist/$(APP)-linux-armv7   ./cmd/$(APP)
	GOOS=darwin  GOARCH=amd64          $(BUILD) -o dist/$(APP)-darwin-amd64  ./cmd/$(APP)
	GOOS=darwin  GOARCH=arm64          $(BUILD) -o dist/$(APP)-darwin-arm64  ./cmd/$(APP)
	GOOS=windows GOARCH=amd64          $(BUILD) -o dist/$(APP)-windows-amd64.exe ./cmd/$(APP)

checksums: ## Gera dist/checksums.txt com SHA-256 dos binários
	@cd dist && sha256sum $(APP)-* > checksums.txt

run: ## Executa a partir do código-fonte usando banco de configurações temporário
	VAULT_NOTE_CLI_DB=/tmp/$(APP)-dev.db $(GO) run ./cmd/$(APP) $(ARGS)

test: ## Executa suíte de testes com detector de race e sem cache
	$(GO) test -race -count=1 ./...

test-cover: ## Executa testes com relatório de cobertura HTML
	$(GO) test -race -count=1 -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

fmt: ## Formata o código-fonte com gofmt
	@gofmt -s -w .

vet: ## Executa go vet
	$(GO) vet ./...

lint: ## Executa golangci-lint (se instalado)
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint não encontrado; executando go vet como fallback"; \
		$(GO) vet ./...; \
	fi

check: fmt vet test ## Gate completo pré-commit (fmt + vet + test -race)

clean: ## Remove artefatos de build e cobertura
	rm -rf bin dist coverage.out coverage.html

install: build ## Instala binário em $(PREFIX)/bin (padrão ~/.local/bin)
	@mkdir -p $(PREFIX)/bin
	install -m 755 bin/$(APP) $(PREFIX)/bin/$(APP)

uninstall: ## Remove binário de $(PREFIX)/bin
	rm -f $(PREFIX)/bin/$(APP)

install-skill: ## Instala skill de agente em $(SKILL_DIR)
	@mkdir -p $(SKILL_DIR)
	install -m 644 skill/$(APP).md $(SKILL_DIR)/SKILL.md
