#!/usr/bin/env bash
# Build the UI (web/dist) then the observatoire binary, which embeds it.
# Usage: ./build.sh
# On Windows, use build.cmd instead.

set -euo pipefail
# shellcheck source=common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
supported_os build.cmd
enter_root

require go
require npm

required="$(go_mod_version)"
current="$(tool_version go)"
if ! version_ge "$current" "$required"; then
  fail "Go ${current} is older than go.mod (${required}). Run ./install.sh."
fi
nver="$(tool_version node)"
if ! version_ge "$nver" "22.12.0"; then
  fail "Node ${nver} is older than 22.12.0. Run ./install.sh."
fi
if [ ! -d web/node_modules ]; then
  fail "UI dependencies are missing. Run ./install.sh first."
fi

step "Building the UI (npm run build)"
run "The UI build" npm --prefix web run build
info "(the warning \"Some chunks are larger than 500 kB\" is normal)"

step "Compiling observatoire"
run "The compile" go build -tags embed -o observatoire ./cmd/observatoire

printf '\n%sBuild complete: observatoire%s\n' "$green" "$reset"
printf 'Next step: ./start.sh "/path/to/your vault"\n'
