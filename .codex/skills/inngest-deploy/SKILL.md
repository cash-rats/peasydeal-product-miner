---
name: inngest-deploy
description: Deployment-focused guidance for Inngest-based services. Use when asked about deployment scripts, env files, build/push, or release workflow; ignore connectivity/runtime modes.
---

# Inngest Deploy

## Scope
- Focus only on deployment workflows: build/push, env files, compose files, remote deploy steps.
- Ignore connectivity/runtime modes (e.g., websocket connect vs HTTP serving).

## Quick start
1) Locate deployment entrypoints:
   - `scripts/deploy.sh`, `Makefile`, CI pipelines, `docker-compose*.yaml`.
2) Inspect env files:
   - `.env.deploy.*` (local deploy config)
   - `.env.prod.*` (runtime env shipped to server)

## Required checks
- Ensure required Inngest env keys exist in `.env.prod.*` (e.g., `INNGEST_APP_ID`, `INNGEST_SIGNING_KEY`, `INNGEST_EVENT_KEY` if used).
- Ensure registry credentials exist for image pull/push (GHCR `GHCR_USER`/`GHCR_TOKEN`).
- If `docker-compose.yml` references `GHCR_USER` in image tags, ensure `GHCR_USER` is present in `.env.prod.*` (runtime env on server).
- Ensure remote path/host/SSH details are provided in `.env.deploy.*`.

## Env example
`.env.deploy.<machine>`:
```env
PROD_DIR=/home/ubuntu/app/my-service
PROD_HOST=example.com
PROD_USER=ubuntu
PROD_PORT=22
PROD_SSH_KEY_PATH=/path/to/id_rsa
GHCR_USER=your-ghcr-username
GHCR_TOKEN=ghcr_token_here
```

`.env.prod.<machine>`:
```env
PORT=3010
PYTHON_ENV=production
INNGEST_APP_ID=...
INNGEST_EVENT_KEY=...
INNGEST_SIGNING_KEY=...
```

## Workflow guidance
- If a deploy script exists, follow it and describe its flags/targets instead of inventing new steps.
- If build/push is optional, highlight the default policy and how to enable build/push.
- If the deploy flow uploads env files to the server, call out the remote filename and path.

## Template script
- A reusable deploy template is provided at `scripts/deploy.sh`.
- Adapt `COMPOSE_FILE`, `SERVICE_NAME`, registry login variables, and required env keys for the project.
- If your deploy uses a non-22 SSH port, ensure the script uses `ssh -p` and `scp -P` (note the case difference).
- Template deploys run `docker system prune -af` after `up` to clear unused Docker resources.

## What to ask the user
- Target machine(s) or environment.
- Whether to build/push or pull-only.
- Which `.env.prod.*` and `.env.deploy.*` to use.

## Expected output
- Summarize the deploy flow with concrete commands from the repo.
- List any missing env keys or mismatches between `.env.deploy.*` and `.env.prod.*`.
