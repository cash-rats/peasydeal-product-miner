# Deployment Plan: GHCR Image + Remote Pull (+ Devtool)

Goal: remove manual SSH + git pull + build; deploy by pushing an image to GHCR and pulling on the server. Also ensure `devtool` binary is available remotely for Chrome diagnostics.

## Current State (for reference)
- SSH to server, git pull, `docker compose up -d --build worker`.
- Optional `make dev-chrome` to open DevTools on Ubuntu.

## Proposed Flow (Simple, Repeatable)

### One-time setup
1) Create env files per environment:
   - `.env.deploy.<env>` (deploy host + GHCR credentials)
   - `.env.prod.<env>` (runtime env used by docker compose on the server)

2) Ensure required variables exist:
   - `.env.deploy.<env>`: `PROD_DIR`, `PROD_HOST`, `PROD_USER`, `GHCR_USER`, `GHCR_TOKEN`
   - `.env.prod.<env>`: `GHCR_USER` (used in `docker-compose.yml` image tag)

3) (Optional) Upload tool auth if needed:
   - `scripts/deploy-auth.sh <env> --tool codex|gemini|both`

4) Upload devtool binary (remote diagnostics):
   - Build or locate the local devtool binary.
   - Run: `scripts/deploy-devtool.sh <env> --bin <local_devtool_path>`
   - Default remote dest: `${PROD_DIR}/devtool` (same folder as `docker-compose.yml`).

### Ongoing deploys (no manual SSH)
- Build + push + remote pull + restart (single command):
  - `scripts/deploy.sh <env> --build`

- Pull-only (useful when build/push is done by CI):
  - `scripts/deploy.sh <env>`

- Update devtool when it changes:
  - `scripts/deploy-devtool.sh <env> --bin <local_devtool_path>`

## Environment Naming
- Example VPS env name: `justin-static-home-4`.
- Env files should be named accordingly:
  - `.env.deploy.justin-static-home-4`
  - `.env.prod.justin-static-home-4`

## Implementation Notes
- The repo already includes `scripts/deploy.sh` which:
  - Builds/pushes the image to GHCR (optional `--build`).
  - Uploads `docker-compose.yml` and `.env` to the server.
  - Runs `docker compose pull` + `up -d` on the remote host.
- `docker-compose.yml` uses:
  - `image: ghcr.io/${GHCR_USER}/peasydeal-product-miner:latest`
  - `service: worker`
- The repo includes `scripts/deploy-devtool.sh` which:
  - Uploads the `devtool` binary to the remote host.
  - Defaults to `${PROD_DIR}/devtool` unless overridden.

## Optional Enhancements (If Desired)
1) Add GitHub Actions workflow to build/push on every push to main.
2) Add Makefile shortcuts:
   - `make deploy-justin-static-home-4` -> `scripts/deploy.sh justin-static-home-4 --build`
   - `make deploy-justin-static-home-4-pull` -> `scripts/deploy.sh justin-static-home-4`
   - `make deploy-devtool-justin-static-home-4` -> `scripts/deploy-devtool.sh justin-static-home-4 --bin <path>`

## Decisions Based On Your Answers
- Env name example: `justin-static-home-4`.
- Image tagging: use `latest` only; overwrite old image each deploy.
- Devtool location: place in `${PROD_DIR}` next to `docker-compose.yml`.
