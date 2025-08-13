#!/usr/bin/env bash
set -euo pipefail

OUTPUT_DIR="binaries"
mkdir -p "$OUTPUT_DIR"

platforms=(
  "linux:amd64"
  "windows:amd64"
  "darwin:arm64"
  "darwin:amd64"
)

have() { command -v "$1" >/dev/null 2>&1; }

install_linux_deps() {
  echo "Installing Linux build dependencies..."
  sudo apt-get update -qq
  sudo apt-get install -y git cmake ninja-build clang llvm lldb \
    build-essential g++ pkg-config \
    libxml2-dev uuid-dev libssl-dev libbz2-dev zlib1g-dev \
    cpio unzip xz-utils curl \
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

  echo "Installing osxcross toolchain to $OSXCROSS_ROOT ..."
  sudo apt-get update -qq
  sudo apt-get install -y git cmake ninja-build clang llvm lldb \
    build-essential g++ pkg-config \
    libxml2-dev uuid-dev libssl-dev libbz2-dev zlib1g-dev \
    cpio unzip xz-utils curl

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

  # Check if we have any SDK tarballs now
  if ! ls tarballs/MacOSX*.sdk.tar.* >/dev/null 2>&1; then
    echo "No macOS SDK found in $OSXCROSS_ROOT/tarballs."
    echo "Place MacOSX*.sdk.tar.* there, or set MACOSX_SDK_URL to a valid SDK tarball and re-run."
    exit 1
  fi

  # Build osxcross (unattended)
  UNATTENDED=1 ./build.sh

  # Export toolchain path so o{,a}64-clang is visible
  export PATH="$OSXCROSS_ROOT/target/bin:$PATH"

  # Sanity check
  if ! have oa64-clang && ! have o64-clang; then
    echo "oa64-clang/o64-clang still not found in PATH ($PATH)."
    exit 1
  fi
  cd - >/dev/null
}

for platform in "${platforms[@]}"; do
  IFS=":" read -r GOOS GOARCH <<<"$platform"
  BIN_NAME="thoomspeak-${GOOS}-${GOARCH}"
  ZIP_NAME="${BIN_NAME}.zip"
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

  # Make sure nothing forces the OpenGL backend for mac
  unset EBITENGINE_OPENGL || true

  env \
    GOOS="$GOOS" GOARCH="$GOARCH" \
    CGO_ENABLED="$CGO_ENABLED" \
    CC="${CC:-}" CXX="${CXX:-}" \
    PATH="${OSXCROSS_ROOT:-$HOME/osxcross}/target/bin:${PATH}" \
    go build \
      -trimpath \
      ${TAGS:+-tags="$TAGS"} \
      -ldflags="$LDFLAGS" \
      -o "${OUTPUT_DIR}/${BIN_NAME}" .
  if [ "$GOOS" = "darwin" ]; then
    APP_NAME="ThoomSpeak"
    APP_DIR="${OUTPUT_DIR}/${APP_NAME}.app"

    echo "Creating ${APP_NAME}.app bundle..."
    rm -rf "$APP_DIR"
    mkdir -p "$APP_DIR/Contents/MacOS"
    cp "${OUTPUT_DIR}/${BIN_NAME}" "$APP_DIR/Contents/MacOS/${APP_NAME}"
    cat <<'EOF' >"$APP_DIR/Contents/Info.plist"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>
  <string>ThoomSpeak</string>
  <key>CFBundleIdentifier</key>
  <string>com.goThoom.client</string>
  <key>CFBundleName</key>
  <string>ThoomSpeak</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleVersion</key>
  <string>1.0</string>
  <key>CFBundleShortVersionString</key>
  <string>1.0</string>
</dict>
</plist>
EOF

    echo "Zipping ${APP_NAME}.app..."
    (
      cd "$OUTPUT_DIR"
      zip -q -r "$ZIP_NAME" "${APP_NAME}.app"
      rm -rf "${APP_NAME}.app" "${BIN_NAME}"
    )
  else
    echo "Zipping ${BIN_NAME}..."
    (
      cd "$OUTPUT_DIR"
      zip -q -m "$ZIP_NAME" "$BIN_NAME"
    )
  fi
done

echo "Binaries and zip files are located in ${OUTPUT_DIR}/"
