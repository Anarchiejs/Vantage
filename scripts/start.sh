#!/usr/bin/env bash
# Import your Obsidian vault, then serve the UI (http://127.0.0.1:7878).
# Usage: ./start.sh "/path/to/your vault"   (the first time)
#        ./start.sh                          (later: reuse the last vault)
# Ctrl+C stops the server.
# On Windows, use start.cmd instead.

set -euo pipefail
# shellcheck source=common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
supported_os start.cmd

open_browser() {
  local url="$1" opener=""
  if [ -n "${CI:-}" ]; then
    info "Open ${url} in a browser."
    return 0
  fi
  case "$(uname -s)" in
    Darwin)
      opener="open"
      ;;
    Linux)
      if [ -n "${DISPLAY:-${WAYLAND_DISPLAY:-}}" ] && command -v xdg-open >/dev/null 2>&1; then
        opener="xdg-open"
      fi
      ;;
  esac
  if [ -z "$opener" ]; then
    info "Open ${url} in a browser."
    return 0
  fi
  (
    sleep 2
    "$opener" "$url" >/dev/null 2>&1 || true
  ) &
  # bash 3.2 (macOS /bin/bash) only accepts a job spec here, and job control
  # is off in a script, so a failure is expected. The opener exits on its own.
  disown 2>/dev/null || true
}

vault="${1:-}"
if [ -n "$vault" ] && [ ! -d "$vault" ]; then
  fail "folder not found: ${vault} (give your Obsidian vault folder, in quotes)."
fi
if [ -n "$vault" ]; then
  vault="$(cd "$vault" && pwd -P)"
fi

enter_root
memo="$(vault_memo_file)"

if [ -z "$vault" ]; then
  if ! vault="$(read_memo "$memo")"; then
    printf '\n%sERROR: give your Obsidian vault folder the first time, for example:%s\n' "$red" "$reset" >&2
    printf '  ./start.sh "%s/Documents/My Vault"\n' "$HOME" >&2
    exit 1
  fi
  printf 'Remembered vault: %s\n' "$vault"
fi

if [ ! -d "$vault" ]; then
  fail "folder not found: ${vault} (give your Obsidian vault folder, in quotes)."
fi
vault="$(cd "$vault" && pwd -P)"

if [ ! -d "$vault/.obsidian" ]; then
  fail "${vault} is not an Obsidian vault (no .obsidian folder). Give your vault folder, not the project folder."
fi
if [ ! -e "$ROOT/observatoire" ]; then
  fail "observatoire is missing. Run ./build.sh first."
fi
if [ ! -x "$ROOT/observatoire" ]; then
  fail "observatoire is not executable. Run ./build.sh again."
fi

mkdir -p "$(dirname "$memo")"
printf '%s\n' "$vault" > "$memo"

step "Importing vault ${vault}"
log="$(mktemp "${TMPDIR:-/tmp}/observatoire-import.XXXXXX")"
set +e
"$ROOT/observatoire" import -vault "$vault" >"$log" 2>&1
code=$?
set -e
sed 's/^/    /' "$log"
if [ "$code" -ne 0 ]; then
  rm -f "$log"
  fail "The import failed (code ${code}). Read the messages above."
fi
if grep -q '^0 repositories' "$log"; then
  warn "no Git repository imported. Check the \"warning\" lines above if there are any."
  info "A project has commits only when its Projects/<Name>/<Name>.md card contains \"type: project\""
  info "and \"source_path:\" with the absolute path of a Git repository on this machine."
fi
rm -f "$log"

step "Starting the UI: http://127.0.0.1:7878  (Ctrl+C to stop)"
info "Live updates: an edited note or commit is reimported within a few seconds."
open_browser "http://127.0.0.1:7878"
"$ROOT/observatoire" watch -vault "$vault"
