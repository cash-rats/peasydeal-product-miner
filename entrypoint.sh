#!/bin/sh
set -eu

HOME_DIR="${HOME:-/home/app}"
export HOME="$HOME_DIR"

mkdir -p "$HOME_DIR"

# Make Codex and Gemini "coexist" by mapping their per-tool config dirs
# into a single neutral $HOME. The actual credential stores are persisted
# via bind mounts at /codex and /gemini.
if [ -d "/codex" ]; then
  mkdir -p "/codex/.codex"
  ln -sfn "/codex/.codex" "$HOME_DIR/.codex"
fi

if [ -d "/gemini" ]; then
  mkdir -p "/gemini/.gemini"
  ln -sfn "/gemini/.gemini" "$HOME_DIR/.gemini"
fi

# Persist crawl outputs via the /out bind mount. Many code paths use a relative
# "out" directory (e.g. /app/out), so map it to /out inside containers.
mkdir -p "/out" || true
if [ -e "/app/out" ] && [ ! -L "/app/out" ]; then
  rm -rf "/app/out"
fi
ln -sfn "/out" "/app/out"

exec "$@"
