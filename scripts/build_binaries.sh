#!/usr/bin/env bash
set -euo pipefail

OUTPUT_DIR="binaries"
mkdir -p "$OUTPUT_DIR"

platforms=(
  "linux:amd64"
  "darwin:amd64"
  "windows:amd64"
)

for platform in "${platforms[@]}"; do
  IFS=":" read -r GOOS GOARCH <<<"$platform"
  BIN_NAME="thoomspeak-${GOOS}-${GOARCH}"
  if [ "$GOOS" = "windows" ]; then
    BIN_NAME+=".exe"
  fi
  echo "Building ${GOOS}/${GOARCH}..."
  env GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=1 \
    go build -o "${OUTPUT_DIR}/${BIN_NAME}" .
done

echo "Binaries are located in ${OUTPUT_DIR}/"
