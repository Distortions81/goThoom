#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
OUTPUT_ZIP="${1:-${ROOT_DIR}/binaries/goThoom-Script-Template.zip}"
TEMPLATE_NAME="goThoom-Script-Template"

if [[ "$OUTPUT_ZIP" != /* ]]; then
  OUTPUT_ZIP="$(pwd)/$OUTPUT_ZIP"
fi

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT
TEMPLATE_DIR="$WORK_DIR/$TEMPLATE_NAME"

mkdir -p "$TEMPLATE_DIR/gt2" "$(dirname "$OUTPUT_ZIP")"
cp -a "$ROOT_DIR/script-template/." "$TEMPLATE_DIR/"
cp "$ROOT_DIR/source/scripts/README.md" "$TEMPLATE_DIR/SCRIPTING_GUIDE.md"
cp "$ROOT_DIR/source/gt2/go.mod" "$TEMPLATE_DIR/gt2/go.mod"
cp "$ROOT_DIR/source/gt2/pluginapi.go" "$TEMPLATE_DIR/gt2/pluginapi.go"
cp "$ROOT_DIR/source/gt2/API_REFERENCE.md" "$TEMPLATE_DIR/gt2/API_REFERENCE.md"

(
  cd "$TEMPLATE_DIR"
  go test -tags script ./...
)

rm -f "$OUTPUT_ZIP"
(
  cd "$WORK_DIR"
  zip -q -r "$OUTPUT_ZIP" "$TEMPLATE_NAME"
)

echo "VS Code script template written to $OUTPUT_ZIP"
