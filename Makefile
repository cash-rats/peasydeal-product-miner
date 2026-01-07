SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

# Load `.env` for developer convenience (Docker Compose already reads it).
ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: help
help:
	@printf "%s\n" \
	"Targets:" \
	"  make dev-chrome                 Start Chrome with DevTools enabled" \
	"  make dev-doctor                 Check DevTools is reachable on localhost" \
	"  make dev-once <product_url>     Crawl one URL on the host (fast loop)" \
	"  make docker-doctor              Check Chrome + Codex auth for Docker runs" \
	"  make docker-once <product_url>  Crawl one URL in Docker (parity check)" \
	"  make docker-shell               Open a shell in the runner container (useful for debugging)" \
	"  make docker-login               Authorize host Codex for the Docker runner"

.PHONY: dev-chrome
dev-chrome:
	go run ./cmd/devtool chrome

.PHONY: dev-doctor
dev-doctor:
	go run ./cmd/devtool doctor

.PHONY: dev-once
dev-once:
	@URL="$(word 2,$(MAKECMDGOALS))"; \
	go run ./cmd/devtool once --url "$$URL"

.PHONY: docker-once
docker-once: docker-doctor
	@URL="$(word 2,$(MAKECMDGOALS))"; \
	if [[ -z "$$URL" ]]; then echo "Missing URL. Usage: make docker-once https://shopee.tw/..."; exit 2; fi; \
	docker compose run --rm --build -e TARGET_URL="$$URL" runner

.PHONY: docker-doctor
docker-doctor:
	go run ./cmd/devtool docker-doctor

.PHONY: docker-shell
docker-shell:
	docker compose run --rm runner sh

.PHONY: docker-login
docker-login:
	@if [[ -f "/.dockerenv" ]]; then echo "Run this on the host (not inside Docker), so your browser can reach the local callback server."; exit 2; fi
	mkdir -p ./codex
	@codex_cmd="$${CODEX_CMD:-codex}"; \
	codex_bin="$$(command -v "$$codex_cmd" || true)"; \
	if [[ -z "$$codex_bin" ]]; then echo "codex not found in PATH (or CODEX_CMD). Try: CODEX_CMD=/full/path/to/codex make docker-login"; exit 127; fi; \
	echo "Using Codex: $$codex_bin"; \
	HOME="$(CURDIR)/codex" "$$codex_bin" login
