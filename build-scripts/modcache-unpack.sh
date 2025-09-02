#!/usr/bin/env bash
set -euo pipefail

# Unpack offline Go pack (modcache + toolchain) for bunker builds.
# Usage: modcache-unpack.sh <offline-go-*.tar.{zst,xz,gz}> [target-dir=.]

ARCHIVE="${1:-}"
TARGET="${2:-.}"

[[ -n "$ARCHIVE" && -f "$ARCHIVE" ]] || { echo "Archive not found"; exit 1; }

case "$ARCHIVE" in
  *.tar.zst) DECOMP="zstd -d -q -c" ;;
  *.tar.xz)  DECOMP="xz -d -c" ;;
  *.tar.gz|*.tgz) DECOMP="gzip -d -c" ;;
  *) echo "Unknown archive type: $ARCHIVE" ; exit 1 ;;
esac

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

$DECOMP "$ARCHIVE" | tar -C "$TMPDIR" -xf -

mkdir -p "$TARGET"
rsync -a --delete "$TMPDIR/modcache/"      "$TARGET/modcache/"
rsync -a --delete "$TMPDIR/toolchain/"     "$TARGET/toolchain/"
[[ -f "$TMPDIR/README-OFFLINE.txt" ]] && cp "$TMPDIR/README-OFFLINE.txt" "$TARGET/"
[[ -f "$TMPDIR/env.sh" ]]            && cp "$TMPDIR/env.sh" "$TARGET/" && chmod +x "$TARGET/env.sh"

echo "Restored to: $(cd "$TARGET" && pwd)"
echo
echo "Use these env vars (or run '$TARGET/env.sh'):"
echo "  export GOPROXY=off"
echo "  export GOSUMDB=off"
echo "  export GOMODCACHE=\"$(cd "$TARGET" && pwd)/modcache\""
echo "  export GOROOT=\"$(cd "$TARGET" && pwd)/toolchain/go\""
echo "  export PATH=\"\$GOROOT/bin:\$PATH\""
echo "  export GOFLAGS=\"-buildvcs=false\""
echo "Then build from your project directory:"
echo "  go version && go build ./..."
