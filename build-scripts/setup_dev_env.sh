#!/usr/bin/env bash
# Fully set up the development environment, including headless support.
# This script is intended for Debian/Ubuntu based systems.

set -euo pipefail

GO_VERSION="${GO_VERSION:-1.26.6}"
export PATH="/usr/local/go/bin:$PATH"

if ! command -v apt-get >/dev/null 2>&1; then
  echo "apt-get not found. Please install dependencies manually." >&2
  exit 1
fi

sudo apt-get update
# Newer Debian/Ubuntu releases renamed the ALSA runtime for 64-bit time_t.
alsa_package=libasound2
if apt-cache show libasound2t64 >/dev/null 2>&1; then
  alsa_package=libasound2t64
fi
sudo apt-get install -y libgl1 libegl1 libgl1-mesa-dri libx11-6 libxcursor1 libxi6 \
  libxinerama1 libxrandr2 libxxf86vm1 "$alsa_package" xvfb xauth zenity \
  curl ca-certificates

export CGO_ENABLED=0

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

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}/../source"

bash "${SCRIPT_DIR}/download_emoji_font.sh"
bash "${SCRIPT_DIR}/download_emoji_data.sh"
go mod download
go fmt ./...
go vet ./...
go build ./...
go test ./...

echo "Development environment setup complete."
