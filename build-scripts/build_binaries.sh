#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

OUTPUT_DIR="binaries"
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
      sudo apt-get update -qq
      sudo apt-get install -y "$pkg"
    else
      echo "$cmd not found and apt-get unavailable; please install $pkg" >&2
    fi
  fi
}

install_linux_deps() {
  echo "Installing Linux build dependencies..."
  sudo apt-get update -qq
  sudo apt-get install -y git cmake ninja-build clang llvm lldb \
    build-essential g++ pkg-config \
    libxml2-dev uuid-dev libssl-dev libbz2-dev zlib1g-dev \
    cpio unzip zip xz-utils curl \
    g++-12 libstdc++-12-dev libc6-dev
}

ensure_osxcross() {
  # You can override where osxcross is installed by exporting OSXCROSS_ROOT
  OSXCROSS_ROOT="${OSXCROSS_ROOT:-$HOME/osxcross}"
  export OSXCROSS_ROOT

  # If tools already present, just export PATH and return
  if [ -x "$OSXCROSS_ROOT/target/bin/oa64-clang" ] || [ -x "$OSXCROSS_ROOT/target/bin/o64-clang" ]; then
    export PATH="$OSXCROSS_ROOT/target/bin:$PATH"
    return
  fi

  # By default, do not auto-bootstrap osxcross due to common SDK/Clang
  # incompatibilities (e.g., macOS 15.x SDK). Provide a clear error
  # and instructions. Opt-in by setting OSXCROSS_BOOTSTRAP=1.
  if [ "${OSXCROSS_BOOTSTRAP:-0}" != "1" ]; then
    cat >&2 <<'MSG'
macOS cross toolchain not found (o64-clang/oa64-clang missing).

To enable macOS builds, install osxcross and an SDK (recommended: MacOSX13.3.sdk),
then set OSXCROSS_ROOT accordingly. You can run the helper installer:

  ./scripts/install_osxcross.sh --sdk-tarball /path/to/MacOSX13.3.sdk.tar.xz

Or manual steps:

  git clone https://github.com/tpoechtrager/osxcross.git "$HOME/osxcross"
  mkdir -p "$HOME/osxcross/tarballs" && cp MacOSX13.3.sdk.tar.xz "$HOME/osxcross/tarballs"
  (cd "$HOME/osxcross" && UNATTENDED=1 ./build.sh)

Once installed, rerun this script. To let this script attempt a bootstrap
automatically (not recommended), set OSXCROSS_BOOTSTRAP=1.
MSG
    exit 1
  fi

  echo "Bootstrapping osxcross toolchain to $OSXCROSS_ROOT ..."
  sudo apt-get update -qq
  sudo apt-get install -y git cmake ninja-build clang llvm lldb \
    build-essential g++ pkg-config \
    libxml2-dev uuid-dev libssl-dev libbz2-dev zlib1g-dev \
    cpio unzip zip xz-utils curl

  mkdir -p "$OSXCROSS_ROOT"
  if [ ! -d "$OSXCROSS_ROOT/.git" ]; then
    git clone https://github.com/tpoechtrager/osxcross.git "$OSXCROSS_ROOT"
  fi

  mkdir -p "$OSXCROSS_ROOT/tarballs"
  cd "$OSXCROSS_ROOT"

  # You need a macOS SDK tarball. Options:
  # 1) Place it yourself into $OSXCROSS_ROOT/tarballs (e.g. MacOSX13.3.sdk.tar.xz)
  # 2) Set MACOSX_SDK_URL to an SDK tarball URL (the script will download it)
  if [ -n "${MACOSX_SDK_URL:-}" ]; then
    fname="$(basename "$MACOSX_SDK_URL")"
    if [ ! -f "tarballs/$fname" ]; then
      echo "Downloading SDK from $MACOSX_SDK_URL ..."
      curl -L "$MACOSX_SDK_URL" -o "tarballs/$fname"
    fi
  fi

  # Pick an SDK tarball and validate version (avoid known-bad 15.x)
  sdk_file="$(ls -1 tarballs/MacOSX*.sdk.tar.* 2>/dev/null | head -n1 || true)"
  if [ -z "$sdk_file" ]; then
    echo "No macOS SDK found in $OSXCROSS_ROOT/tarballs." >&2
    echo "Place MacOSX*.sdk.tar.* there, or set MACOSX_SDK_URL to a valid SDK tarball and re-run." >&2
    exit 1
  fi
  sdk_base="$(basename "$sdk_file")"
  sdk_ver="$(printf '%s' "$sdk_base" | sed -n 's/^MacOSX\([0-9][0-9]*\)\(\.[0-9][0-9]*\)\?\.sdk.*/\1/p')"
  if [ -n "$sdk_ver" ] && [ "$sdk_ver" -ge 15 ]; then
    echo "Detected SDK $sdk_base (major $sdk_ver), which is often incompatible with osxcross on Linux." >&2
    echo "Use an older SDK like MacOSX13.3.sdk.* and retry." >&2
    exit 1
  fi

  # Build osxcross (unattended)
  UNATTENDED=1 ./build.sh

  # Export toolchain path so o{,a}64-clang is visible
  export PATH="$OSXCROSS_ROOT/target/bin:$PATH"

  # Sanity check
  if ! have oa64-clang && ! have o64-clang; then
    echo "oa64-clang/o64-clang still not found in PATH ($PATH)." >&2
    exit 1
  fi
  cd - >/dev/null
}

# Ensure zip is available for packaging on Ubuntu systems
ensure_cmd zip

for platform in "${platforms[@]}"; do
  IFS=":" read -r GOOS GOARCH <<<"$platform"
  FRIENDLY="${FRIENDLY_NAMES["$GOOS:$GOARCH"]}"
  BIN_NAME="${FRIENDLY}"
  ZIP_NAME="${FRIENDLY}.zip"
  TAGS=""
  LDFLAGS="-s -w"

  if [ "$GOOS" = "windows" ]; then
    BIN_NAME+=".exe"
  fi

  echo "Building ${GOOS}/${GOARCH}..."

  # Default: disable cgo unless explicitly enabled
  CGO_ENABLED=0
  CC=""
  CXX=""

  case "$GOOS:$GOARCH" in
    linux:amd64)
      install_linux_deps
      CGO_ENABLED=1
      ;;
    darwin:arm64)
      ensure_osxcross
      CGO_ENABLED=1
      CC=oa64-clang
      CXX=oa64-clang++
      TAGS="metal"
      ;;
    darwin:amd64)
      ensure_osxcross
      CGO_ENABLED=1
      CC=o64-clang
      CXX=o64-clang++
      TAGS="metal"
      ;;
    *)
      # windows: no system deps; Ebiten uses DirectX without cgo
      ;;
  esac

  if [ "$GOOS" = "windows" ]; then
    LDFLAGS="$LDFLAGS -H=windowsgui"
    if ! command -v go-winres >/dev/null 2>&1; then
      echo "go-winres not found; install with 'go install github.com/tc-hib/go-winres@latest'" >&2
      exit 1
    fi
    rm -f rsrc*.syso
    go-winres simply --icon goThoom.png --arch "$GOARCH" --manifest gui
  fi

  # Make sure nothing forces the OpenGL backend for mac (support old/new env names)
  # Note: unsetting a non-existent var is OK; keep '|| true' for safety under -e
  unset EBITEN_GRAPHICS_LIBRARY EBITENGINE_GRAPHICS_LIBRARY EBITEN_USEGL || true

  # Build argument list safely (avoid embedding quotes into -tags)
  extra_args=()
  if [ -n "$TAGS" ]; then
    extra_args+=( -tags "$TAGS" )
  fi

  env \
    GOOS="$GOOS" GOARCH="$GOARCH" \
    CGO_ENABLED="$CGO_ENABLED" \
    CC="${CC:-}" CXX="${CXX:-}" \
    PATH="${OSXCROSS_ROOT:-$HOME/osxcross}/target/bin:${PATH}" \
    go build \
      -trimpath \
      "${extra_args[@]}" \
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
    APP_NAME="gothoom"
    APP_DIR="${OUTPUT_DIR}/${APP_NAME}.app"

    echo "Creating ${APP_NAME}.app bundle..."
    rm -rf "$APP_DIR"
      mkdir -p "$APP_DIR/Contents/MacOS"
      cp "${OUTPUT_DIR}/${BIN_NAME}" "$APP_DIR/Contents/MacOS/${APP_NAME}"
      mkdir -p "$APP_DIR/Contents/Resources"
      ensure_cmd convert imagemagick
      convert "$SCRIPT_DIR/../goThoom.png" -define icon:auto-resize=16,32,64,128,256,512 "$APP_DIR/Contents/Resources/goThoom.icns"
      cat <<'EOF' >"$APP_DIR/Contents/Info.plist"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>
  <string>gothoom</string>
  <key>CFBundleIdentifier</key>
  <string>com.goThoom.client</string>
  <key>CFBundleName</key>
  <string>gothoom</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleVersion</key>
  <string>1.0</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0</string>
    <key>CFBundleIconFile</key>
    <string>goThoom.icns</string>
    <key>com.apple.security.app-sandbox</key>
    <true/>
  </dict>
</plist>
EOF

    if command -v rcodesign >/dev/null 2>&1; then
      echo "Ad-hoc signing ${APP_NAME}.app with rcodesign..."
      rcodesign sign "$APP_DIR" || echo "rcodesign sign failed, continuing" >&2
      rcodesign verify --verbose "$APP_DIR/Contents/MacOS/gothoom" || echo "rcodesign verify failed, continuing" >&2
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

  PKG_DIR="${OUTPUT_DIR}/goThoom"
  rm -rf "$PKG_DIR"
  mkdir -p "$PKG_DIR"

  if [ "$GOOS" = "darwin" ]; then
    mv "$APP_DIR" "$PKG_DIR/"
  else
    mv "${OUTPUT_DIR}/${BIN_NAME}" "$PKG_DIR/"
  fi

  if [ "$GOOS" = "linux" ]; then
    ensure_cmd convert imagemagick
    ICON_DIR="$PKG_DIR/share/icons/hicolor/256x256/apps"
    DESKTOP_DIR="$PKG_DIR/share/applications"
    mkdir -p "$ICON_DIR" "$DESKTOP_DIR"
    convert "$SCRIPT_DIR/../goThoom.png" -resize 256x256 "$ICON_DIR/goThoom.png"
    cat <<'EOF' >"$DESKTOP_DIR/goThoom.desktop"
[Desktop Entry]
Type=Application
Name=goThoom
Exec=goThoom
Icon=goThoom
Categories=Game;
EOF
  fi
  (
    cd "$OUTPUT_DIR"
    zip -q -r "$ZIP_NAME" "goThoom"
    rm -rf "goThoom"
  )
done

echo "Binaries and zip files are located in ${OUTPUT_DIR}/"
