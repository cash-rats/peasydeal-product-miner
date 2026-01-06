#!/usr/bin/env bash
set -euo pipefail

PORT="${CHROME_DEBUG_PORT:-9222}"
PROFILE_DIR="${CHROME_PROFILE_DIR:-$HOME/chrome-mcp-profiles/shopee}"

mkdir -p "$PROFILE_DIR"

if [[ "$OSTYPE" == "darwin"* ]]; then
  # Start a separate Chrome instance with a dedicated profile dir.
  # NOTE: Chrome 136+ requires a non-default --user-data-dir for remote debugging.
  open -na "Google Chrome" --args \
    --remote-debugging-port="$PORT" \
    --user-data-dir="$PROFILE_DIR"
  exit 0
fi

if command -v google-chrome >/dev/null 2>&1; then
  google-chrome --remote-debugging-port="$PORT" --user-data-dir="$PROFILE_DIR" >/dev/null 2>&1 &
  disown
  exit 0
fi

if command -v chromium >/dev/null 2>&1; then
  chromium --remote-debugging-port="$PORT" --user-data-dir="$PROFILE_DIR" >/dev/null 2>&1 &
  disown
  exit 0
fi

echo "Could not find Chrome/Chromium. Start Chrome manually with:" >&2
echo "  --remote-debugging-port=$PORT --user-data-dir=$PROFILE_DIR" >&2
exit 1

