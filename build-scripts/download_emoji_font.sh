#!/usr/bin/env bash
# Restore the bundled color emoji font from a pinned upstream revision.
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
font_dir="${script_dir}/../source/data/font"
revision=8998f5dd683424a73e2314a8c1f1e359c19e8742
font_sha=72a635cb3d2f3524c51620cdde406b217204e8a6a06c6a096ff8ed4b5fd6e27b
license_sha=6a73f9541c2de74158c0e7cf6b0a58ef774f5a780bf191f2d7ec9cc53efe2bf2
verify() { printf '%s  %s\n' "$1" "$2" | sha256sum --check --status; }
if [[ -f "${font_dir}/NotoColorEmoji.ttf" && -f "${font_dir}/NotoColorEmoji-LICENSE.txt" ]] \
  && verify "$font_sha" "${font_dir}/NotoColorEmoji.ttf" \
  && verify "$license_sha" "${font_dir}/NotoColorEmoji-LICENSE.txt"; then
  exit 0
fi
download_dir="$(mktemp -d)"
trap 'rm -rf "$download_dir"' EXIT
url="https://raw.githubusercontent.com/googlefonts/noto-emoji/${revision}/fonts"
curl -fsSL "${url}/NotoColorEmoji.ttf" -o "${download_dir}/NotoColorEmoji.ttf"
curl -fsSL "${url}/LICENSE" -o "${download_dir}/NotoColorEmoji-LICENSE.txt"
verify "$font_sha" "${download_dir}/NotoColorEmoji.ttf"
verify "$license_sha" "${download_dir}/NotoColorEmoji-LICENSE.txt"
mkdir -p "$font_dir"
mv "${download_dir}/NotoColorEmoji.ttf" "${download_dir}/NotoColorEmoji-LICENSE.txt" "$font_dir/"
