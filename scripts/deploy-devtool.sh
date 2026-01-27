#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/deploy-devtool.sh <env-name> --bin <local_devtool_path> [--dest <remote_path>]

Reads:
  .env.deploy.<env-name>

Required vars in .env.deploy.<env-name>:
  PROD_HOST
  PROD_USER

Optional vars in .env.deploy.<env-name>:
  PROD_PORT (default: 22)
  PROD_SSH_KEY_PATH
  PROD_DIR (used for default dest)
  DEVTOOL_REMOTE_PATH (overrides default dest)

Notes:
  - If --dest (or DEVTOOL_REMOTE_PATH) points to a root-owned directory like /usr/local/bin,
    the SSH user must have write permission (or you must place it somewhere writable like $HOME/bin).
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

BIN=""
DEST=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --bin)
      BIN="${2:-}"
      shift 2
      ;;
    --dest)
      DEST="${2:-}"
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

if [[ -z "$BIN" ]]; then
  echo "Missing required flag: --bin" >&2
  usage
  exit 2
fi
if [[ ! -f "$BIN" ]]; then
  echo "Binary not found: $BIN" >&2
  exit 1
fi

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

require_var PROD_HOST
require_var PROD_USER

PROD_PORT="${PROD_PORT:-22}"
PROD_SSH_KEY_PATH="${PROD_SSH_KEY_PATH:-}"

if [[ -z "$DEST" ]]; then
  if [[ -n "${DEVTOOL_REMOTE_PATH:-}" ]]; then
    DEST="$DEVTOOL_REMOTE_PATH"
  elif [[ -n "${PROD_DIR:-}" ]]; then
    DEST="${PROD_DIR%/}/devtool"
  else
    DEST="devtool"
  fi
fi

ssh_opts=(-p "$PROD_PORT")
scp_opts=(-P "$PROD_PORT")
if [[ -n "$PROD_SSH_KEY_PATH" ]]; then
  ssh_opts+=(-i "$PROD_SSH_KEY_PATH")
  scp_opts+=(-i "$PROD_SSH_KEY_PATH")
fi

# If DEST is (or resolves to) a directory on the remote, upload as DEST/devtool.
# This avoids the surprising behavior where scp/mv leaves a *.tmp.<pid> filename in that directory.
if [[ "$DEST" == */ ]]; then
  DEST="${DEST%/}/devtool"
else
  if ssh "${ssh_opts[@]}" "${PROD_USER}@${PROD_HOST}" "test -d '$DEST'" 2>/dev/null; then
    DEST="${DEST%/}/devtool"
  fi
fi

remote_tmp="${DEST}.tmp.$$"
remote_dir="$(dirname "$DEST")"

echo "Uploading devtool to ${PROD_USER}@${PROD_HOST}:${DEST}"
ssh "${ssh_opts[@]}" "${PROD_USER}@${PROD_HOST}" "mkdir -p '$remote_dir'"
scp "${scp_opts[@]}" "$BIN" "${PROD_USER}@${PROD_HOST}:${remote_tmp}"
ssh "${ssh_opts[@]}" "${PROD_USER}@${PROD_HOST}" "mv '$remote_tmp' '$DEST' && chmod +x '$DEST'"

echo "OK: ${DEST}"
