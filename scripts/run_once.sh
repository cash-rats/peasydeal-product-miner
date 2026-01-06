#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "Usage: $0 \"https://shopee.tw/...\"" >&2
  exit 2
fi

URL="$1"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

mkdir -p "$ROOT_DIR/out"

if ! command -v go >/dev/null 2>&1; then
  echo "go is required for local runs. Install Go 1.22+." >&2
  exit 1
fi

go run "$ROOT_DIR/runner" \
  -url "$URL" \
  -prompt-file "$ROOT_DIR/config/prompt.product.txt" \
  -out-dir "$ROOT_DIR/out"
