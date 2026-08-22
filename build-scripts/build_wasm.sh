#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="${SCRIPT_DIR}/.."
OUTPUT_DIR="${ROOT_DIR}/binaries/goThoom-Web"
WASM_OUT="${OUTPUT_DIR}/gothoom.wasm"
WEB_DIR="${ROOT_DIR}/website/wasm"

if [ ! -f "${WEB_DIR}/index.html" ]; then
  echo "Missing website/wasm/index.html. Please create it before building the WASM bundle." >&2
  exit 1
fi

if ! command -v brotli >/dev/null 2>&1; then
  echo "brotli is required to package the WASM bundle." >&2
  exit 1
fi

rm -rf "${OUTPUT_DIR}"
mkdir -p "${OUTPUT_DIR}"

(
  cd "${ROOT_DIR}/source"
  env \
    GOOS=js GOARCH=wasm \
    CGO_ENABLED=0 \
    go build \
      -trimpath \
      -ldflags "-s -w" \
      -o "${WASM_OUT}" .
)

GOROOT="$(go env GOROOT)"
WASM_EXEC="${GOROOT}/misc/wasm/wasm_exec.js"
if [ ! -f "${WASM_EXEC}" ]; then
  WASM_EXEC="$(find "${GOROOT}" -type f -name 'wasm_exec.js' 2>/dev/null | head -n1 || true)"
fi
if [ -z "${WASM_EXEC:-}" ] || [ ! -f "${WASM_EXEC}" ]; then
  echo "wasm_exec.js not found in GOROOT (${GOROOT})." >&2
  exit 1
fi

cp "${WASM_EXEC}" "${OUTPUT_DIR}/"
cp -a "${WEB_DIR}/." "${OUTPUT_DIR}/"
brotli -f "${WASM_OUT}"

echo "WASM bundle ready in ${OUTPUT_DIR} (gothoom.wasm.br only)"
