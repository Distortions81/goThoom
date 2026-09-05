#!/usr/bin/env bash
# Linux/GLX diagnostic build only. Never changes the checkout or Go module cache.
set -euo pipefail

probe_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
probe_output="${1:?Usage: build_gpu_frame_probe.sh /absolute/new-output-directory}"
[[ "$probe_output" = /* && ! -e "$probe_output" ]] || {
    echo 'Use an absolute output directory that does not exist.' >&2
    exit 1
}
cd "$probe_root/source"
[[ "$(go env GOOS)" = linux ]] || { echo 'This probe requires Linux/GLX.' >&2; exit 1; }
[[ "$(go list -m -f '{{.Version}}' github.com/hajimehoshi/ebiten/v2)" = v2.9.10 ]] || {
    echo 'The diagnostic patch requires Ebitengine v2.9.10.' >&2
    exit 1
}
probe_module="$(go list -m -f '{{.Dir}}' github.com/hajimehoshi/ebiten/v2)"
mkdir -p "$probe_output"
cp -R "$probe_module" "$probe_output/ebiten"
chmod -R u+w "$probe_output/ebiten"
git -C "$probe_output/ebiten" apply "$probe_root/build-scripts/diagnostics/ebitengine-2.9.10-gpu-timing.patch"
cp go.mod "$probe_output/probe.mod"
cp go.sum "$probe_output/probe.sum"
go mod edit -modfile="$probe_output/probe.mod" \
    -replace="github.com/hajimehoshi/ebiten/v2=$probe_output/ebiten" \
    -replace="gt2=$probe_root/source/gt2"
go test -modfile="$probe_output/probe.mod" -c -o "$probe_output/render.test" .
echo "Built $probe_output/render.test; set GOTHOOM_GPU_TIMELINE to a CSV output path."
