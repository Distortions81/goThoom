#!/usr/bin/env bash
set -euo pipefail

# install_osxcross.sh — bootstrap osxcross and a macOS SDK
#
# Usage examples:
#   scripts/install_osxcross.sh \
#     --root "$HOME/osxcross" \
#     --sdk-tarball "/path/to/MacOSX13.3.sdk.tar.xz"
#
#   scripts/install_osxcross.sh \
#     --sdk-url "https://example/MacOSX13.3.sdk.tar.xz"
#
#   scripts/install_osxcross.sh \
#     --sdk-version 13.3 \
#     --sdks-json "https://raw.githubusercontent.com/joseluisq/macosx-sdks/master/macosx_sdks.json"
#
#   OSXCROSS_ALLOW_SDK15=1 scripts/install_osxcross.sh --sdk-tarball ./MacOSX15.5.sdk.tar.xz
#
# Notes:
# - Installs required build deps on Debian/Ubuntu if apt-get is available
# - Prefers SDK <= 14; blocks 15.x unless OSXCROSS_ALLOW_SDK15=1
# - After success, add "$OSXCROSS_ROOT/target/bin" to your PATH

have() { command -v "$1" >/dev/null 2>&1; }

msg() { printf "[osxcross] %s\n" "$*"; }
err() { printf "[osxcross][ERROR] %s\n" "$*" >&2; }

print_help() {
  cat <<'EOF'
install_osxcross.sh — bootstrap osxcross and a macOS SDK

Flags:
  --root DIR         Install location (default: $HOME/osxcross)
  --sdk-url URL      Download SDK tarball into osxcross/tarballs
  --sdk-tarball PATH Copy existing SDK tarball into osxcross/tarballs
  --sdk-version X.Y  Resolve and download SDK matching version X or X.Y from JSON index
  --sdks-json URL    JSON index URL (default: joseluisq/macosx-sdks macosx_sdks.json)
  --branch NAME      osxcross git branch (default: master)
  --no-deps          Skip apt-get dependency installation
  -h, --help         Show this help

Env:
  OSXCROSS_ALLOW_SDK15=1   Allow SDK 15.x (unsupported; may fail)

You must provide either --sdk-url or --sdk-tarball, or place
MacOSX*.sdk.tar.* into $OSXCROSS_ROOT/tarballs beforehand.
EOF
}

OSXCROSS_ROOT="${OSXCROSS_ROOT:-$HOME/osxcross}"
SDK_URL=""
SDK_TARBALL=""
SDK_VERSION=""
SDKS_JSON_URL="${SDKS_JSON_URL:-https://raw.githubusercontent.com/joseluisq/macosx-sdks/master/macosx_sdks.json}"
OSXCROSS_BRANCH="${OSXCROSS_BRANCH:-master}"
INSTALL_DEPS=1

while [ $# -gt 0 ]; do
  case "$1" in
    --root) OSXCROSS_ROOT="$2"; shift 2;;
    --sdk-url) SDK_URL="$2"; shift 2;;
    --sdk-tarball) SDK_TARBALL="$2"; shift 2;;
    --sdk-version) SDK_VERSION="$2"; shift 2;;
    --sdks-json) SDKS_JSON_URL="$2"; shift 2;;
    --branch) OSXCROSS_BRANCH="$2"; shift 2;;
    --no-deps) INSTALL_DEPS=0; shift;;
    -h|--help) print_help; exit 0;;
    *) err "Unknown arg: $1"; print_help; exit 2;;
  esac
done

msg "Target root: $OSXCROSS_ROOT (branch: $OSXCROSS_BRANCH)"

if [ "$INSTALL_DEPS" = 1 ] && have apt-get; then
  msg "Installing dependencies via apt-get..."
  sudo apt-get update -qq
  sudo apt-get install -y git cmake ninja-build clang llvm lldb \
    build-essential g++ pkg-config \
    libxml2-dev uuid-dev libssl-dev libbz2-dev zlib1g-dev \
    cpio unzip zip xz-utils curl jq
else
  msg "Skipping dependency installation or non-apt system. Ensure required packages are present."
fi

mkdir -p "$OSXCROSS_ROOT"
if [ ! -d "$OSXCROSS_ROOT/.git" ]; then
  msg "Cloning osxcross into $OSXCROSS_ROOT ..."
  git clone --depth 1 --branch "$OSXCROSS_BRANCH" https://github.com/tpoechtrager/osxcross.git "$OSXCROSS_ROOT"
else
  msg "osxcross already present at $OSXCROSS_ROOT"
fi

mkdir -p "$OSXCROSS_ROOT/tarballs"

if [ -n "$SDK_URL" ]; then
  fname="$(basename "$SDK_URL")"
  if [ ! -f "$OSXCROSS_ROOT/tarballs/$fname" ]; then
    msg "Downloading SDK: $SDK_URL"
    curl -L "$SDK_URL" -o "$OSXCROSS_ROOT/tarballs/$fname"
  else
    msg "SDK already downloaded: $fname"
  fi
fi

if [ -n "$SDK_TARBALL" ]; then
  if [ ! -f "$SDK_TARBALL" ]; then
    err "SDK tarball not found: $SDK_TARBALL"; exit 1
  fi
  msg "Copying SDK tarball into tarballs/ ..."
  cp -n "$SDK_TARBALL" "$OSXCROSS_ROOT/tarballs/"
fi

# Resolve SDK URL from JSON if version provided and no URL/TARBALL given
if [ -z "$SDK_URL" ] && [ -z "$SDK_TARBALL" ] && [ -n "$SDK_VERSION" ]; then
  tmpjson="$(mktemp)"
  msg "Fetching SDKs index: $SDKS_JSON_URL"
  curl -L "$SDKS_JSON_URL" -o "$tmpjson"

  # Normalize version pattern for regex matching
  ver_re="$(printf '%s' "$SDK_VERSION" | sed 's/\./\\./g')"

  resolved_url=""
  if have jq; then
    # Look for any string fields that contain both the SDK filename and an http(s) URL
    resolved_url=$(jq -r --arg re "MacOSX${ver_re}\\.sdk\\.tar\\." '
      .. | strings | select(test($re)) | select(test("^https?://")) | .
    ' "$tmpjson" | head -n1 || true)
  fi

  if [ -z "$resolved_url" ] && have python3; then
    resolved_url=$(python3 - "$tmpjson" "$SDK_VERSION" <<'PY'
import json, re, sys
path, ver = sys.argv[1], sys.argv[2]
pat = re.compile(r"MacOSX" + re.escape(ver) + r"\.sdk\.tar\.")
def walk(x):
    if isinstance(x, dict):
        for v in x.values():
            yield from walk(v)
    elif isinstance(x, list):
        for v in x:
            yield from walk(v)
    elif isinstance(x, str):
        if pat.search(x) and x.startswith(("http://","https://")):
            yield x

with open(path,'r', encoding='utf-8') as f:
    data = json.load(f)
for s in walk(data):
    print(s)
    break
PY
)
  fi

  rm -f "$tmpjson"

  if [ -z "$resolved_url" ]; then
    err "Could not resolve download URL for SDK version $SDK_VERSION from JSON index."
    err "Specify --sdk-url or --sdk-tarball instead, or provide a different --sdks-json."
    exit 1
  fi

  SDK_URL="$resolved_url"
  msg "Resolved SDK URL: $SDK_URL"
  fname="$(basename "$SDK_URL")"
  if [ ! -f "$OSXCROSS_ROOT/tarballs/$fname" ]; then
    msg "Downloading SDK: $SDK_URL"
    curl -L "$SDK_URL" -o "$OSXCROSS_ROOT/tarballs/$fname"
  else
    msg "SDK already downloaded: $fname"
  fi
fi

# Choose an SDK tarball
sdk_file="$(ls -1 "$OSXCROSS_ROOT"/tarballs/MacOSX*.sdk.tar.* 2>/dev/null | head -n1 || true)"
if [ -z "$sdk_file" ]; then
  err "No SDK tarball found in $OSXCROSS_ROOT/tarballs."
  err "Provide --sdk-url or --sdk-tarball, or place MacOSX*.sdk.tar.* there."
  exit 1
fi

sdk_base="$(basename "$sdk_file")"
sdk_ver_major="$(printf '%s' "$sdk_base" | sed -n 's/^MacOSX\([0-9][0-9]*\)\(\.[0-9][0-9]*\)\?\.sdk.*/\1/p')"
if [ -n "$sdk_ver_major" ] && [ "$sdk_ver_major" -ge 15 ] && [ "${OSXCROSS_ALLOW_SDK15:-0}" != "1" ]; then
  err "Detected SDK $sdk_base (major $sdk_ver_major). This may fail with osxcross on Linux."
  err "Use MacOSX13.3.sdk or 14.x, or set OSXCROSS_ALLOW_SDK15=1 to continue."
  exit 1
fi

msg "Using SDK: $sdk_base"

(
  cd "$OSXCROSS_ROOT"
  msg "Building osxcross (this can take a while)..."
  UNATTENDED=1 ./build.sh
)

TARGET_BIN="$OSXCROSS_ROOT/target/bin"
if [ ! -x "$TARGET_BIN/o64-clang" ] && [ ! -x "$TARGET_BIN/oa64-clang" ]; then
  err "osxcross build did not produce o64-clang/oa64-clang in $TARGET_BIN"
  err "Check build logs above."
  exit 1
fi

msg "Done. Add to your shell profile or current env:"
echo "  export OSXCROSS_ROOT=\"$OSXCROSS_ROOT\""
echo "  export PATH=\"$TARGET_BIN:\$PATH\""

msg "Verify with: which o64-clang || which oa64-clang"
