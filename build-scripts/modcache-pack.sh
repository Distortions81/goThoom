#!/usr/bin/env bash
set -euo pipefail

# Freeze deps + the CURRENT Go toolchain (OS/arch specific) for offline builds.
# Usage: modcache-pack.sh [project-dir=. ] [outdir=. ]
# Output: <outdir>/offline-go-<name>-<yyyymmdd>.tar.(zst|xz|gz) (+ .sha256)

PROJ="${1:-.}"
OUTDIR="${2:-.}"

cd "$PROJ"
[[ -f go.mod ]] || { echo "No go.mod in $PROJ"; exit 1; }
command -v go >/dev/null || { echo "Go toolchain not found in PATH"; exit 1; }

export GOWORK=off
export GOFLAGS="${GOFLAGS:-} -buildvcs=false"

NAME="$(basename "$(pwd)")"
DATE="$(date +%Y%m%d)"
GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"
GOVERSION="$(go env GOVERSION || true)"
[[ -n "${GOVERSION:-}" ]] || GOVERSION="$(go version | awk '{print $3}')"
GOROOT="$(go env GOROOT)"

# Compressor
if command -v zstd >/dev/null 2>&1; then COMP="zstd -q -T0 --ultra -22"; EXT="zst";
elif command -v xz   >/dev/null 2>&1; then COMP="xz -T0 -9e";              EXT="xz";
else                                           COMP="gzip -9n";            EXT="gz"; fi

OUT="${OUTDIR}/offline-go-${NAME}-${DATE}.tar.${EXT}"

# 1) Ensure deps are in local module cache
go mod download -x all

# 2) Enumerate modules (Path@Version + Dir)
TMP_JSON="$(mktemp)"
go list -m -json -e all >"$TMP_JSON"

# 3) Stage just the needed modules into modcache/
GOMODCACHE="$(go env GOMODCACHE)"
STAGE="$(mktemp -d)"
CACHEDIR="${STAGE}/modcache"
mkdir -p "$CACHEDIR"

awk '
/^{/ {path=""; ver=""; dir=""}
/"Path":/    {sub(/.*"Path": *"/,""); sub(/".*/,""); path=$0}
/"Version":/ {sub(/.*"Version": *"/,""); sub(/".*/,""); ver=$0}
/"Dir":/     {sub(/.*"Dir": *"/,""); sub(/".*/,""); dir=$0}
$0 ~ /^}/ && path != "" {
  key = (ver=="" ? path : path "@" ver)
  print key "\t" dir
}
' "$TMP_JSON" | while IFS=$'\t' read -r KEY DIR; do
  [[ "$KEY" == *@* ]] || continue       # skip main module
  [[ -n "${DIR:-}"   ]] || continue
  SRC="${GOMODCACHE}/${KEY}"
  [[ -d "$SRC" ]] || continue
  DEST="${CACHEDIR}/${KEY}"
  mkdir -p "$(dirname "$DEST")"
  rsync -a --delete --exclude='.git' "$SRC/" "$DEST/"
done
rm -f "$TMP_JSON"

# 4) Stage the CURRENT toolchain under toolchain/go/
#    We copy the existing GOROOT. This guarantees no download and exact match.
TOOLDIR="${STAGE}/toolchain/go"
mkdir -p "$(dirname "$TOOLDIR")"
# Keep it simple: copy everything. It’s big but bulletproof.
rsync -a --delete --exclude='.git' "$GOROOT/" "$TOOLDIR/"

# 5) Add metadata + helper env script
cat > "${STAGE}/README-OFFLINE.txt" <<EOF
Offline Go build pack
---------------------
Project: ${NAME}
Date:    ${DATE}

Toolchain:
  OS/Arch: ${GOOS}/${GOARCH}
  Go:      ${GOVERSION}
  Source:  ${GOROOT}

Contents:
  modcache/         # frozen modules used by 'go list -m all'
  toolchain/go/     # exact Go toolchain (GOROOT)

How to use (on target, offline):
  # unpack somewhere accessible and then:
  export GOPROXY=off
  export GOSUMDB=off
  export GOMODCACHE="\$PWD/modcache"
  export GOROOT="\$PWD/toolchain/go"
  export PATH="\$GOROOT/bin:\$PATH"
  # optional: faster, cleaner builds in detached trees
  export GOFLAGS="-buildvcs=false"

  # from your project directory:
  go version        # should print ${GOVERSION}
  go build ./...

Notes:
- This toolchain is for ${GOOS}/${GOARCH}. If you need another platform,
  run the pack script on that platform as well.
- Local 'replace ../foo' paths are not modules; they’re NOT in modcache.
  Convert them to proper versioned modules or vendor your local libs.
EOF

cat > "${STAGE}/env.sh" <<'EOSH'
#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export GOPROXY=off
export GOSUMDB=off
export GOMODCACHE="${ROOT}/modcache"
export GOROOT="${ROOT}/toolchain/go"
export PATH="${GOROOT}/bin:${PATH}"
export GOFLAGS="-buildvcs=false"
echo "Env set for offline Go builds."
go version
EOSH
chmod +x "${STAGE}/env.sh"

# 6) Pack deterministically
set -x
tar \
  --sort=name \
  --owner=0 --group=0 --numeric-owner \
  --mtime='UTC 2020-01-01' \
  -C "$STAGE" \
  -cf - \
  modcache toolchain README-OFFLINE.txt env.sh \
| eval "$COMP" > "$OUT"
set +x

# Optional checksum
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum "$OUT" > "${OUT}.sha256"
elif command -v shasum >/dev/null 2>&1; then
  shasum -a 256 "$OUT" > "${OUT}.sha256"
fi

echo "Wrote: $OUT"
[[ -f "${OUT}.sha256" ]] && echo "Checksum: ${OUT}.sha256"
