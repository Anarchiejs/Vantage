# Shared helpers for the Linux and macOS scripts.
# Sourced by install.sh, build.sh, and start.sh. Does not change directory;
# callers use enter_root after they have resolved relative paths.

if [ -z "${BASH_VERSION:-}" ]; then
  echo "ERROR: these scripts require bash." >&2
  exit 1
fi

set -euo pipefail

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  echo "Source this file from install.sh, build.sh, or start.sh." >&2
  exit 1
fi

_common_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${_common_dir}/.." && pwd)"

if [ -t 1 ]; then
  cyan=$'\033[36m'
  red=$'\033[31m'
  yellow=$'\033[33m'
  green=$'\033[32m'
  reset=$'\033[0m'
else
  cyan=""
  red=""
  yellow=""
  green=""
  reset=""
fi

enter_root() {
  cd "$ROOT"
}

step() { printf '%s==> %s%s\n' "$cyan" "$1" "$reset"; }
info() { printf '    %s\n' "$1"; }
warn() { printf '    %sWarning: %s%s\n' "$yellow" "$1" "$reset"; }
fail() {
  printf '\n%sERROR: %s%s\n' "$red" "$1" "$reset" >&2
  exit 1
}

# $1 is the Windows launcher to name when this script is started from Git Bash.
supported_os() {
  local cmd_name="$1"
  case "$(uname -s)" in
    Linux|Darwin) ;;
    MINGW*|MSYS*|CYGWIN*)
      fail "On Windows, run ${cmd_name} from Command Prompt or PowerShell."
      ;;
    *)
      fail "Unsupported operating system $(uname -s). Observatoire supports Windows, Linux, and macOS."
      ;;
  esac
}

run() {
  local description="$1"
  shift
  if ! "$@" 2>&1 | while IFS= read -r line; do
    printf '    %s\n' "$line"
  done; then
    fail "${description} failed. Read the messages above."
  fi
}

require() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "$1 was not found. Run ./install.sh first."
  fi
}

# version_ge returns success when $1 is greater than or equal to $2.
version_ge() {
  awk -v a="$1" -v b="$2" 'BEGIN {
    n = split(a, A, ".")
    m = split(b, B, ".")
    if (m > n) n = m
    for (i = 1; i <= n; i++) {
      x = A[i] + 0
      y = B[i] + 0
      if (x > y) exit 0
      if (x < y) exit 1
    }
    exit 0
  }'
}

go_mod_version() {
  awk '/^go [0-9]/{ print $2; exit }' "$ROOT/go.mod"
}

tool_version() {
  local bin="$1"
  if [ "$bin" = "go" ]; then
    go version 2>/dev/null | awk '{ print $3 }' | sed 's/^go//' || true
  else
    node --version 2>/dev/null | sed 's/^v//' || true
  fi
}

prepend_user_tools() {
  local required current
  required="$(go_mod_version)"
  if [ -z "$required" ]; then
    fail "could not read the Go version from go.mod"
  fi
  if [ -x "${HOME}/.local/opt/go/bin/go" ]; then
    current="0"
    if command -v go >/dev/null 2>&1; then
      current="$(tool_version go)"
      if [ -z "$current" ]; then
        current="0"
      fi
    fi
    if ! version_ge "$current" "$required"; then
      PATH="${HOME}/.local/opt/go/bin:${PATH}"
    fi
  fi
  if [ -x "${HOME}/.local/opt/node/bin/node" ]; then
    current="0"
    if command -v node >/dev/null 2>&1; then
      current="$(tool_version node)"
      if [ -z "$current" ]; then
        current="0"
      fi
    fi
    if ! version_ge "$current" "22.12.0"; then
      PATH="${HOME}/.local/opt/node/bin:${PATH}"
    fi
  fi
  export PATH
  hash -r
}

vault_memo_file() {
  case "$(uname -s)" in
    Darwin)
      printf '%s\n' "${HOME}/Library/Application Support/Observatoire/vault.txt"
      ;;
    Linux)
      printf '%s\n' "${XDG_DATA_HOME:-${HOME}/.local/share}/observatoire/vault.txt"
      ;;
    *)
      fail "unsupported operating system $(uname -s)"
      ;;
  esac
}

read_memo() {
  local file="$1" line
  [ -f "$file" ] || return 1
  IFS= read -r line < "$file" || return 1
  line="${line%$'\r'}"
  line="${line#"${line%%[![:space:]]*}"}"
  line="${line%"${line##*[![:space:]]}"}"
  [ -n "$line" ] || return 1
  printf '%s\n' "$line"
}

prepend_user_tools
