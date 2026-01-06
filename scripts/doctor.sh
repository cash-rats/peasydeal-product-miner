#!/usr/bin/env bash
set -euo pipefail

PORT="${CHROME_DEBUG_PORT:-9222}"
URL="http://127.0.0.1:${PORT}/json/version"

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required for this check." >&2
  exit 1
fi

echo "Checking: $URL"
resp="$(curl -fsS "$URL" || true)"
if [[ -z "$resp" ]]; then
  echo "FAIL: Chrome DevTools not reachable. Is Chrome running with --remote-debugging-port=$PORT ?" >&2
  exit 1
fi

echo "OK: Chrome DevTools reachable."

