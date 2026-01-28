SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

# AI tool selection for dev-once (codex or gemini).
tool ?= codex
# URL to crawl for dev-once (required).
url ?=

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
	"  make worker                    Start RabbitMQ crawl worker (AMQP consumer)" \
	"  make dev-chrome                 Start Chrome with DevTools enabled" \
	"  make dev-doctor                 Check DevTools is reachable on localhost" \
	"  make devtool-build              Build Linux devtool binary (out/devtool-linux-amd64)" \
	"  make devtool-upload env=<name> [dest=<remote_path>]  Upload devtool binary to server" \
	"  make devtool-deploy env=<name> [dest=<remote_path>]  Build + upload devtool to server" \
	"  make auth-upload env=<name> [auth_tool=codex|gemini|both]  Upload tool auth/config to server" \
	"  make dev-once tool=codex|gemini url=<product_url>  Crawl one URL on the host (fast loop)" \
	"  make docker-doctor tool=codex|gemini  Check Chrome + tool auth for Docker runs" \
	"  make docker-once tool=codex|gemini url=<product_url>  Crawl one URL in Docker (parity check)" \
	"  make docker-shell               Open a shell in the runner container (useful for debugging)" \
	"  make docker-login tool=codex|gemini  Authorize host tool for the Docker runner" \
	"  make ghcr-login               Login to GHCR using GHCR_USER and GHCR_TOKEN from .env" \
	"  make deploy env=<name> build=1        Deploy to remote server via scripts/deploy.sh" \
	"  make goose-create name=<migration_name>  Create a goose SQL migration in db/migrations"

.PHONY: start
start:
	go run ./cmd/server

.PHONY: worker
worker:
	go run ./cmd/worker

## start/inngest: start the inngest dev server
.PHONY: start/inngest
INNGEST_SERVE_HOST ?= localhost:3012
start/inngest:
	npx inngest-cli@latest dev \
		--no-discovery \
		--poll-interval 10000 \
		-u http://$(INNGEST_SERVE_HOST)$(INNGEST_SERVE_PATH)

.PHONY: dev-chrome
dev-chrome:
	go run ./cmd/devtool chrome

.PHONY: dev-doctor
dev-doctor:
	go run ./cmd/devtool doctor

.PHONY: devtool-build
DEVTOOL_BIN ?= out/devtool-linux-amd64
devtool-build:
	@mkdir -p "$(dir $(DEVTOOL_BIN))"
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" -o "$(DEVTOOL_BIN)" ./cmd/devtool
	@echo "Built: $(DEVTOOL_BIN)"

.PHONY: devtool-upload
devtool-upload:
	@if [ -z "$(env)" ]; then \
		echo "Error: Missing env name."; \
		echo "Usage: make devtool-upload env=<name> [dest=/path/on/server/devtool]"; \
		exit 1; \
	fi
	@dest="$(strip $(dest))"; \
	args=(); \
	if [ -n "$$dest" ]; then args+=(--dest "$$dest"); fi; \
	bash ./scripts/deploy-devtool.sh "$(env)" --bin "$(DEVTOOL_BIN)" "$${args[@]}"

.PHONY: devtool-deploy
devtool-deploy: devtool-build devtool-upload

.PHONY: dev-once
dev-once: dev-doctor
	@URL="$(strip $(url))"; \
	if [[ -z "$$URL" ]]; then echo "Missing URL. Usage: make dev-once url=https://shopee.tw/..."; exit 2; fi; \
	go run ./cmd/devtool once --tool "$(tool)" --url "$$URL"

.PHONY: docker-once
docker-once: docker-doctor
	@URL="$(strip $(url))"; \
	if [[ -z "$$URL" ]]; then echo "Missing URL. Usage: make docker-once url=https://shopee.tw/..."; exit 2; fi; \
	docker compose run --rm --build runner /app/devtool once --tool "$(tool)" --url "$$URL" --out-dir /out

.PHONY: docker-doctor
docker-doctor:
	go run ./cmd/devtool docker-doctor --tool "$(tool)"

.PHONY: docker-shell
docker-shell:
	docker compose run --rm --build runner sh

.PHONY: docker-login
docker-login:
	@case "$(tool)" in \
		codex) $(MAKE) docker-codex-login ;; \
		gemini) $(MAKE) docker-gemini-login ;; \
		*) echo "Error: unknown tool '$(tool)' (expected codex or gemini)"; exit 2 ;; \
	esac

.PHONY: docker-codex-login
docker-codex-login:
	@if [[ -f "/.dockerenv" ]]; then echo "Run this on the host (not inside Docker), so your browser can reach the local callback server."; exit 2; fi
	mkdir -p ./codex
	@codex_cmd="$${CODEX_CMD:-codex}"; \
	codex_bin="$$(command -v "$$codex_cmd" || true)"; \
	if [[ -z "$$codex_bin" ]]; then echo "codex not found in PATH (or CODEX_CMD). Try: CODEX_CMD=/full/path/to/codex make docker-codex-login"; exit 127; fi; \
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
	HOME="$(CURDIR)/gemini" "$$gemini_bin" --prompt "Just Return exactly: OK"

.PHONY: auth-upload
auth_tool ?= both
auth-upload:
	@if [ -z "$(env)" ]; then \
		echo "Error: Missing env name."; \
		echo "Usage: make auth-upload env=<name> [auth_tool=codex|gemini|both]"; \
		exit 1; \
	fi
	@bash ./scripts/deploy-auth.sh "$(env)" --tool "$(auth_tool)"

.PHONY: ghcr-login
ghcr-login:
	@if [ -z "$$GHCR_USER" ]; then \
		echo "Error: Missing GHCR_USER (set in .env)."; \
		exit 1; \
	fi
	@if [ -z "$$GHCR_TOKEN" ]; then \
		echo "Error: Missing GHCR_TOKEN (set in .env)."; \
		exit 1; \
	fi
	@echo "$$GHCR_TOKEN" | docker login ghcr.io -u "$$GHCR_USER" --password-stdin

.PHONY: deploy
deploy:
	@if [ -z "$(env)" ]; then \
		echo "Error: Missing env name."; \
		echo "Usage: make deploy env=<name> [build=1]"; \
		exit 1; \
	fi
	@if [ "$(build)" = "1" ]; then \
		./scripts/deploy.sh "$(env)" --build; \
	else \
		./scripts/deploy.sh "$(env)"; \
	fi

.PHONY: goose-create
goose-create:
	@if [ -z "$(name)" ]; then \
		echo "Error: Missing migration name."; \
		echo "Usage: make goose-create NAME=add_users_table"; \
		exit 1; \
	fi
	@mkdir -p db/migrations
	goose -dir db/migrations create "$(name)" sql


.PHONY: goose
goose:
	@if [ -z "$(cmd)" ]; then \
		echo "Error: Missing goose command name."; \
		echo "Usage: make goose cmd=status"; \
		exit 1; \
	fi
	@mkdir -p db/migrations
	dotenvx run -f .env -- go run cmd/migrate/main.go $(cmd)

.PHONY: schema-dump
schema-dump:
	@mkdir -p db
	@echo "Turso exporting Schema..."
	turso db shell peasydeal ".schema" > db/schema.sql
	@echo "complete！exported to db/schema.sql"
