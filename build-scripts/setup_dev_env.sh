#!/usr/bin/env bash
# Fully set up the development environment, including headless support.
# This script is intended for Debian/Ubuntu based systems.

set -euo pipefail

if ! command -v apt-get >/dev/null 2>&1; then
  echo "apt-get not found. Please install dependencies manually." >&2
  exit 1
fi

sudo apt-get update
sudo apt-get install -y golang-go build-essential libgl1-mesa-dev \
  libglu1-mesa-dev xorg-dev xvfb pkg-config libasound2-dev libgtk-3-dev

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
