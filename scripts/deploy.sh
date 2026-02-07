#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/deploy.sh <env-name> [--build]

Requires:
  .env.deploy.<env-name>  # deploy target + registry creds
  .env.prod.<env-name>    # runtime env pushed to the server

Optional flags:
  --build   Build + push the image before remote deploy (default: pull-only)
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

ENV_NAME="${1:-}"
if [[ -z "$ENV_NAME" ]]; then
  usage
  exit 2
fi

BUILD=0
if [[ "${2:-}" == "--build" ]]; then
  BUILD=1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

DEPLOY_ENV="${ROOT_DIR}/.env.deploy.${ENV_NAME}"
PROD_ENV="${ROOT_DIR}/.env.prod.${ENV_NAME}"
if [[ ! -f "$DEPLOY_ENV" ]]; then
  echo "Missing deploy env: $DEPLOY_ENV" >&2
  exit 1
fi
if [[ ! -f "$PROD_ENV" ]]; then
  echo "Missing prod env: $PROD_ENV" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "$DEPLOY_ENV"
set +a

require_var() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "Missing required var in ${DEPLOY_ENV}: ${name}" >&2
    exit 1
  fi
}

require_var PROD_DIR
require_var PROD_HOST
require_var PROD_USER
require_var GHCR_USER
require_var GHCR_TOKEN

PROD_PORT="${PROD_PORT:-22}"
PROD_SSH_KEY_PATH="${PROD_SSH_KEY_PATH:-}"
COMPOSE_FILE="${COMPOSE_FILE:-${ROOT_DIR}/docker-compose.yml}"
SERVICE_NAME="${SERVICE_NAME:-worker}"
IMAGE="${IMAGE:-ghcr.io/${GHCR_USER}/peasydeal-product-miner:latest}"

ssh_opts=(-p "$PROD_PORT")
scp_opts=(-P "$PROD_PORT")
if [[ -n "$PROD_SSH_KEY_PATH" ]]; then
  ssh_opts+=(-i "$PROD_SSH_KEY_PATH")
  scp_opts+=(-i "$PROD_SSH_KEY_PATH")
fi

if [[ "$BUILD" == "1" ]]; then
  echo "Building + pushing image: $IMAGE"
  echo "$GHCR_TOKEN" | docker login ghcr.io -u "$GHCR_USER" --password-stdin
  docker compose -f "$COMPOSE_FILE" build "$SERVICE_NAME"
  docker compose -f "$COMPOSE_FILE" push "$SERVICE_NAME"
fi

echo "Deploying to ${PROD_USER}@${PROD_HOST}:${PROD_DIR}"
ssh "${ssh_opts[@]}" "${PROD_USER}@${PROD_HOST}" "mkdir -p '$PROD_DIR' '$PROD_DIR/config'"

scp "${scp_opts[@]}" "$COMPOSE_FILE" "${PROD_USER}@${PROD_HOST}:${PROD_DIR}/docker-compose.yml"
scp "${scp_opts[@]}" "$PROD_ENV" "${PROD_USER}@${PROD_HOST}:${PROD_DIR}/.env"
scp -r "${scp_opts[@]}" "${ROOT_DIR}/config/." "${PROD_USER}@${PROD_HOST}:${PROD_DIR}/config"

ssh "${ssh_opts[@]}" "${PROD_USER}@${PROD_HOST}" \
  "cd '$PROD_DIR' && \
  echo '$GHCR_TOKEN' | docker login ghcr.io -u '$GHCR_USER' --password-stdin && \
  docker compose pull '$SERVICE_NAME' && \
  docker compose up --remove-orphans -d --force-recreate '$SERVICE_NAME' && \
  docker system prune -af"
