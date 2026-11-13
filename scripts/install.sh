#!/usr/bin/env bash
# Install what Vantage needs to build: Git, Go, and Node,
# then the Go and npm dependencies.
# Usage: ./install.sh
# On Windows, use install.cmd instead.

set -euo pipefail
# shellcheck source=common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
supported_os install.cmd
enter_root

fetch() {
  local url="$1" dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$dest"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$dest" "$url"
  else
    fail "curl or wget is required to download ${url}"
  fi
}

verify_sha256() {
  local file="$1" want="$2" got
  if [ -z "$want" ]; then
    fail "checksum for $(basename "$file") was empty"
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    got="$(sha256sum "$file" | awk '{ print tolower($1) }')"
  elif command -v shasum >/dev/null 2>&1; then
    got="$(shasum -a 256 "$file" | awk '{ print tolower($1) }')"
  else
    fail "sha256sum or shasum is required to verify $(basename "$file")"
  fi
  if [ "$got" != "$want" ]; then
    fail "checksum mismatch for $(basename "$file")"
  fi
}

run_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  else
    sudo "$@"
  fi
}

host_go_os() {
  case "$(uname -s)" in
    Linux) printf 'linux\n' ;;
    Darwin) printf 'darwin\n' ;;
    *) fail "unsupported operating system $(uname -s)" ;;
  esac
}

host_node_os() {
  case "$(uname -s)" in
    Linux) printf 'linux\n' ;;
    Darwin) printf 'darwin\n' ;;
    *) fail "unsupported operating system $(uname -s)" ;;
  esac
}

host_go_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64\n' ;;
    aarch64|arm64) printf 'arm64\n' ;;
    *) fail "unsupported architecture $(uname -m). Install Go from https://go.dev/dl/, then run ./install.sh again." ;;
  esac
}

host_node_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'x64\n' ;;
    aarch64|arm64) printf 'arm64\n' ;;
    *) fail "unsupported architecture $(uname -m). Install Node.js 22.12 or newer from https://nodejs.org/, then run ./install.sh again." ;;
  esac
}

install_go() {
  local version="$1" os arch name url tmp want
  os="$(host_go_os)"
  arch="$(host_go_arch)"
  name="go${version}.${os}-${arch}.tar.gz"
  # dl.google.com serves the tarball and a one-line checksum. go.dev/dl redirects
  # the tarball but returns an HTML page for the .sha256 URL.
  url="https://dl.google.com/go/${name}"
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/vantage-go.XXXXXX")"
  trap 'rm -rf "$tmp"' EXIT
  info "Downloading ${url}"
  fetch "$url" "$tmp/$name"
  fetch "${url}.sha256" "$tmp/sum.sha256"
  want="$(awk 'NF { print tolower($1); exit }' "$tmp/sum.sha256")"
  verify_sha256 "$tmp/$name" "$want"
  mkdir -p "${HOME}/.local/opt"
  rm -rf "${HOME}/.local/opt/go"
  tar -C "$tmp" -xzf "$tmp/$name"
  mv "$tmp/go" "${HOME}/.local/opt/go"
  rm -rf "$tmp"
  trap - EXIT
}

install_node() {
  local os arch suffix tmp asset want extracted
  os="$(host_node_os)"
  arch="$(host_node_arch)"
  suffix="-${os}-${arch}.tar.gz"
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/vantage-node.XXXXXX")"
  trap 'rm -rf "$tmp"' EXIT
  info "Downloading the current Node.js 22 release"
  fetch "https://nodejs.org/dist/latest-v22.x/SHASUMS256.txt" "$tmp/SHASUMS256.txt"
  asset="$(awk -v suf="$suffix" '$2 ~ /^node-v/ && substr($2, length($2) - length(suf) + 1) == suf { print $2; exit }' "$tmp/SHASUMS256.txt")"
  want="$(awk -v suf="$suffix" '$2 ~ /^node-v/ && substr($2, length($2) - length(suf) + 1) == suf { print tolower($1); exit }' "$tmp/SHASUMS256.txt")"
  if [ -z "$asset" ] || [ -z "$want" ]; then
    fail "Node.js 22 has no official build for ${os}/${arch}."
  fi
  fetch "https://nodejs.org/dist/latest-v22.x/${asset}" "$tmp/$asset"
  verify_sha256 "$tmp/$asset" "$want"
  mkdir -p "${HOME}/.local/opt"
  rm -rf "${HOME}/.local/opt/node"
  tar -C "$tmp" -xzf "$tmp/$asset"
  extracted=""
  for dir in "$tmp"/node-v*; do
    if [ -d "$dir" ]; then
      extracted="$dir"
      break
    fi
  done
  if [ -z "$extracted" ]; then
    fail "the Node.js archive did not contain a node directory"
  fi
  mv "$extracted" "${HOME}/.local/opt/node"
  rm -rf "$tmp"
  trap - EXIT
}

ensure_git() {
  if command -v git >/dev/null 2>&1; then
    info "git: already installed ($(git --version))"
    return
  fi
  step "Installing git"
  case "$(uname -s)" in
    Darwin)
      if ! command -v brew >/dev/null 2>&1; then
        fail "git is missing and Homebrew was not found. Install the Xcode command line tools (xcode-select --install) or Homebrew (https://brew.sh), then run ./install.sh again."
      fi
      run "Installing git" brew install git
      ;;
    Linux)
      if [ "$(id -u)" -ne 0 ] && ! command -v sudo >/dev/null 2>&1; then
        fail "git is missing, and sudo was not found to install it. Install git, then run ./install.sh again."
      fi
      if command -v apt-get >/dev/null 2>&1; then
        run "Updating packages" run_root env DEBIAN_FRONTEND=noninteractive apt-get update
        run "Installing git" run_root env DEBIAN_FRONTEND=noninteractive apt-get install -y git
      elif command -v dnf >/dev/null 2>&1; then
        run "Installing git" run_root dnf install -y git
      elif command -v pacman >/dev/null 2>&1; then
        run "Installing git" run_root pacman -S --noconfirm git
      elif command -v zypper >/dev/null 2>&1; then
        run "Installing git" run_root zypper --non-interactive install git
      else
        fail "git is missing and no supported package manager was found (apt-get, dnf, pacman, or zypper). Install git, then run ./install.sh again."
      fi
      ;;
  esac
  if ! command -v git >/dev/null 2>&1; then
    fail "git was just installed but is still not found. Open a new terminal and run ./install.sh again."
  fi
  info "git: installed ($(git --version))"
}

ensure_go() {
  local required current
  required="$(go_mod_version)"
  if command -v go >/dev/null 2>&1; then
    current="$(tool_version go)"
    if version_ge "$current" "$required"; then
      info "go: already installed (${current})"
      return
    fi
    warn "Go ${current} is older than go.mod (${required}). Installing Go ${required} under ~/.local/opt/go."
  else
    step "Installing Go ${required} under ~/.local/opt/go"
  fi
  install_go "$required"
  prepend_user_tools
  require go
  info "go: installed ($(go version))"
  info "Add this to your shell profile if you want go in new terminals:"
  info '  export PATH="$HOME/.local/opt/go/bin:$PATH"'
}

ensure_node() {
  local min="22.12.0" current
  if command -v node >/dev/null 2>&1; then
    current="$(tool_version node)"
    if version_ge "$current" "$min"; then
      info "node: already installed (${current})"
      if ! command -v npm >/dev/null 2>&1; then
        fail "npm was not found next to node. Install Node.js ${min} or newer from https://nodejs.org/, then run ./install.sh again."
      fi
      info "npm: already installed ($(npm --version))"
      return
    fi
    warn "Node ${current} is older than ${min}. Installing the current Node.js 22 under ~/.local/opt/node."
  else
    step "Installing the current Node.js 22 under ~/.local/opt/node"
  fi
  install_node
  prepend_user_tools
  require node
  require npm
  current="$(tool_version node)"
  if ! version_ge "$current" "$min"; then
    fail "the installed Node.js ${current} is older than ${min}"
  fi
  info "node: installed (v${current})"
  info "npm: installed ($(npm --version))"
  info "Add this to your shell profile if you want node in new terminals:"
  info '  export PATH="$HOME/.local/opt/node/bin:$PATH"'
}

step "Checking git, Go, and Node"
ensure_git
ensure_go
ensure_node
require npm

step "Go dependencies (go mod download)"
run "go mod download" go mod download

if [ -f web/package-lock.json ]; then
  step "UI dependencies (npm ci)"
  run "npm ci" npm --prefix web ci
else
  step "UI dependencies (npm install)"
  run "npm install" npm --prefix web install
fi

printf '\n%sInstallation complete.%s\n' "$green" "$reset"
printf 'Next step: ./build.sh\n'
