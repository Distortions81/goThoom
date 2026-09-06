# PGO hotspot follow-up, September 2026

Three changes address the test51 CPU profile:

- Short UI lines use a single cached mask quad, including title-bar grip
  marks. Longer lines keep their shared stretched masks. Whole-mask draws
  also avoid an unnecessary SubImage lookup.
- Plain text items reuse rendered text across parent-window repaints. The key
  includes content, font source (including bold/italic), size, layout,
  fractional position, and color. Separately constructed Chat/Console faces
  share entries. The cache is limited to 256 entries and 8 MiB of image pixels;
  these are cache payload limits, not total atlas/driver memory. Text above
  4096 bytes or 1024x256 pixels uses the direct path. Selection, cursor,
  underlines, and background are still painted separately.
- Layered character and artwork shadows check individual prior caster bounds
  after a cheap union rejection. Empty gaps inside the union can now use the
  two-submission path. Receiver alpha clears retain conservative bounds, and
  genuine overlaps retain the existing maximum-opacity composition and painter
  order. The rectangle slice is reused between frames.

## Validation and focused measurements

The opt-in GPU tests compare text against direct rendering, primitives against
vector rendering, and sparse shadow coverage against the former union path.
Text checks include clipping, fractional positions, translucent colors,
regular/bold/italic fonts, font sizes, changed content, equivalent face objects,
and both cache limits. Shadow checks include foreground receivers and mixed
character/artwork casters. Pixel tolerance is two channel values for text, one
for shadow comparison, and the existing vector AA tolerance for primitives.

Focused timings include a readback after each group and warm both paths. They
measure deliberately repetitive workloads, not typical gameplay FPS. Hardware:
AMD Radeon RX 7900 XT, accelerated OpenGL, Mesa 26.1.3, Linux/amd64, Go 1.26.6.

| Focused workload | Previous path | Optimized path | Reduction |
| --- | ---: | ---: | ---: |
| 3,000 short title grips | 29.57 ms | 8.15 ms | 72% |
| 1,800 repeated text draws | 24.55 ms | 5.99 ms | 76% |
| 3,000 separated character shadows | 47.80 ms | 27.44 ms | 43% |

These single-process comparisons run the previous path followed by the
optimized path after warmup. Shadow timing deliberately stresses gaps inside
prior coverage bounds; densely overlapping shadows still need the full path.

## Full-scene comparison

The corrected existing render-stall fixture used a 1920x1080 surface, artwork
scale 2, UI scale 1, a 960x540 presentation window, VSync off, and a frozen tour
scene. Each case discarded 90 warmup frames and sampled at least 600 frames
and ten seconds. Cases were baseline, four panes refreshed at 5 Hz without
repaint deferral, and baseline repeated. Neither ordinary Game.Update nor
moving-movie asset loading is measured.

The final pair ran **after, then before** in separate processes with no other
builds/tests running. Before is commit `fc02ebd`; both builds use its existing
PGO profile. Earlier runs interrupted by other desktop activity were discarded.

| Case | Before Draw p95 | After Draw p95 | Before frame p95 | After frame p95 |
| --- | ---: | ---: | ---: | ---: |
| Baseline | 0.660 ms | 0.613 ms | 0.820 ms | 0.765 ms |
| Four panes refreshed at 5 Hz | 0.630 ms | 0.682 ms | 0.788 ms | 0.843 ms |
| Baseline repeated | 0.646 ms | 0.658 ms | 0.804 ms | 0.814 ms |

These results do **not** show a consistent whole-frame improvement. Repaints
are infrequent relative to the uncapped frame rate, and gains in these targeted
paths do not establish a general FPS gain. Initial text rendering also pays the
cost of populating its new cache. The PGO capture used a different GPU (Renoir)
and a mixed movie/Settings workload, so its hotspot percentages cannot be
converted into a speedup on this machine.

Raw reports: [before](benchmarks/pgo_hotspots_before.json),
[after](benchmarks/pgo_hotspots_after.json).

Validation passed: `go test ./...` under Xvfb, a normal client `go build`,
all three opt-in GPU checks, and `git diff --check`. The rendered fixture
was also visually inspected. The compiler profile itself was not replaced.

## Reproduce

From `source/`, run the GPU checks individually (one Ebitengine game loop per
process):

```sh
GOTHOOM_RENDER_PRIMITIVE_CACHE=1 go test ./eui -run '^TestRenderPrimitiveCache$' -count=1 -v
GOTHOOM_RENDER_TEXT_CACHE=1 go test ./eui -run '^TestRenderUITextCache$' -count=1 -v
GOTHOOM_RENDER_SHADOW_ORDER_TEST=1 go test . -run '^TestRenderLayeredCharacterShadowOrder$' -count=1 -v
```

Build each render-study variant with `go test -c -o /tmp/render.test .`, then
run on the same hardware display with:

```sh
GOTHOOM_PERF_IMAGES=/path/to/CL_Images \
GOTHOOM_RENDER_SCALE=2 GOTHOOM_RENDER_ARTWORK_SCALE=2 \
GOTHOOM_RENDER_SAMPLES=600 GOTHOOM_RENDER_SECONDS=10 \
GOTHOOM_RENDER_CASES=baseline,ui_refresh_5hz_unbatched,baseline_repeat \
GOTHOOM_RENDER_STUDY=/tmp/render.json \
/tmp/render.test -test.run '^TestRenderStallStudy$' -test.v
```
