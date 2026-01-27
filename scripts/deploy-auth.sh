#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/deploy-auth.sh <env-name> --tool codex|gemini|both

Reads:
  .env.deploy.<env-name>

Required vars in .env.deploy.<env-name>:
  PROD_DIR
  PROD_HOST
  PROD_USER

Optional vars in .env.deploy.<env-name>:
  PROD_PORT (default: 22)
  PROD_SSH_KEY_PATH

Local inputs (created by login targets):
  Codex:
    codex/.codex/config.toml (repo-tracked)
    codex/.codex/auth.json   (created by: make docker-codex-login)
  Gemini:
    gemini/.gemini/settings.json (repo-tracked)
    gemini/.gemini/oauth_creds.json OR gemini/.gemini/google_accounts.json
      (created by: make docker-gemini-login)
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
shift

TOOL=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --tool)
      TOOL="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown arg: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if [[ -z "$TOOL" ]]; then
  echo "Missing required flag: --tool codex|gemini|both" >&2
  usage
  exit 2
fi
case "$TOOL" in
  codex|gemini|both) ;;
  *)
    echo "Invalid --tool: $TOOL (expected codex, gemini, or both)" >&2
    exit 2
    ;;
esac

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
DEPLOY_ENV="${ROOT_DIR}/.env.deploy.${ENV_NAME}"
if [[ ! -f "$DEPLOY_ENV" ]]; then
  echo "Missing deploy env: $DEPLOY_ENV" >&2
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

PROD_PORT="${PROD_PORT:-22}"
PROD_SSH_KEY_PATH="${PROD_SSH_KEY_PATH:-}"
if [[ "$PROD_SSH_KEY_PATH" == "~/"* ]]; then
  PROD_SSH_KEY_PATH="${HOME}/${PROD_SSH_KEY_PATH:2}"
fi

ssh_opts=(-p "$PROD_PORT")
scp_opts=(-P "$PROD_PORT")
if [[ -n "$PROD_SSH_KEY_PATH" ]]; then
  ssh_opts+=(-i "$PROD_SSH_KEY_PATH")
  scp_opts+=(-i "$PROD_SSH_KEY_PATH")
fi

remote_base="${PROD_DIR%/}"

upload_one() {
  local local_path="$1"
  local remote_path="$2"

  if [[ ! -f "$local_path" ]]; then
    echo "Missing local file: $local_path" >&2
    return 1
  fi

  local remote_dir
  remote_dir="$(dirname "$remote_path")"

  ssh "${ssh_opts[@]}" "${PROD_USER}@${PROD_HOST}" "mkdir -p '$remote_dir'"

  local tmp="${remote_path}.tmp.$$"
  scp "${scp_opts[@]}" "$local_path" "${PROD_USER}@${PROD_HOST}:${tmp}"
  ssh "${ssh_opts[@]}" "${PROD_USER}@${PROD_HOST}" "mv '$tmp' '$remote_path'"
}

echo "Uploading tool auth to ${PROD_USER}@${PROD_HOST}:${remote_base}"

if [[ "$TOOL" == "codex" || "$TOOL" == "both" ]]; then
  codex_cfg="${ROOT_DIR}/codex/.codex/config.toml"
  codex_auth="${ROOT_DIR}/codex/.codex/auth.json"
  if [[ ! -f "$codex_auth" ]]; then
    echo "Missing Codex auth: ${codex_auth}" >&2
    echo "Run: make docker-codex-login" >&2
    exit 1
  fi

  upload_one "$codex_cfg"  "${remote_base}/codex/.codex/config.toml"
  upload_one "$codex_auth" "${remote_base}/codex/.codex/auth.json"
  ssh "${ssh_opts[@]}" "${PROD_USER}@${PROD_HOST}" \
    "chmod 700 '${remote_base}/codex' '${remote_base}/codex/.codex' 2>/dev/null || true; chmod 600 '${remote_base}/codex/.codex/'*.json 2>/dev/null || true"
fi

if [[ "$TOOL" == "gemini" || "$TOOL" == "both" ]]; then
  gemini_settings="${ROOT_DIR}/gemini/.gemini/settings.json"
  gemini_oauth="${ROOT_DIR}/gemini/.gemini/oauth_creds.json"
  gemini_accounts="${ROOT_DIR}/gemini/.gemini/google_accounts.json"

  if [[ ! -f "$gemini_oauth" && ! -f "$gemini_accounts" ]]; then
    echo "Missing Gemini auth under ${ROOT_DIR}/gemini/.gemini/" >&2
    echo "Expected oauth_creds.json or google_accounts.json" >&2
    echo "Run: make docker-gemini-login" >&2
    exit 1
  fi

  upload_one "$gemini_settings" "${remote_base}/gemini/.gemini/settings.json"
  if [[ -f "$gemini_oauth" ]]; then
    upload_one "$gemini_oauth" "${remote_base}/gemini/.gemini/oauth_creds.json"
  fi
  if [[ -f "$gemini_accounts" ]]; then
    upload_one "$gemini_accounts" "${remote_base}/gemini/.gemini/google_accounts.json"
  fi

  ssh "${ssh_opts[@]}" "${PROD_USER}@${PROD_HOST}" \
    "chmod 700 '${remote_base}/gemini' '${remote_base}/gemini/.gemini' 2>/dev/null || true; chmod 600 '${remote_base}/gemini/.gemini/'*.json 2>/dev/null || true"
fi

echo "OK"

