#!/usr/bin/env bash
# Verify EUI builds and tests without any client packages or bundled game data.
set -euo pipefail
repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
eui_check_dir=$(mktemp -d)
trap 'rm -rf -- "$eui_check_dir"' EXIT
mkdir -p "$eui_check_dir/eui" "$eui_check_dir/internal"
cp -R "$repo_root/source/eui/." "$eui_check_dir/eui/"
cp -R "$repo_root/source/internal/inputkeys" "$eui_check_dir/internal/"
cp "$repo_root/source/go.sum" "$eui_check_dir/go.sum"
cd "$eui_check_dir"
export GOWORK=off
export CGO_ENABLED=0
# Keep the current import prefix for this extraction check. A published repo
# will replace this prefix with its actual module path.
go mod init gothoom
go mod edit -go=1.27.1 \
  -require=github.com/hajimehoshi/ebiten/v2@v2.10.1 \
  -require=golang.design/x/clipboard@v0.9.0 \
  -require=golang.org/x/image@v0.46.0 \
  -require=golang.org/x/time@v0.16.0
go mod tidy
go test ./...
go build ./eui/examples/basic
