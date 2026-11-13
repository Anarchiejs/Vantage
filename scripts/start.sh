#!/usr/bin/env bash
# Import your Obsidian vault, then serve the UI (http://127.0.0.1:7878).
# Usage: ./start.sh [options] [vault]
#        ./start.sh "/path/to/your vault"          (the first time)
#        ./start.sh                                 (later: reuse the last vault)
#        ./start.sh --port 8080 --repos "Name=/abs/path" "/path/to/vault"
# Ctrl+C stops the server.
# On Windows, use start.cmd instead (-Port and -Repos).

set -euo pipefail
# shellcheck source=common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
supported_os start.cmd

usage() {
  cat <<'EOF'
Usage: ./start.sh [options] [vault]

  vault                  Obsidian vault folder. Omitted later, the remembered path is used.
  -port, --port N        UI port on 127.0.0.1 (default 7878)
  -repos, --repos LIST   Replace source_path: "Name=/absolute/path,Other=/path"
                         Also read from <vault>/.observatoire/repos when that file exists.
                         Paths in the file are relative to the vault. -repos replaces a name.
  -h, --help             Show this help
EOF
}

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

# The sample vault maps Atlas and Compass onto testdata/repos. Create those
# repositories the first time, so ./start.sh testdata/vault works on a fresh tree.
ensure_fixtures() {
  local map="$vault/.observatoire/repos"
  if [ ! -f "$map" ]; then
    return 0
  fi
  if [ -d "$ROOT/testdata/repos/alpha/.git" ] && [ -d "$ROOT/testdata/repos/beta/.git" ]; then
    return 0
  fi
  if ! grep -q 'repos/alpha' "$map" && ! grep -q 'repos/beta' "$map"; then
    return 0
  fi
  require go
  step "Generating the sample Git repositories"
  run "Fixture repositories" go run ./scripts/fixtures/make-repos.go
}

port="7878"
repos=""
vault=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    -port|--port)
      if [ "$#" -lt 2 ]; then
        fail "-port needs a number."
      fi
      port="$2"
      shift 2
      ;;
    -repos|--repos)
      if [ "$#" -lt 2 ]; then
        fail "-repos needs a value, for example \"Name=/absolute/path\"."
      fi
      repos="$2"
      shift 2
      ;;
    --)
      shift
      break
      ;;
    -*)
      fail "unknown option: $1 (try ./start.sh --help)"
      ;;
    *)
      if [ -n "$vault" ]; then
        fail "unexpected argument: $1"
      fi
      vault="$1"
      shift
      ;;
  esac
done
if [ "$#" -gt 0 ]; then
  if [ -n "$vault" ]; then
    fail "unexpected argument: $1"
  fi
  vault="$1"
  shift
  if [ "$#" -gt 0 ]; then
    fail "unexpected argument: $1"
  fi
fi

case "$port" in
  ''|*[!0-9]*)
    fail "port must be a number between 1 and 65535."
    ;;
esac
port_number=$((10#$port))
if [ "$port_number" -lt 1 ] || [ "$port_number" -gt 65535 ]; then
  fail "port must be a number between 1 and 65535."
fi
port="$port_number"

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
    if ! vault="$(read_memo "$(legacy_vault_memo_file)")"; then
      printf '\n%sERROR: give your Obsidian vault folder the first time, for example:%s\n' "$red" "$reset" >&2
      printf '  ./start.sh "%s/Documents/My Vault"\n' "$HOME" >&2
      exit 1
    fi
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
if [ ! -e "$ROOT/vantage" ]; then
  fail "vantage is missing. Run ./build.sh first."
fi
if [ ! -x "$ROOT/vantage" ]; then
  fail "vantage is not executable. Run ./build.sh again."
fi

mkdir -p "$(dirname "$memo")"
printf '%s\n' "$vault" > "$memo"

ensure_fixtures

url="http://127.0.0.1:${port}"
step "Importing vault ${vault}"
log="$(mktemp "${TMPDIR:-/tmp}/vantage-import.XXXXXX")"
set +e
if [ -n "$repos" ]; then
  "$ROOT/vantage" import -vault "$vault" -repos "$repos" >"$log" 2>&1
else
  "$ROOT/vantage" import -vault "$vault" >"$log" 2>&1
fi
code=$?
set -e
sed 's/^/    /' "$log"
if [ "$code" -ne 0 ]; then
  rm -f "$log"
  fail "The import failed (code ${code}). Read the messages above."
fi
if grep -q '^0 repositories' "$log"; then
  warn "no Git repository imported. Check the \"warning\" lines above if there are any."
  info "A project has commits when its Projects/<Name>/<Name>.md card has \"type: project\""
  info "and \"source_path:\" is a Git repository on this machine."
  info "Or pass --repos \"Name=/absolute/path\", or add <vault>/.observatoire/repos."
fi
rm -f "$log"

step "Starting the UI: ${url}  (Ctrl+C to stop)"
info "Live updates: an edited note or commit is reimported within a few seconds."
open_browser "$url"
if [ -n "$repos" ]; then
  "$ROOT/vantage" watch -vault "$vault" -port "$port" -repos "$repos"
else
  "$ROOT/vantage" watch -vault "$vault" -port "$port"
fi
