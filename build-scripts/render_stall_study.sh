#!/usr/bin/env bash
# Run on the logged-in Linux desktop, or over SSH with its DISPLAY/XAUTHORITY.
set -euo pipefail

study_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
study_output="${1:-/tmp/gothoom-render-$(date +%Y%m%d-%H%M%S)}"
study_scales="${GOTHOOM_RENDER_SCALES:-2}"
study_repeats="${GOTHOOM_RENDER_REPEATS:-3}"
study_order_offset="${GOTHOOM_RENDER_ORDER_OFFSET:-0}"
export GOTHOOM_RENDER_SECONDS="${GOTHOOM_RENDER_SECONDS:-10}"
export GOTHOOM_RENDER_FULLSCREEN="${GOTHOOM_RENDER_FULLSCREEN:-1}"
export GOTHOOM_RENDER_VSYNC="${GOTHOOM_RENDER_VSYNC:-0}"
export GOTHOOM_RENDER_DOCKED="${GOTHOOM_RENDER_DOCKED:-1}"
export EBITENGINE_GRAPHICS_LIBRARY=opengl
export GOTHOOM_PERF_IMAGES="${GOTHOOM_PERF_IMAGES:-${XDG_DATA_HOME:-$HOME/.local/share}/goThoom/CL_Images}"

for study_tool in python3 glxinfo sha256sum; do
    command -v "$study_tool" >/dev/null || { echo "Missing $study_tool" >&2; exit 1; }
done
[[ -n "${DISPLAY:-}" ]] || { echo 'Set DISPLAY to the logged-in hardware desktop; do not use Xvfb.' >&2; exit 1; }
[[ -f "$GOTHOOM_PERF_IMAGES" ]] || { echo 'Set GOTHOOM_PERF_IMAGES to the CL_Images file.' >&2; exit 1; }
[[ "$study_repeats" =~ ^[1-9][0-9]*$ ]] || { echo 'GOTHOOM_RENDER_REPEATS must be positive.' >&2; exit 1; }
[[ "$study_order_offset" =~ ^[01]$ ]] || { echo 'GOTHOOM_RENDER_ORDER_OFFSET must be 0 or 1.' >&2; exit 1; }
[[ "$GOTHOOM_RENDER_VSYNC" =~ ^[01]$ ]] || { echo 'GOTHOOM_RENDER_VSYNC must be 0 or 1.' >&2; exit 1; }
[[ "$GOTHOOM_RENDER_DOCKED" =~ ^[01]$ ]] || { echo 'GOTHOOM_RENDER_DOCKED must be 0 or 1.' >&2; exit 1; }
for study_scale in $study_scales; do
    [[ "$study_scale" =~ ^[234]$ ]] || { echo 'Use space-separated scales: 2 3 4.' >&2; exit 1; }
done
[[ ! -e "$study_output" ]] || { echo "Output already exists: $study_output" >&2; exit 1; }
mkdir -p "$study_output"
study_output="$(cd "$study_output" && pwd)"
glxinfo -B > "$study_output/gpu.txt"
if rg_pattern='llvmpipe|softpipe|Software Rasterizer'; python3 - "$study_output/gpu.txt" "$rg_pattern" <<'PY'
import pathlib, re, sys
sys.exit(0 if re.search(sys.argv[2], pathlib.Path(sys.argv[1]).read_text(), re.I) else 1)
PY
then
    echo 'Software rendering detected; use the laptop desktop GPU.' >&2
    exit 1
fi

{
    date --iso-8601=seconds
    uname -srmo
    command -v lscpu >/dev/null && lscpu
    command -v powerprofilesctl >/dev/null && powerprofilesctl get
    printf 'DISPLAY=%s\nFULLSCREEN=%s\nVSYNC=%s\nDOCKED=%s\nSCALES=%s\nSECONDS=%s\n' "$DISPLAY" "$GOTHOOM_RENDER_FULLSCREEN" "$GOTHOOM_RENDER_VSYNC" "$GOTHOOM_RENDER_DOCKED" "$study_scales" "$GOTHOOM_RENDER_SECONDS"
} > "$study_output/system.txt"
if git -C "$study_root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git -C "$study_root" rev-parse HEAD > "$study_output/commit.txt"
    git -C "$study_root" status --short > "$study_output/worktree.txt"
fi
sha256sum "$GOTHOOM_PERF_IMAGES" > "$study_output/asset.sha256"
if [[ -n "${GOTHOOM_PERF_MOVIE:-}" ]]; then
    sha256sum "$GOTHOOM_PERF_MOVIE" > "$study_output/movie.sha256"
fi

study_binary="${GOTHOOM_RENDER_BINARY:-}"
if [[ -z "$study_binary" ]]; then
    command -v go >/dev/null || { echo 'Go 1.26.6 is required to build the benchmark.' >&2; exit 1; }
    [[ "$(cd "$study_root/source" && go env GOVERSION)" == go1.26.6 ]] || { echo 'Use the project Go 1.26.6 toolchain.' >&2; exit 1; }
    study_binary="$study_output/render-study.test"
    (cd "$study_root/source" && go test -c -o "$study_binary" .)
fi
study_binary="$(realpath "$study_binary")"
sha256sum "$study_binary" > "$study_output/binary.sha256"

for study_scale in $study_scales; do
    for ((study_run=1; study_run<=study_repeats; study_run++)); do
        study_prefix="$study_output/scale-${study_scale}-run-${study_run}"
        echo "Running scale $study_scale, repeat $study_run/$study_repeats"
        # Reverse interior cases on alternate runs to expose order/heat bias.
        GOTHOOM_RENDER_REVERSE="$(( (study_run-1+study_order_offset)%2 ))" \
        GOTHOOM_RENDER_SCALE="$study_scale" GOTHOOM_RENDER_STUDY="$study_prefix.json" \
            "$study_binary" -test.run '^TestRenderStallStudy$' -test.count=1 \
            -test.timeout=30m -test.v > "$study_prefix.log" 2>&1
    done
done

python3 - "$study_output" <<'PY'
import json, pathlib, sys
root = pathlib.Path(sys.argv[1])
lines = ["| Run | Case | Frame p95 ms | Frame p99 ms | Draw p95 ms | Frames >16.67 ms |", "| --- | --- | ---: | ---: | ---: | ---: |"]
for path in sorted(root.glob("scale-*-run-*.json")):
    for case in json.loads(path.read_text())["cases"]:
        f, d = case["FrameInterval"], case["Submission"]
        lines.append(f"| {path.stem} | {case['Name']} | {f['p95_ms']:.3f} | {f['p99_ms']:.3f} | {d['p95_ms']:.3f} | {f['over_16_67ms_pct']:.2f}% |")
(root / "summary.md").write_text("\n".join(lines)+"\n")
print(f"Results: {root / 'summary.md'}")
PY
