#!/usr/bin/env bash
# Proves start.sh remembers a vault and that SIGTERM stops the server.
# Linux and macOS only. Requires ./observatoire from ./build.sh.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [ ! -x "$ROOT/observatoire" ]; then
  echo "observatoire is missing. Run ./build.sh first." >&2
  exit 1
fi

# curl, not bash /dev/tcp: the bash shipped with macOS is built without network redirections.
# Not the default 7878: the script must forward --port or this check never connects.
port=7877
url="http://127.0.0.1:${port}"

port_open() {
  if command -v curl >/dev/null 2>&1; then
    curl -sf -o /dev/null --max-time 1 "${url}/"
    return
  fi
  python3 -c 'import socket,sys; s=socket.create_connection(("127.0.0.1",int(sys.argv[1])),1); s.close()' "$port"
}

if port_open; then
  echo "port ${port} is already in use" >&2
  exit 1
fi

tmp="$(mktemp -d "${TMPDIR:-/tmp}/observatoire-smoke.XXXXXX")"
ok=0
pid=""
log=""

stop_server() {
  if [ -z "$pid" ]; then
    return 0
  fi
  if kill -0 "$pid" 2>/dev/null; then
    kill -TERM -"$pid" 2>/dev/null || true
    kill -TERM "$pid" 2>/dev/null || true
    if command -v pkill >/dev/null 2>&1; then
      pkill -TERM -P "$pid" 2>/dev/null || true
    fi
    wait "$pid" 2>/dev/null || true
  fi
  pid=""
}

cleanup() {
  stop_server
  if [ "$ok" -eq 1 ]; then
    rm -rf "$tmp"
  else
    echo "smoke logs kept in $tmp" >&2
  fi
}
trap cleanup EXIT

wait_until_listening() {
  local i
  i=0
  while [ "$i" -lt 150 ]; do
    if port_open; then
      return 0
    fi
    if [ -n "$pid" ] && ! kill -0 "$pid" 2>/dev/null; then
      echo "start.sh exited before listening" >&2
      cat "$log" >&2
      exit 1
    fi
    sleep 0.2
    i=$((i + 1))
  done
  echo "timed out waiting for ${url}" >&2
  cat "$log" >&2
  exit 1
}

wait_until_closed() {
  local i
  i=0
  while [ "$i" -lt 25 ]; do
    if ! port_open; then
      return 0
    fi
    sleep 0.2
    i=$((i + 1))
  done
  echo "server still listening on ${url}" >&2
  exit 1
}

launch() {
  local name="$1"
  shift
  log="$tmp/log-${name}"
  : >"$log"
  set -m || true
  ./start.sh "$@" >"$log" 2>&1 &
  pid=$!
  set +m || true
}

# Keep the remembered path inside this temporary home.
export CI=1
export HOME="$tmp/home"
export XDG_DATA_HOME="$tmp/share"
mkdir -p "$HOME" "$XDG_DATA_HOME"
vault="$tmp/vault"
mkdir -p "$vault/.obsidian"
log="$tmp/log-early"

set +e
./start.sh >"$log" 2>&1
code=$?
set -e
if [ "$code" -eq 0 ]; then
  echo "start.sh without a vault should fail" >&2
  cat "$log" >&2
  exit 1
fi
if ! grep -q "give your Obsidian vault folder" "$log"; then
  echo "missing first-run message" >&2
  cat "$log" >&2
  exit 1
fi

set +e
./start.sh --port "$port" --repos "Atlas=/tmp/observatoire-smoke" >"$log" 2>&1
code=$?
set -e
if [ "$code" -eq 0 ]; then
  echo "start.sh with options and no vault should fail" >&2
  cat "$log" >&2
  exit 1
fi
if ! grep -q "give your Obsidian vault folder" "$log"; then
  echo "options were not accepted before the vault check" >&2
  cat "$log" >&2
  exit 1
fi

set +e
./start.sh --not-an-option >"$log" 2>&1
code=$?
set -e
if [ "$code" -eq 0 ] || ! grep -q "unknown option" "$log"; then
  echo "unknown option was accepted" >&2
  cat "$log" >&2
  exit 1
fi

set +e
./start.sh --port 0 "$vault" >"$log" 2>&1
code=$?
set -e
if [ "$code" -eq 0 ] || ! grep -q "port must be a number" "$log"; then
  echo "port 0 was accepted" >&2
  cat "$log" >&2
  exit 1
fi

set +e
./start.sh "$tmp/missing" >"$log" 2>&1
code=$?
set -e
if [ "$code" -eq 0 ]; then
  echo "start.sh accepted a missing folder" >&2
  cat "$log" >&2
  exit 1
fi
if ! grep -q "folder not found" "$log"; then
  echo "missing folder message" >&2
  cat "$log" >&2
  exit 1
fi

mkdir -p "$tmp/plain"
set +e
./start.sh "$tmp/plain" >"$log" 2>&1
code=$?
set -e
if [ "$code" -eq 0 ]; then
  echo "start.sh accepted a folder that is not a vault" >&2
  cat "$log" >&2
  exit 1
fi
if ! grep -q "not an Obsidian vault" "$log"; then
  echo "missing vault message" >&2
  cat "$log" >&2
  exit 1
fi

launch first --port "$port" "$vault"
wait_until_listening
if ! grep -q "Observatoire on ${url}" "$log"; then
  echo "server log missing" >&2
  cat "$log" >&2
  exit 1
fi
if ! grep -q "0 repositories" "$log"; then
  echo "import summary missing" >&2
  cat "$log" >&2
  exit 1
fi

if [ "$(uname -s)" = "Darwin" ]; then
  memo="${HOME}/Library/Application Support/Observatoire/vault.txt"
else
  memo="${XDG_DATA_HOME}/observatoire/vault.txt"
fi
expected="$(cd "$vault" && pwd -P)"
got="$(tr -d '\r' < "$memo")"
if [ "$got" != "$expected" ]; then
  echo "memo is [${got}], want [${expected}]" >&2
  exit 1
fi

stop_server
wait_until_closed

launch remembered --port "$port"
wait_until_listening
if ! grep -q "Remembered vault:" "$log"; then
  echo "vault was not remembered" >&2
  cat "$log" >&2
  exit 1
fi
stop_server
wait_until_closed

ok=1
echo "start.sh smoke passed"
