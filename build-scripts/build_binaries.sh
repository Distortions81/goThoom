#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}/source"

OUTPUT_DIR="${ROOT_DIR}/binaries"
mkdir -p "$OUTPUT_DIR"

platforms=(
  "linux:amd64"
  #"linux:arm64"
  "windows:amd64"
  "darwin:arm64"
  "darwin:amd64"
)

declare -A FRIENDLY_NAMES=(
  ["linux:amd64"]="goThoom-Linux-x86_64"
  ["windows:amd64"]="goThoom-Windows-x86_64"
  ["darwin:arm64"]="goThoom-macOS-AppleSilicon"
  ["darwin:amd64"]="goThoom-macOS-Intel"
)

have() { command -v "$1" >/dev/null 2>&1; }

ensure_cmd() {
  local cmd="$1"
  local pkg="${2:-$1}"
  if ! have "$cmd"; then
    if have apt-get; then
      echo "Installing $pkg..."
      apt-get update -qq
      apt-get install -y "$pkg"
    else
      echo "$cmd not found and apt-get unavailable; please install $pkg" >&2
    fi
  fi
}

ensure_spellcheck_dict() {
  local dict="${SCRIPT_DIR}/../source/spellcheck_words.txt"
  if [ ! -f "$dict" ]; then
    ensure_cmd curl curl
    bash "${SCRIPT_DIR}/download_spellcheck_dict.sh"
  fi
}

# Ensure zip is available for packaging on Ubuntu systems
ensure_cmd zip
ensure_spellcheck_dict
bash "${SCRIPT_DIR}/build_script_template.sh" "${OUTPUT_DIR}/goThoom-Script-Template.zip"

APP_VERSION="$(
  sed -nE 's/^[[:space:]]*"Version":[[:space:]]*([0-9]+),?[[:space:]]*$/\1/p' data/versions.json |
    sort -n |
    tail -n 1
)"
if [[ ! "$APP_VERSION" =~ ^[0-9]+$ ]]; then
  echo "Could not determine the app version from source/data/versions.json" >&2
  exit 1
fi

for platform in "${platforms[@]}"; do
  IFS=":" read -r GOOS GOARCH <<<"$platform"
  FRIENDLY="${FRIENDLY_NAMES["$GOOS:$GOARCH"]}"
  BIN_NAME="goThoom-${APP_VERSION}"
  ZIP_NAME="${FRIENDLY}.zip"
  LDFLAGS="-s -w"

  if [ "$GOOS" = "windows" ]; then
    BIN_NAME+=".exe"
  fi

  echo "Building ${GOOS}/${GOARCH}..."

  # Desktop dependencies support pure Go cross-compilation.
  if [ "$GOOS" = "windows" ]; then
    LDFLAGS="$LDFLAGS -H=windowsgui"
    if ! command -v go-winres >/dev/null 2>&1; then
      echo "go-winres not found; install with 'go install github.com/tc-hib/go-winres@latest'" >&2
      exit 1
    fi
    rm -f rsrc*.syso
    go-winres simply --icon logo.png --arch "$GOARCH" --manifest gui
  fi

  # Make sure nothing forces the OpenGL backend for mac (support old/new env names)
  # Note: unsetting a non-existent var is OK; keep '|| true' for safety under -e
  unset EBITEN_GRAPHICS_LIBRARY EBITENGINE_GRAPHICS_LIBRARY EBITEN_USEGL || true

  env \
    GOOS="$GOOS" GOARCH="$GOARCH" \
    CGO_ENABLED=0 \
    go build \
      -trimpath \
      -ldflags "$LDFLAGS" \
      -o "${OUTPUT_DIR}/${BIN_NAME}" .

  if [ "$GOOS" = "windows" ]; then
    cert_file="${WINDOWS_CERT_FILE:-${SCRIPT_DIR}/fullchain.pem}"
    key_file="${WINDOWS_KEY_FILE:-${SCRIPT_DIR}/privkey.pem}"
    ensure_cmd osslsigncode osslsigncode
    if command -v osslsigncode >/dev/null 2>&1 && [ -f "$cert_file" ] && [ -f "$key_file" ]; then
      echo "Signing ${BIN_NAME}..."
      signed_tmp="${OUTPUT_DIR}/${BIN_NAME}.signed"
      osslsigncode sign \
        -certs "$cert_file" \
        -key "$key_file" \
        ${WINDOWS_KEY_PASS:+-pass "$WINDOWS_KEY_PASS"} \
        -n "${WINDOWS_CERT_NAME:-goThoom}" \
        ${WINDOWS_TIMESTAMP_URL:+-t "$WINDOWS_TIMESTAMP_URL"} \
        -in "${OUTPUT_DIR}/${BIN_NAME}" \
        -out "$signed_tmp"
      mv "$signed_tmp" "${OUTPUT_DIR}/${BIN_NAME}"
    else
      echo "Skipping Windows signing; osslsigncode or certificate not configured." >&2
    fi
    rm -f rsrc*.syso
  fi
  if [ "$GOOS" = "darwin" ]; then
    APP_NAME="goThoom-${APP_VERSION}"
    MAC_EXECUTABLE="gothoom"
    APP_DIR="${OUTPUT_DIR}/${APP_NAME}.app"

    echo "Creating ${APP_NAME}.app bundle..."
    rm -rf "$APP_DIR"
      mkdir -p "$APP_DIR/Contents/MacOS"
      cp "${OUTPUT_DIR}/${BIN_NAME}" "$APP_DIR/Contents/MacOS/${MAC_EXECUTABLE}"
      mkdir -p "$APP_DIR/Contents/Resources"
      ensure_cmd convert imagemagick
      convert "$ROOT_DIR/source/logo.png" -define icon:auto-resize=16,32,64,128,256,512 "$APP_DIR/Contents/Resources/goThoom.icns"
      cat <<EOF >"$APP_DIR/Contents/Info.plist"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>
  <string>${MAC_EXECUTABLE}</string>
  <key>CFBundleIdentifier</key>
  <string>com.goThoom.client</string>
  <key>CFBundleName</key>
  <string>${APP_NAME}</string>
  <key>CFBundleDisplayName</key>
  <string>${APP_NAME}</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleVersion</key>
  <string>${APP_VERSION}</string>
    <key>CFBundleShortVersionString</key>
    <string>${APP_VERSION}</string>
    <key>CFBundleIconFile</key>
    <string>goThoom.icns</string>
    <key>com.apple.security.app-sandbox</key>
    <true/>
  </dict>
</plist>
EOF

    if command -v rcodesign >/dev/null 2>&1; then
      echo "Ad-hoc signing ${APP_NAME}.app with rcodesign..."
      rcodesign -C /dev/null sign "$APP_DIR" || echo "rcodesign sign failed, continuing" >&2
    elif command -v codesign >/dev/null 2>&1; then
      echo "Codesigning ${APP_NAME}.app..."
      MAC_ENTITLEMENTS="${MAC_ENTITLEMENTS:-${SCRIPT_DIR}/goThoom.entitlements}"
      if [ -f "$MAC_ENTITLEMENTS" ]; then
        codesign --force --deep --sign "${MAC_SIGN_IDENTITY:--}" --entitlements "$MAC_ENTITLEMENTS" "$APP_DIR" || echo "codesign failed, continuing" >&2
      else
        codesign --force --deep --sign "${MAC_SIGN_IDENTITY:--}" "$APP_DIR" || echo "codesign failed, continuing" >&2
      fi
    else
      echo "rcodesign/codesign not found; skipping macOS signing." >&2
    fi
    rm "${OUTPUT_DIR}/${BIN_NAME}"
  fi

  (
    cd "$OUTPUT_DIR"
    # zip updates existing archives in place, so remove the old archive first
    # to ensure deleted package files cannot survive from an earlier build.
    rm -f "$ZIP_NAME"
    if [ "$GOOS" = "darwin" ]; then
      zip -q -r "$ZIP_NAME" "${APP_NAME}.app"
      rm -rf "${APP_NAME}.app"
    else
      zip -q "$ZIP_NAME" "$BIN_NAME"
      rm -f "$BIN_NAME"
    fi
  )
done

echo "Binaries and zip files are located in ${OUTPUT_DIR}/"
