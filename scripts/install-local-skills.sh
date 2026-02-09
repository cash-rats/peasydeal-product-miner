#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/install-local-skills.sh [--tool codex|gemini|both]

Install repo-tracked skills into the current user's home:
  - codex  -> $HOME/.codex/skills
  - gemini -> gemini CLI user-scope install + enable
EOF
}

TOOL="both"
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
      usage >&2
      exit 2
      ;;
  esac
done

case "$TOOL" in
  codex|gemini|both) ;;
  *)
    echo "Invalid --tool value: $TOOL (expected codex|gemini|both)" >&2
    exit 2
    ;;
esac

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
CODEX_SKILLS_DIR="${ROOT_DIR}/codex/.codex/skills"
GEMINI_SKILLS_DIR="${ROOT_DIR}/gemini/.gemini/skills"

list_skills() {
  local dir="$1"
  if [[ ! -d "$dir" ]]; then
    return 0
  fi

  find "$dir" -mindepth 1 -maxdepth 1 -type d -exec basename {} \; | LC_ALL=C sort
}

install_codex() {
  local skills=()
  while IFS= read -r name; do
    [[ -n "$name" ]] && skills+=("$name")
  done < <(list_skills "$CODEX_SKILLS_DIR")

  if [[ ${#skills[@]} -eq 0 ]]; then
    echo "No Codex skills found under ${CODEX_SKILLS_DIR}" >&2
    return 1
  fi

  local dst_base="${HOME}/.codex/skills"
  mkdir -p "$dst_base"

  for s in "${skills[@]}"; do
    local src="${CODEX_SKILLS_DIR}/${s}/SKILL.md"
    local dst_dir="${dst_base}/${s}"
    if [[ ! -f "$src" ]]; then
      echo "Skip Codex skill without SKILL.md: ${s}" >&2
      continue
    fi
    mkdir -p "$dst_dir"
    cp "$src" "${dst_dir}/SKILL.md"
    echo "Installed Codex skill: ${s}"
  done
}

install_gemini() {
  local skills=()
  while IFS= read -r name; do
    [[ -n "$name" ]] && skills+=("$name")
  done < <(list_skills "$GEMINI_SKILLS_DIR")

  if [[ ${#skills[@]} -eq 0 ]]; then
    echo "No Gemini skills found under ${GEMINI_SKILLS_DIR}" >&2
    return 1
  fi

  local gemini_bin
  gemini_bin="$(command -v gemini || true)"
  if [[ -z "$gemini_bin" ]]; then
    echo "gemini CLI not found in PATH" >&2
    return 127
  fi

  for s in "${skills[@]}"; do
    local src_dir="${GEMINI_SKILLS_DIR}/${s}"
    if [[ ! -f "${src_dir}/SKILL.md" ]]; then
      echo "Skip Gemini skill without SKILL.md: ${s}" >&2
      continue
    fi
    "$gemini_bin" skills install "$src_dir" --scope user --consent
    "$gemini_bin" skills enable "$s"
    echo "Installed Gemini skill: ${s}"
  done
}

case "$TOOL" in
  codex)
    install_codex
    ;;
  gemini)
    install_gemini
    ;;
  both)
    install_codex
    install_gemini
    ;;
esac
