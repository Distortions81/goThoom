# Sub-image and recolor shader checks

The temporary clips used by character-shadow composition and thought bubbles
use recyclable Ebitengine views. Each view is returned after its final draw or
clear submission. Persistent sprite views and fixed UI mask slices retain their
existing cached ownership.

The single-frame and blended mobile recolor shaders decode the packed influence
bytes with integer masks and shifts. The influence format is unchanged: three
5-bit palette slots followed by an 8-bit weight. This avoids reconstructing a
large floating-point integer and repeatedly dividing and flooring it.

## Measurements

Measured with Ebitengine commit `f3e017a2e5c2` on September 9, 2026:

- A moving thought-bubble clear/composite operation used **34 allocations with
  cached views and 28 with recyclable views**. Both paths produced identical
  pixels over 1,001 distinct clip positions/sizes. This fixture includes pixel
  readback and ran under Xvfb with Mesa llvmpipe; it is not a whole-client FPS
  measurement.
- The recolor pixel comparison passed on both llvmpipe and the Radeon RX 7900 XT
  (Mesa 26.1.3). It covers every weight byte, all pairs of center/first palette
  slots, changing secondary slots, offset source rectangles, nearest/linear
  sampling, frame blending, tint, opacity, and flash color. The allowance is one
  8-bit channel step, matching other rendering comparisons.
- Alternating 100 draws per implementation on the Radeon, including readback,
  gave mixed results (about 64–70 ms per implementation and case). There is no
  demonstrated overall shader speedup or client FPS gain from this check.

## Reproduce

Run from the repository root, with each rendering test in its own process:

```sh
CGO_ENABLED=0 GOTHOOM_RENDER_TRANSIENT_VIEWS=1 xvfb-run -a \
  go -C source test -run '^TestRenderTransientBubbleViews$' -count=1 -v .

CGO_ENABLED=0 GOTHOOM_RENDER_RECOLOR_DECODE=1 xvfb-run -a \
  go -C source test -run '^TestRenderRecolorDecode$' -count=1 -v .

CGO_ENABLED=0 GOTHOOM_RENDER_SHADOW_ORDER_TEST=1 xvfb-run -a \
  go -C source test -run '^TestRenderLayeredCharacterShadowOrder$' -count=1 -v .
```

To measure recoloring on the hardware desktop, omit `xvfb-run` and add
`GOTHOOM_BENCH_RECOLOR_SHADERS=1`. The recolor test keeps its window hidden.
Check `glxinfo -B` on the same display to identify the renderer. The timings
include submission and readback; they do not isolate GPU execution time.

The shader reference fixtures under `source/testdata/shaders/` preserve the
floating-point decoder for comparison and are embedded only in tests.

The separate sprite-slot filtered-render test fails under llvmpipe with both
the original and proposed upload-view implementations: a 48×8 view with nearest
sampling at 2.25 scale differs at byte 18688 (`want=0`, `got=30`). Sprite uploads
are unchanged; that existing failure is outside these shadow/bubble changes.
