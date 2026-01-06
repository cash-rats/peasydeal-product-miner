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
	"  make dev-once URL=<product_url> Crawl one URL on the host (fast loop)" \
	"  make docker-once TARGET_URL=<product_url> Crawl one URL in Docker (parity check)"

.PHONY: dev-chrome
dev-chrome:
	go run ./cmd/devtool chrome

.PHONY: dev-doctor
dev-doctor:
	go run ./cmd/devtool doctor

.PHONY: dev-once
dev-once:
	@if [[ -z "${URL:-}" ]]; then echo "Missing URL. Usage: make dev-once URL=https://shopee.tw/..."; exit 2; fi
	go run ./cmd/devtool once --url "$(URL)"

.PHONY: docker-once
docker-once:
	@if [[ -z "${TARGET_URL:-}" ]]; then echo "Missing TARGET_URL. Usage: make docker-once TARGET_URL=https://shopee.tw/..."; exit 2; fi
	mkdir -p ./out
	TARGET_URL="$(TARGET_URL)" docker compose run --rm --build runner
