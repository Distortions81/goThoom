#!/usr/bin/env bash
# Build a tarball containing goThoom's Go toolchain, module cache, and
# selected data files. The resulting archive can be unpacked on another
# machine to speed up environment setup without hitting the network for Go
# artifacts.

set -euo pipefail

# Usage: build-scripts/build_dep_bundle.sh [output.tar.gz]
# Default output is gothoom_deps.tar.gz in the current directory.

OUT_FILE="${1:-gothoom_deps.tar.gz}"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT
GO_DIR="$WORK_DIR/go"

mkdir -p "$GO_DIR"

# System packages are not bundled; install them separately via your package
# manager before using this archive.

# Include Go toolchain
# NOTE: If this version changes, update AGENTS.md instructions to match.
GO_VERSION="1.25.0"
GO_TARBALL="go${GO_VERSION}.linux-amd64.tar.gz"
echo "Downloading Go ${GO_VERSION}..."
curl -L -o "$GO_DIR/$GO_TARBALL" "https://go.dev/dl/$GO_TARBALL"

# Cache Go modules into a local mod cache inside the bundle.
GO_CACHE="$GO_DIR/mod"
mkdir -p "$GO_CACHE"

echo "Downloading Go modules..."
GOMODCACHE="$GO_CACHE" /usr/local/go/bin/go mod download


# Copy useful data files
DATA_SRC="data"
DATA_DST="$WORK_DIR/data"

# List only the files you want to include
WHITELIST=(
  #"CL_Images"
  #"CL_Sounds"
  "font/NotoSans-Regular.ttf"
  "font/NotoSans-Bold.ttf"
  "font/NotoSans-Italic.ttf"
  "font/NotoSans-BoldItalic.ttf"

  "font/NotoSansMono-Regular.ttf"
  "font/NotoSansMono-Bold.ttf"

  #"soundfont.sf2"
)

if [ -d "$DATA_SRC" ]; then
  echo "Copying whitelisted data files..."
  mkdir -p "$DATA_DST"
  for file in "${WHITELIST[@]}"; do
    if [ -f "$DATA_SRC/$file" ]; then
      mkdir -p "$DATA_DST/$(dirname "$file")"
      cp -a "$DATA_SRC/$file" "$DATA_DST/$file"
    else
      echo "Warning: $file not found in $DATA_SRC, skipping."
    fi
  done
else
  echo "No data directory found; skipping data copy."
fi

cp -a "spellcheck_words.txt" "$WORK_DIR/"

# Create archive.

tar -czf "$OUT_FILE" -C "$WORK_DIR" .

echo "Dependency bundle written to $OUT_FILE"
