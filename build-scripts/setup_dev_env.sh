#!/usr/bin/env bash
# Fully set up the development environment, including headless support.
# This script is intended for Debian/Ubuntu based systems.

set -euo pipefail

GO_VERSION="${GO_VERSION:-1.26.3}"
export PATH="/usr/local/go/bin:$PATH"

if ! command -v apt-get >/dev/null 2>&1; then
  echo "apt-get not found. Please install dependencies manually." >&2
  exit 1
fi

sudo apt-get update
sudo apt-get install -y build-essential libgl1-mesa-dev \
  libglu1-mesa-dev xorg-dev xvfb pkg-config libasound2-dev libgtk-3-dev \
  curl ca-certificates

required_go="go${GO_VERSION}"
if ! command -v go >/dev/null 2>&1 || [[ "$(go version | awk '{print $3}')" != "${required_go}" ]]; then
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "${tmpdir}"' EXIT
  curl -fsSLo "${tmpdir}/${required_go}.linux-amd64.tar.gz" \
    "https://go.dev/dl/${required_go}.linux-amd64.tar.gz"
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf "${tmpdir}/${required_go}.linux-amd64.tar.gz"
fi

# Start Xvfb for headless environments if not already running
if ! pgrep -x Xvfb >/dev/null 2>&1; then
  echo "Starting Xvfb on display :99..."
  Xvfb :99 -screen 0 1024x768x24 >/tmp/Xvfb.log 2>&1 &
  disown
fi
export DISPLAY=${DISPLAY:-:99}

go mod download
go fmt ./...
go vet ./...
go build ./...
go test ./...

echo "Development environment setup complete."
