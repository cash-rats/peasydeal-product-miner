# PRD: Tool-Native Skills for Shopee Crawling (Codex + Gemini)

## 1) Summary

This document proposes replacing the current “large prompt template per crawl request” approach with **tool-native workspace skills** and a **short runtime prompt**.

In the new model, the Go program does **not** load `config/prompt.shopee.product.txt` for Shopee crawling. Instead:
- the crawling instructions live in a workspace skill named `shopee-product-crawler`
- the Go program sends a small prompt that references that skill and provides only the URL + hard output constraints
- existing Go-side JSON extraction/repair and `validateContract(...)` remain the final gate

## 2) Problem Statement

Today, each crawl request embeds a large prompt template string (browser control, heuristics, crash recovery, output contract). This causes:
- hard-to-maintain prompt files that are edited frequently and are easy to regress
- larger per-request token payloads and higher interpretation variance
- weaker reuse: the same crawling behavior can’t be easily shared across projects/tools

## 3) Goals

- Make Shopee crawling behavior editable as a **workspace skill** (versioned in-repo).
- Reduce per-request prompt size to a stable “skill invocation” prompt.
- Keep the final output contract unchanged and still enforced in Go.
- Support both tools (Codex CLI + Gemini CLI) in a predictable way.
- Provide a safe, incremental rollout with fallback to the legacy prompt file.

## 4) Non-Goals

- Changing Chrome DevTools MCP usage (still uses `chrome-devtools` MCP; no Shopee API calls).
- Changing the output contract or downstream consumers.
- Building a multi-stage pipeline (single skill invocation only).
- Solving authentication/captcha (assumed handled by a persistent Chrome profile/session).

## 5) Current State (As-Is)

- `internal/runner/runner.go` loads a prompt file (defaults to `config/prompt.shopee.product.txt` for Shopee).
- `loadPrompt(...)` replaces `{{URL}}` and passes the full string to:
  - `internal/runner/codex.go` (Codex CLI)
  - `internal/runner/gemini.go` (Gemini CLI)
- Go parses/extracts JSON and validates the final contract in `internal/runner/contract.go`.

## 6) Proposed Design (To-Be)

### 6.1 Skill packaging

Add a workspace skill in the repo:

- `.agents/skills/shopee-product-crawler/SKILL.md`

Optional support files (only if needed):
- `.agents/skills/shopee-product-crawler/references/...`
- `.agents/skills/shopee-product-crawler/scripts/...`

The skill content should be migrated from `config/prompt.shopee.product.txt` and made **URL-agnostic**:
- do **not** include `{{URL}}`
- assume the runtime prompt provides the URL
- keep the exact output contract requirements (JSON only, one object)

Skill file format:
- `SKILL.md` MUST include YAML frontmatter with at least `name` and `description` (required by Gemini; also supported by Codex).

### 6.2 Runtime prompt (short)

In “skill mode”, Go builds a short prompt:
- explicitly instruct to use the `shopee-product-crawler` skill
- provide `URL: <url>`
- restate the hard output constraints:
  - output exactly one JSON object
  - no markdown fences
  - must satisfy the existing contract validated by Go

This prompt should be stable over time and not embed long extraction instructions.

### 6.3 Tool-specific behavior

Codex CLI:
- Workspace skills are discovered from `.agents/skills/...` when the command runs with the repo as the working directory.
- Requirement: the runner sets `exec.Cmd.Dir` to the repo root so skill discovery does not depend on the caller’s cwd.

Gemini CLI:
- Gemini requires explicit skill install/enable steps.
- Requirement: document (and optionally automate) a one-time workspace setup:
  - install the skill into workspace scope
  - enable the skill by name (`shopee-product-crawler`)
- Requirement: the runner should fail with a clear error if Gemini returns “unknown skill / not enabled”.

### 6.4 Backward-compatible rollout

Introduce a mode flag (config + CLI override) for Shopee crawls:
- `legacy`: existing prompt-file behavior (default initially)
- `skill`: use `shopee-product-crawler` + short runtime prompt

If `skill` mode is enabled but the tool does not have the skill available, the runner should either:
- fail fast with an actionable message, or
- fallback to legacy mode (configurable; default is “fail fast” for correctness).

### 6.5 Environments (docker vs local)

This project runs in two environments with different “skill storage” expectations. Call this out explicitly because it impacts setup and failure modes.

Docker:
- Auth/config and skills are provided via mounted tool home directories:
  - `codex/.codex` mounted to `$HOME/.codex`
  - `gemini/.gemini` mounted to `$HOME/.gemini`
- In this environment, `shopee-product-crawler` is expected to be present (and enabled for Gemini) under the mounted homes (user-scope skills). Skill installation/sync happens on the host before starting the container, or via an entrypoint/setup step.

Local:
- Use project-local skill sources for development iteration (plus any user-installed skills on the machine):
  - Codex: workspace skill in `.agents/skills/...` (requires subprocess working dir = repo root).
  - Gemini: install the same skill either to `--scope workspace` (project-scoped) or `--scope user` (machine-scoped), depending on developer preference.

Source of truth:
- Treat `.agents/skills/shopee-product-crawler` as the canonical, versioned definition.
- Docker/user-scope copies in `codex/.codex` and `gemini/.gemini` are runtime state (gitignored) and should be synced from `.agents/skills/...` to avoid drift.

## 7) Requirements

### 7.1 Skill output contract

`shopee-product-crawler` MUST output exactly one JSON object matching the existing “Shopee product contract”:
- `status`: `ok | needs_manual | error`
- `captured_at`: RFC3339/RFC3339Nano UTC timestamp
- `url` plus required fields based on status
- `price` numeric (number or numeric string)
- `currency` non-empty when `status=ok`

The skill MUST NOT output additional prose, markdown fences, or multiple JSON objects.

### 7.2 Runner behavior

The Go runner MUST:
- keep the current JSON extraction/repair logic (tools sometimes wrap JSON)
- keep `validateContract(...)` as the final gate
- ensure `url`, `captured_at`, and `images` defaults as today
- set tool subprocess working directory to the repo root (deterministic workspace skill resolution)

### 7.3 Developer workflow

Provide documented steps to:
- edit the skill and rerun `devtool once ...` without recompiling skill artifacts
- set up Gemini workspace skill install/enable once per repo checkout

## 8) MVP Plan

1. Add `.agents/skills/shopee-product-crawler/SKILL.md` (migrate prompt content, make URL-agnostic).
2. Add a “skill mode” switch and short runtime prompt generation for Shopee source.
3. Ensure Codex + Gemini runner subprocesses run with repo root as their working directory.
4. Update `README.md` with Gemini skill install/enable instructions.
5. Add a small validation suite (unit test or golden output) asserting that skill mode output passes `validateContract(...)`.

## 9) Success Metrics

Track before/after on the same URL set:
- contract-pass rate
- repair-pass rate (how often Go needs a “repair prompt”)
- average tokens sent per crawl (should drop substantially in skill mode)
- codex vs gemini parity on required fields for `status=ok`

## 10) Risks and Mitigations

- Risk: Gemini skill setup is manual and can be forgotten.
  - Mitigation: add a `devtool skills setup` command (optional) and fail with explicit next steps.

- Risk: Tool updates change skill discovery behavior.
  - Mitigation: keep runtime prompt explicit (“use skill X”) and keep Go contract validation.

- Risk: Skill content drifts from the Go contract.
  - Mitigation: treat `internal/runner/contract.go` as the enforcement point; add a regression test that crawls a known HTML fixture or checks output normalization rules where feasible.

## 11) Open Questions

1. Should skill mode become default for Shopee after a soak period, or remain opt-in?
2. Do we want a separate skill per locale (e.g., Shopee TW vs SG) or a single robust skill?
3. Should the runner preflight-check Gemini skill availability (`gemini skills list`) to produce a better error, or just surface the tool’s error?
