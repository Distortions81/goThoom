#!/usr/bin/env bash
set -euo pipefail

OUTPUT_DIR="binaries"
mkdir -p "$OUTPUT_DIR"

platforms=(
  "linux:amd64"
  "windows:amd64"
  #"darwin:amd64"
)

# Install this before building Linux targets:
# sudo apt install libglfw3-dev libgl1-mesa-dev libasound2-dev

for platform in "${platforms[@]}"; do
  IFS=":" read -r GOOS GOARCH <<<"$platform"
  BIN_NAME="thoomspeak-${GOOS}-${GOARCH}"
  ZIP_NAME="${BIN_NAME}.zip"
  if [ "$GOOS" = "windows" ]; then
    BIN_NAME+=".exe"
  fi

  echo "Building ${GOOS}/${GOARCH}..."

  if [ "$GOOS" = "linux" ]; then
    CGO_ENABLED=1
  else
    CGO_ENABLED=0  # Disable cgo for unsupported cross-compilation targets
  fi

  # Build binary with optimization flags to reduce size and speed up execution
  env GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED="$CGO_ENABLED" \
    go build \
      -trimpath \
      -ldflags="-s -w" \
      -o "${OUTPUT_DIR}/${BIN_NAME}" .

  # Zip it
  echo "Zipping ${BIN_NAME}..."
  (
    cd "$OUTPUT_DIR"
    zip -q -m "$ZIP_NAME" "$BIN_NAME"
  )
done

echo "Binaries and zip files are located in ${OUTPUT_DIR}/"
