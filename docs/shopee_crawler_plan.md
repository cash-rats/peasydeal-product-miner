# Deployable Shopee Crawler (Option A): Docker Runner + Host Chrome (DevTools) + Codex CLI + Chrome DevTools MCP

> Goal: A deployable workflow where **each host machine runs a dedicated Chrome profile** (pre-authenticated session) exposed via **DevTools remote debugging**, while a **Docker container** runs a scheduled crawler using **Codex CLI + Chrome DevTools MCP** to open product pages and extract data into JSON files.

## 0) Developer experience goals (don’t skip)

This plan optimizes for deployment, but it must also be easy to develop. The intended dev loop:

- **Start Chrome once** (dedicated profile + DevTools port).
- Edit prompt/schema/URL list in plain files.
- Run **one URL** quickly (local runner) and iterate.
- Run the same logic in Docker with the same config files (no “dev-only” code paths).

To support that, keep all “runtime inputs” in versioned files:
- prompt: `config/prompt.product.txt`
- schema/contract: `config/schema.product.json`
- URLs: `config/urls.txt`

---

## 1) High-level architecture

### Components

**On the host (your laptop/desktop):**
- **Google Chrome** launched with:
  - `--remote-debugging-port=9222`
  - `--user-data-dir=<dedicated_profile_dir>` (non-default)
- The dedicated profile holds:
  - Shopee login session
  - any manual CAPTCHA validation you complete

**In Docker (portable across machines):**
- **Runner service** (Go) that:
  - runs on an interval (e.g., every 30 minutes)
  - calls `codex exec` non-interactively
  - writes crawl results as JSON files to a mounted `/out` directory
- **Codex CLI** configured with an MCP server:
  - `chrome-devtools-mcp` launched via `npx`
  - configured to connect to the host Chrome endpoint:
    - `http://host.docker.internal:9222` (Docker Desktop)

---

## 2) Key constraints (must understand)

### 2.1 Chrome 136+ remote debugging requires non-default user data dir

From Chrome 136 onward, `--remote-debugging-port` is **not respected** when attempting to debug the default Chrome data directory. You **must** provide `--user-data-dir` pointing to a **non-standard** directory (it uses a different encryption key and helps protect user data).

**Implication:** always use a dedicated profile directory for the crawler.

### 2.2 Docker-to-host connectivity uses `host.docker.internal` (Docker Desktop)

On macOS/Windows with Docker Desktop, containers can reach services on the host using:
- `host.docker.internal`

On macOS/Windows with Docker Desktop, containers can reach services on the host using the special hostname:
- `host.docker.internal`

**Implication:** your MCP server inside Docker should connect to:
- `http://host.docker.internal:9222`

### 2.3 CAPTCHA is not reliably automatable

Your workflow can be automated **while the session is valid**, but occasionally Shopee may require manual verification again. The runner should:
- detect verify/login walls
- emit `status="needs_manual"`
- back off / alert

---

## 3) Host machine setup (per computer)

### 3.1 Create a dedicated Chrome profile directory

Choose something like:
- macOS: `~/chrome-mcp-profiles/shopee`

This directory should be **only** for crawling (not your daily browsing profile).

### 3.2 Start Chrome with DevTools port + dedicated profile

**macOS example:**
```bash
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome \
  --remote-debugging-port=9222 \
  --user-data-dir="$HOME/chrome-mcp-profiles/shopee"
```

### 3.3 Verify DevTools is reachable on the host

Open (in any browser) on the host:
- `http://127.0.0.1:9222/json/version`

If Chrome is correctly running in debug mode, you’ll see JSON.

### 3.4 Login to Shopee once (manual)

In that Chrome instance:
- open `https://shopee.tw/`
- log in
- solve CAPTCHA if required

Leave Chrome running while the Docker runner works, or restart Chrome later using the same profile directory.

---

## 4) Docker stack design (deployable)

### 4.1 What the container needs

- Your **runner binary/script** (Go preferred)
- **Codex CLI**
- **Node.js** (for `npx chrome-devtools-mcp@latest`)
- Mounted volumes for:
  - `/out` (crawl results)
  - `/codex` (Codex config + auth state, optional but recommended)
  - `/app/config` (schemas, prompts, URL lists)

### 4.2 Codex configuration inside container

Codex stores config at `~/.codex/config.toml`.

**Strategy:** mount a folder so that inside container:
- `HOME=/codex`
- config file lives at: `/codex/.codex/config.toml`

#### Minimal MCP server entry (connect to host Chrome)
Example `config.toml` concept:

- MCP server: `chrome-devtools-mcp`
- command: `npx -y chrome-devtools-mcp@latest --browser-url=http://host.docker.internal:9222`

This makes Codex call the MCP server over stdio; the MCP server then connects to host Chrome.

### 4.3 Dev mode (recommended): local runner + local Codex, same config

Developers should be able to iterate without Docker:

- Start host Chrome with DevTools enabled (same as deployment).
- Run a local runner script that calls `codex exec`.
- Use the same `config/*` files as the deployable stack.

This reduces “container debugging” while you’re still iterating on:
- prompt wording
- page readiness waits
- extraction fields
- verify/login wall detection

When ready, switch to Docker to validate deployment parity.

---

## 5) Runner behavior (Go-first implementation strategy)

### 5.1 Core loop responsibilities

On each interval tick:
1. Select the next URL(s) to crawl
   - MVP: a hardcoded URL
   - Next: read from a file list, folder queue, or DB
2. Execute Codex non-interactively:
   - `codex exec <prompt>`
3. Require structured output:
   - validate output with a JSON schema
4. Persist result to `/out/<timestamp>_<id>.json`
5. If `status == needs_manual`:
   - stop further crawling or exponential backoff
   - log clearly that host Chrome requires manual intervention

### 5.2 Stable output contract (JSON schema)

Create a schema file, e.g. `/app/config/schema.product.json`:

- `url` (string, required)
- `status` (enum: `ok | needs_manual | error`, required)
- `title`, `price`, `currency`, `shop_name`
- `error` (string)

**Benefit:** your runner can consume results reliably without fragile string parsing.

### 5.3 Prompt template (predefined context)

Keep a versioned prompt file like `/app/config/prompt.txt` that instructs the agent to:

- open product URL
- wait for page readiness
- extract: title/price/currency/shop name
- detect “verify/captcha/login” and return `needs_manual` with explanation
- output **only JSON** matching the schema

### 5.4 Concurrency & robustness

- Single-run lock (avoid overlapping runs)
- Timeouts (kill `codex exec` if stuck)
- Retry policy for transient navigation errors
- Backoff policy for `needs_manual`

---

## 6) Example project layout (recommended)

```
README.md
docker-compose.yml
Dockerfile
internal/runner/
  runner.go
config/
  schema.product.json
  prompt.product.txt
  urls.txt
codex/
  .codex/
    config.toml
    auth.json        # (optional) if you want auth persisted
out/                # mounted output dir
cmd/devtool/
  main.go           # chrome/doctor/once helper commands
```

---

## 7) Deployment workflow (per new computer)

### 7.1 One-time steps on the new host
1. Install Chrome
2. Create dedicated profile dir
3. Start Chrome with debug port + profile
4. Log in to Shopee manually

### 7.2 Container steps
1. Copy the repo (or deploy artifact)
2. `docker compose up -d`
3. Watch logs; verify JSON files are being written to `/out`

---

## 8) Developer quickstart (new dev machine)

1) Start the dedicated Chrome profile (DevTools on `9222`)
- `make dev-chrome`

2) Verify DevTools is reachable
- `make dev-doctor`

3) Run a single URL locally (fastest iteration loop)
- `make dev-once URL="https://shopee.tw/..."` (writes a JSON file into `out/`)

4) Run the deployable stack (parity check)
- `cp .env.example .env` (edit `TARGET_URL` as needed)
- `docker compose up --build`

---

## 9) Security guidance (practical)

- **Never expose** port 9222 to your LAN/Internet.
- Treat the debug session as highly privileged:
  - it can inspect and control browser content
- Use a dedicated profile with minimal saved credentials.

---

## 10) Troubleshooting checklist

### “Codex can’t connect to Chrome”
- Host Chrome not started with `--remote-debugging-port`
- Missing required `--user-data-dir` on Chrome 136+
- Port 9222 already in use
- Container cannot reach host Chrome
  - ensure MCP browser URL is `http://host.docker.internal:9222`

### “Crawl output says needs_manual”
- Shopee session expired
- CAPTCHA/verify wall triggered
- Fix by manually solving in host Chrome profile, then next interval should proceed.

---

## 11) Next extensions (after MVP)

- URL queue:
  - local file queue
  - Redis
  - Postgres (Supabase alternative if quota issues)
- Dedup + state tracking:
  - avoid re-crawling unchanged products
- Alerting:
  - Slack webhook when `needs_manual`
- Multi-machine scale-out:
  - one host Chrome per machine
  - same docker stack deployed across machines

---

## References (for implementation notes)
- Chrome security change: remote debugging requires non-default `--user-data-dir` (Chrome 136+)
- Docker Desktop: host connectivity via `host.docker.internal`
- Codex CLI: `~/.codex/config.toml` and MCP server configuration
- `chrome-devtools-mcp`: `--browser-url` option for connecting to an existing Chrome instance
