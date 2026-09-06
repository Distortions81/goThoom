#!/usr/bin/env bash
# Verify EUI builds and tests without any client packages or bundled game data.
set -euo pipefail
repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
eui_check_dir=$(mktemp -d)
trap 'rm -rf -- "$eui_check_dir"' EXIT
cp -R "$repo_root/source/eui/." "$eui_check_dir/"
cp "$repo_root/source/go.sum" "$eui_check_dir/go.sum"
cd "$eui_check_dir"
export GOWORK=off
# Keep the current import prefix for this extraction check. A published repo
# will replace this prefix with its actual module path.
go mod init gothoom/eui
go mod edit -go=1.26.6 \
  -require=github.com/hajimehoshi/ebiten/v2@v2.9.10 \
  -require=golang.design/x/clipboard@v0.9.0 \
  -require=golang.org/x/image@v0.45.0 \
  -require=golang.org/x/time@v0.15.0
go mod tidy
go test ./...
go build ./examples/basic
