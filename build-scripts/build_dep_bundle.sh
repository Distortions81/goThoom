#!/usr/bin/env bash
# Build a tarball containing selected data files. The resulting archive can
# be unpacked on another machine to speed up environment setup without
# hitting the network for these assets.

set -euo pipefail

# Usage: build-scripts/build_dep_bundle.sh [output.tar.gz]
# Default output is gothoom_deps.tar.gz in the current directory.

OUT_FILE="${1:-gothoom_deps.tar.gz}"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

# System packages are not bundled; install them separately via your package
# manager before using this archive.

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
