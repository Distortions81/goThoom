# Pixel-aligned UI fills — 2026-09-05

UI rectangle fills already round their position and dimensions to whole pixels.
Disable antialiasing for these fills: Ebitengine 2.9.10's `vector.FillRect`
then uses a batchable image quad instead of a vector stencil and fill pass.
Rounded corners, outlines, text, and bubble rendering retain their existing paths.
No graphics preferences or repaint scheduling defaults change.

## Measured result

Ryzen 5 5500U, accelerated Radeon OpenGL (radeonsi/renoir), Mesa 25.2.8,
official Go 1.26.6. Existing frozen tour scene: 146 pictures, 30 mobiles,
50% night, animated bubbles, four UI panes. Render surface 1920x1080,
artwork scale 2, UI scale 1, presented in a 960x540 window, VSync off.
Each case discards 90 warmup frames and samples at least 600 frames and ten
seconds. The study excludes `Game.Update`, asset loading, and network traffic.

| Run | Case | Before frame p95 | After frame p95 | Before frame p99 | After frame p99 |
| --- | --- | ---: | ---: | ---: | ---: |
| First pair | Baseline | 39.16 ms | 38.41 ms | 39.33 ms | 38.99 ms |
| First pair | Four panes refreshed at 5 Hz | 75.15 ms | 60.98 ms | 76.22 ms | 61.29 ms |
| First pair | Baseline repeated | 39.09 ms | 38.27 ms | 39.41 ms | 38.89 ms |
| Reverse-order confirmation | Four panes refreshed at 5 Hz | 74.98 ms | 60.99 ms | 75.93 ms | 61.20 ms |

Repaint-burst frame p95 improved by approximately 19%, or 14 ms, in both
comparisons. Baseline changes were small. All sampled frames still exceeded
33.33 ms on this machine: this is a reduction in repaint stalls, not a claim
of smooth 60 FPS. These wall-clock intervals include engine/driver work and
presentation; they are not GPU timestamps or measurements of typical gameplay.

The first pair ran before then after, with baseline/repaint/baseline cases.
Confirmation ran the repaint case alone in fresh processes, after then before.
Builds and other tests were paused during these four accepted runs. Earlier
interrupted/unprepared measurements were discarded.

Raw reports: [before](benchmarks/ui_rect_before_rerun.json),
[after](benchmarks/ui_rect_after_rerun.json),
[confirmation before](benchmarks/ui_rect_before_confirm.json),
[confirmation after](benchmarks/ui_rect_after_confirm.json).

The original binary was built from `9c814216e1a54ac4f50d35e77deafaeed743daf7`.
The modified binary differs only in `source/eui/util.go`'s rectangle fill helper.
SHA-256:

```text
before:    47fdfc1575898108667b3cc8f6762a5bd603ec57338600cb178757a7404eee5d
after:     32950eafea40cf92170b5d9d107c59c10f44abd847c1184c3ab67c67df451291
CL_Images: 0f4c61e2b4d7e983d921ab6b4e5b37b61758604ccd56bb20cce606ba47ad0297
```

## Reproduce and verify

Build each variant with `cd source && go test -c -o /tmp/render.test .`.
Run on the same hardware display and presentation settings:

```sh
GOTHOOM_PERF_IMAGES=/path/to/CL_Images \
GOTHOOM_RENDER_SCALE=2 GOTHOOM_RENDER_ARTWORK_SCALE=2 \
GOTHOOM_RENDER_SAMPLES=600 GOTHOOM_RENDER_SECONDS=10 \
GOTHOOM_RENDER_CASES=baseline,ui_refresh_5hz_unbatched,baseline_repeat \
GOTHOOM_RENDER_STUDY=/tmp/render.json \
/tmp/render.test -test.run '^TestRenderStallStudy$' -test.v
```

For confirmation, set `GOTHOOM_RENDER_CASES=ui_refresh_5hz_unbatched` and use
separate report paths. The study now fails if its window closes before every
requested case completes, instead of reporting success with partial results.

The before/after screenshots matched exactly in the Chat, Inventory, and
Players panes. The independent GPU pixel test compares old and new fill paths
on transparent/opaque backgrounds, translucent fills, rounded fractional
coordinates, clipped subimages, and zero/negative sizes (one-channel-value
tolerance for shader rounding):

```sh
cd source
GOTHOOM_RENDER_RECT_TEST=1 go test ./eui -run '^TestRenderPixelAlignedRect$' -count=1
```
