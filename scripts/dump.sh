#!/usr/bin/env bash
# run_ebitengine_debug.sh
# Launch a Go/Ebitengine game with the debug mode that dumps sprite atlases.

set -euo pipefail

KEY="i"          # default key to press in-game to dump internal images
MODULE_PATH="."  # default Go module/package to run (current dir)

usage() {
  cat <<EOF
Usage: $0 [-k KEY] [-- GO_RUN_ARGS...]

-k KEY     Key to press in-game to dump atlases (default: 'i')
Anything after -- is passed to 'go run'.
Examples:
  $0
  $0 -k d
  $0 -- -args passed to your game
EOF
}

# Parse flags
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    -k) KEY="${2:-}"; shift 2 ;;
    --) shift; break ;;
    *)  break ;;
  esac
done

# If the user provided an explicit package/dir to run, use it
if [[ $# -gt 0 ]]; then
  MODULE_PATH="$1"; shift
fi

# Confirm Go is installed
command -v go >/dev/null 2>&1 || { echo "Go not found in PATH"; exit 1; }

# Enable Ebitengine's internal image dump key (press this in-game to dump atlases)
export EBITENGINE_INTERNAL_IMAGES_KEY="$KEY"

# Optional: also enable a screenshot key if you want (uncomment to use)
# export EBITENGINE_SCREENSHOT_KEY="q"

echo "==> Running with atlas-dump key: '$EBITENGINE_INTERNAL_IMAGES_KEY'"
echo "==> Build tag: ebitenginedebug (required for internal image dump)"
echo "==> go run ${MODULE_PATH} -- $*"
echo

# Run with the debug build tag enabled
go build -tags ebitenginedebug "${MODULE_PATH}"
./go_client -clmov=clmovFiles/chain.clMov
