#!/usr/bin/env bash
# Generic multi-machine deploy template for Docker Compose + Inngest services.
# Adapt for each project; do not assume Inngest connectivity modes here.

set -Eeuo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yaml}"
SERVICE_NAME="${SERVICE_NAME:-app}"
ENV_REMOTE_NAME="${ENV_REMOTE_NAME:-.env.prod}"

discover_all_machines() {
  shopt -s nullglob
  local names=()
  for f in .env.deploy.*; do
    [[ -f "$f" ]] || continue
    local base="${f##.env.deploy.}"
    [[ "$base" == "example" ]] && continue
    [[ -n "$base" ]] && names+=("$base")
  done
  if ((${#names[@]} > 0)); then
    printf "%s\n" "${names[@]}" | sort -u
  fi
}

require_file() {
  local f="$1"
  [[ -f "$f" ]] || { echo "Missing required file: $f" >&2; exit 1; }
}

do_build_push() {
  echo "==> docker compose -f $COMPOSE_FILE build $SERVICE_NAME"
  docker compose -f "$COMPOSE_FILE" build "$SERVICE_NAME"
  echo "==> docker compose -f $COMPOSE_FILE push $SERVICE_NAME"
  docker compose -f "$COMPOSE_FILE" push "$SERVICE_NAME"
}

usage() {
  cat <<USAGE
Usage:
  $(basename "$0") <machine|all|name1,name2,...>
  $(basename "$0") --target <machine|all|name1,name2> [--build [once|skip]]

Available machines:
$(discover_all_machines | sed 's/^/  - /')
USAGE
}

deploy_one() {
  local machine="$1"
  local deploy_cfg=".env.deploy.$machine"
  local app_env=".env.prod.$machine"

  require_file "$deploy_cfg"
  require_file "$app_env"

  set -a
  # shellcheck disable=SC1090
  source "$deploy_cfg"
  set +a

  : "${PROD_DIR:?PROD_DIR is required in $deploy_cfg}"
  : "${PROD_HOST:?PROD_HOST is required in $deploy_cfg}"
  : "${PROD_USER:?PROD_USER is required in $deploy_cfg}"
  : "${PROD_PORT:?PROD_PORT is required in $deploy_cfg}"
  : "${PROD_SSH_KEY_PATH:?PROD_SSH_KEY_PATH is required in $deploy_cfg}"
  : "${GHCR_USER:?GHCR_USER is required in $deploy_cfg}"
  : "${GHCR_TOKEN:?GHCR_TOKEN is required in $deploy_cfg}"

  local COMPOSE_FILES="$COMPOSE_FILE"
  local COMPOSE_FLAGS="-f $COMPOSE_FILE"

  echo "==> [$machine] mkdir -p $PROD_DIR"
  ssh -i "$PROD_SSH_KEY_PATH" -p "$PROD_PORT" "$PROD_USER@$PROD_HOST" "mkdir -p '$PROD_DIR'"

  echo "==> [$machine] upload app env: $app_env -> $PROD_DIR/$ENV_REMOTE_NAME"
  scp -i "$PROD_SSH_KEY_PATH" -P "$PROD_PORT" "$app_env" "$PROD_USER@$PROD_HOST:$PROD_DIR/$ENV_REMOTE_NAME"

  echo "==> [$machine] upload compose files: $COMPOSE_FILES"
  scp -i "$PROD_SSH_KEY_PATH" -P "$PROD_PORT" $COMPOSE_FILES "$PROD_USER@$PROD_HOST:$PROD_DIR/"

  echo "==> [$machine] docker login ghcr.io"
  ssh -i "$PROD_SSH_KEY_PATH" -p "$PROD_PORT" "$PROD_USER@$PROD_HOST" \
    "docker login ghcr.io -u '$GHCR_USER' --password-stdin" <<<"$GHCR_TOKEN"

  echo "==> [$machine] docker compose pull/up/prune"
  ssh -i "$PROD_SSH_KEY_PATH" -p "$PROD_PORT" -t "$PROD_USER@$PROD_HOST" "
    set -e
    cd '$PROD_DIR'
    docker compose --env-file '$ENV_REMOTE_NAME' $COMPOSE_FLAGS pull
    docker compose --env-file '$ENV_REMOTE_NAME' $COMPOSE_FLAGS up -d --no-deps --remove-orphans
    docker system prune -af
  "
  echo "✅ [$machine] done."
}

TARGETS=""
BUILD_POLICY="skip"

if [[ $# -eq 0 ]]; then
  usage
  exit 1
elif [[ "$1" =~ ^- ]]; then
  while [[ $# -gt 0 ]]; do
    case "$1" in
    --target | -t) TARGETS="${2:-}"; shift 2 ;;
    --build | -b)
      if [[ -n "${2:-}" && "${2}" =~ ^(once|skip)$ ]]; then
        BUILD_POLICY="$2"; shift 2
      else
        BUILD_POLICY="once"; shift 1
      fi
      ;;
    -h | --help) usage; exit 0 ;;
    *) echo "Unknown arg: $1" >&2; usage; exit 1 ;;
    esac
  done
  [[ -n "$TARGETS" ]] || { echo "Error: --target required" >&2; usage; exit 1; }
else
  TARGETS="$1"
fi

TARGET_ARRAY=()
if [[ "$TARGETS" == "all" ]]; then
  while IFS= read -r machine; do
    [[ -n "$machine" ]] && TARGET_ARRAY+=("$machine")
  done <<<"$(discover_all_machines)"
  [[ ${#TARGET_ARRAY[@]} -gt 0 ]] || { echo "No .env.deploy.* found" >&2; exit 1; }
elif [[ "$TARGETS" == *","* ]]; then
  IFS=',' read -r -a TARGET_ARRAY <<<"$TARGETS"
else
  TARGET_ARRAY=("$TARGETS")
fi

if [[ "$BUILD_POLICY" == "once" ]]; then
  echo "[Build policy] once: build/push once for all machines"
  do_build_push
elif [[ "$BUILD_POLICY" == "skip" ]]; then
  echo "[Build policy] skip: no build/push, pull only"
else
  echo "Unknown build policy: $BUILD_POLICY (valid: once, skip)" >&2
  exit 1
fi

for m in "${TARGET_ARRAY[@]}"; do
  echo ""
  echo "================ Deploy to: $m ================"
  deploy_one "$m"
done

echo ""
echo "✅ All done."
