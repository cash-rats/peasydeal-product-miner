SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

# AI tool selection for dev-once (codex or gemini).
TOOL ?= codex

# Load `.env` for developer convenience (Docker Compose already reads it).
ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: help
help:
	@printf "%s\n" \
	"Targets:" \
	"  make start                     Start long-lived HTTP server (/health)" \
	"  make dev-chrome                 Start Chrome with DevTools enabled" \
	"  make dev-doctor                 Check DevTools is reachable on localhost" \
	"  make dev-once TOOL=codex|gemini <product_url>  Crawl one URL on the host (fast loop)" \
	"  make docker-doctor              Check Chrome + Codex auth for Docker runs" \
	"  make docker-once <product_url>  Crawl one URL in Docker (parity check)" \
	"  make docker-shell               Open a shell in the runner container (useful for debugging)" \
	"  make docker-login               Authorize host Codex for the Docker runner"

.PHONY: start
start:
	go run ./cmd/server

## start/inngest: start the inngest dev server
.PHONY: start/inngest
INNGEST_SERVE_HOST ?= localhost:3012
start/inngest:
	npx inngest-cli@latest dev \
		--no-discovery \
		--poll-interval 10000 \
		-u http://$(INNGEST_SERVE_HOST)/api/inngest

.PHONY: dev-chrome
dev-chrome:
	go run ./cmd/devtool chrome

.PHONY: dev-doctor
dev-doctor:
	go run ./cmd/devtool doctor

.PHONY: dev-once
dev-once: dev-doctor
	@URL="$(word 2,$(MAKECMDGOALS))"; \
	go run ./cmd/devtool once --tool "$(TOOL)" --url "$$URL"

.PHONY: docker-once
docker-once: docker-doctor
	@URL="$(word 2,$(MAKECMDGOALS))"; \
	if [[ -z "$$URL" ]]; then echo "Missing URL. Usage: make docker-once https://shopee.tw/..."; exit 2; fi; \
	docker compose run --rm --build -e TARGET_URL="$$URL" runner

.PHONY: docker-doctor
docker-doctor:
	go run ./cmd/devtool docker-doctor

.PHONY: docker-codex-login
docker-codex-login:
	@if [[ -f "/.dockerenv" ]]; then echo "Run this on the host (not inside Docker), so your browser can reach the local callback server."; exit 2; fi
	mkdir -p ./codex
	@codex_cmd="$${CODEX_CMD:-codex}"; \
	codex_bin="$$(command -v "$$codex_cmd" || true)"; \
	if [[ -z "$$codex_bin" ]]; then echo "codex not found in PATH (or CODEX_CMD). Try: CODEX_CMD=/full/path/to/codex make docker-login"; exit 127; fi; \
	echo "Using Codex: $$codex_bin"; \
	HOME="$(CURDIR)/codex" "$$codex_bin" login

.PHONY: docker-gemini-login
docker-gemini-login:
	@if [[ -f "/.dockerenv" ]]; then echo "Run this on the host (not inside Docker), so your browser can reach the local callback server."; exit 2; fi
	mkdir -p ./gemini
	@gemini_cmd="$${GEMINI_CMD:-gemini}"; \
	gemini_bin="$$(command -v "$$gemini_cmd" || true)"; \
	if [[ -z "$$gemini_bin" ]]; then echo "gemini not found in PATH (or GEMINI_CMD). Try: GEMINI_CMD=/full/path/to/gemini make docker-gemini-login"; exit 127; fi; \
	echo "Using Gemini: $$gemini_bin"; \
	echo "Please check if your Gemini CLI requires authentication or is already authenticated."; \
	HOME="$(CURDIR)/gemini" "$$gemini_bin"
