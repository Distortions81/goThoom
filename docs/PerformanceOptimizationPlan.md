# Performance Optimization Plan

## Goals

- Reduce CPU use during busy scenes without changing visual output unless a
  lower-cost behavior is explicitly part of the iGPU preset.
- Reduce GPU render passes, target switches, draw work, shader cost, and fill
  rate without reducing normal rendering quality.
- Improve sustained FPS and frame pacing rather than optimizing only average
  frame rate.
- Keep the implementation simple and verify each optimization independently.

## Current baseline

The current reference workload is a five-minute playback of:

`source/clmovFiles/tour-2025.08.02.clMov.zip`

The current representative profile averages about 17% of one CPU core after
assets are loaded. The largest actionable costs are:

| Area | CPU time over five minutes |
| --- | ---: |
| Draw-state parsing | 7.95s |
| Sound enhancement and reverb | 4.18s |
| Scene drawing | 3.57s |
| Speech bubbles | 3.10s |
| Obscuring detection | 2.89s |
| EUI rendering | 2.50s |
| Chat wrapping and updates | 1.29s |

CPU profiles do not directly measure GPU stalls, fill rate, or the number of
real render passes. GPU work must be instrumented separately.

### Repeatable CPU benchmarks

`source/performance_benchmark_test.go` replays the real tour recording with the
actual `CL_Images` archive and default game settings. Fixture loading and the
initial replay are completed before benchmark timing starts. The suite measures
the complete draw-state stream, a 146-picture/30-mobile busy scene, the most
expensive 105-picture/49-mobile obscuring scene, and real bubble text from the
recording. Synthetic scene data is not used.

Run the complete tour-backed benchmark once with:

```bash
cd source
go test -run '^$' \
  -bench 'Benchmark(TourDrawState|PrepareBusySceneRenderCache|CaptureBusySceneSnapshot|BusyScenePictureObscuring|TourBubbleTextLayout)$' \
  -benchmem -benchtime=1x -count=1
```

Run the shorter benchmarks long enough to get stable comparison samples with:

```bash
cd source
go test -run '^$' \
  -bench 'Benchmark(ApplyGameSoundReverb|BuildMicroAmbience|PrepareBusySceneRenderCache|CaptureBusySceneSnapshot|BusyScenePictureObscuring|TourBubbleTextLayout)$' \
  -benchmem -benchtime=2s -count=3
```

Baseline on an AMD Ryzen 5 5500U with Radeon Graphics:

| Benchmark | Current result | Allocations |
| --- | ---: | ---: |
| Tour state update, 31,908 packets | 767ms total | 73.3MB, 498,505 allocs |
| Tour state update plus render caches | 942ms total | 80.3MB, 535,041 allocs |
| Busy-scene render-cache preparation | 16.0-16.9us | 5,376B, 13 allocs/op |
| Busy-scene snapshot capture | 7.63-7.68us | 0B, 0 allocs/op |
| Busy-scene picture obscuring | 174.8-182.0us | 1.5-1.7KB, 182-202 allocs/op |
| Tour bubble layout, uncached | 17.9-18.4us | 1,128-1,130B, 23 allocs/op |
| Tour bubble layout, cached | 31.9-32.4ns | 0B, 0 allocs/op |
| One second of game-sound reverb | 1.41-1.42ms | 599,712B, 12 allocs/op |
| One second of micro ambience | 350-351us | 198,656B, 4 allocs/op |

These CPU benchmarks deliberately do not claim to measure GPU time. A separate
in-game counter is still needed for render-target changes, full-screen passes,
and draw submissions because queued Ebitengine calls alone do not reveal GPU
execution cost.

## Phase 1: Performance instrumentation

Add a repeatable performance report that records:

- Per-function CPU time and call paths from the standard PGO/pprof capture;
  avoid manual wall-clock instrumentation around individual functions.
- Average FPS and p50, p95, and p99 frame times.
- Allocation counts and steady-state texture-atlas growth.
- Visible picture, mobile, bubble, light, dark, and shadow counts.
- Ebitengine draw submissions where measurable.
- GPU command flushes, render-target switches, full-screen passes, and shader
  changes where measurable.
- Whether assets were fully loaded before measurement began.

Maintain standard five-minute tour captures for:

1. Current saved settings.
2. Full Quality.
3. iGPU Graphics.

The same movie section, window size, and settings must be used for comparisons.

## Phase 2: Remaining CPU work

### Audio enhancement

- Completed: move the working signal and filters to `float32`, use SIMD for
  saturation, and combine the two ambience results in place. This reduced the
  one-second reverb benchmark by about 39% and temporary memory by about 62%.
  Compared with the former float64 path, the test waveform differs by at most
  one integer sample step with about 128dB SNR.
- Completed: cache final rendered sound combinations and retain up to 64
  reusable Ebitengine players. Repeated sounds now rewind an idle player and
  reuse immutable PCM; overlapping copies create additional players up to the
  existing 64-sound limit. Rendered PCM is retained without a memory budget.
- Reuse the remaining scratch buffers instead of allocating them for each
  sound.
- Combine the two micro-ambience passes when practical.
- Cache delay layouts, filter coefficients, and other values derived only from
  sample rate and enhancement strength.
- Consider `float32` processing where listening tests show no quality loss.
- Preserve current output closely enough for waveform and listening tests.

The iGPU preset already disables audio enhancement and high-quality resampling.

### Picture obscuring

- Keep overlap detection at actual game-update frequency, never render FPS.
- Completed: index mobile bounding boxes in 128x128 world blocks and check only
  opaque positive-plane pictures in overlapping blocks. Bounding boxes and draw
  order reject candidates before the existing quarter-resolution alpha-mask
  test. The 105-picture/49-mobile benchmark fell from 214.9us to 30.7us while
  retaining pixel-mask accuracy. Pooled block-map scratch storage and typed
  mask-cache keys subsequently reduced the warmed path to 21.1us with zero
  allocations.
- Keep render-time work limited to interpolation of the cached previous and
  current opacity states.

### State and snapshot processing

- Completed: reset picture-shift indexes only for IDs used by the previous
  update instead of every historical picture ID. A 60-second tour benchmark
  improved from 918.7ms to 549.9ms per 31,908-packet pass, a 40.1% reduction.
- Completed: pool picture-position matching slices, replace reflection-backed
  render sorts with generic sorts, retain the render-cache picture sort buffer,
  and reuse picture-shift result storage. Together with allocation-free
  obscuring, the warmed tour now runs at 298.3ms with 37.8MB and 217,712
  allocations per pass.
- Completed: rotate the current/previous picture slices and mobile maps instead
  of allocating and copying them on every update, and retain cleared
  interpolation maps across discontinuities. The warmed tour now runs at
  287.6ms with 12.7MB and 194,358 allocations per pass.
- Investigate publishing immutable render snapshots once per game update
  instead of copying stable maps and slices every rendered frame.
- Reuse bounded buffers for parsing and sorted render partitions.
- Avoid recalculating stable sprite metadata in both update and draw paths.

### Chat and text

- Completed: cache rendered name-tag images globally by every visual input,
  including friend-label frame color, and reuse them across mobile indexes and
  disappear/reappear cycles. The 31,908-packet tour improved from 378.0ms to
  309.8ms per pass; allocations fell from 100.2MB/661,867 objects to
  67.6MB/450,949 objects. Modern health bars are composed separately so health
  color changes reuse the same cached text surface.
- Incrementally wrap newly appended chat text rather than remeasuring unchanged
  content.
- Cache text layouts using bounded caches that are cleared when font or scale
  settings change.

## Phase 3: Stable surface and geometry caching

### Speech bubbles

- Cache the bubble body, border, and text as a tight local surface.
- Never include the connecting triangle or ponder-tail circles in the cached
  surface. Draw the connector dynamically so it continues to follow the player
  and remains correct when the bubble body is clamped at a screen edge.
- For animated bubbles, redraw only the moving ponder, yell, or monster
  decoration around the cached body.
- Use tight local images instead of screen-sized intermediate masks.
- Invalidate caches on text, type, scale, font, theme, opacity, or tail-layout
  changes.
- Keep caches bounded to prevent long sessions from growing GPU memory.

### Names and UI

- Retain name-tag images across game updates until their cache key changes.
- Keep configuration windows cached and invalidate them only when displayed
  state changes.
- Avoid rebuilding invisible, closed, or fully off-screen windows.
- Track steady-state atlas allocation; the target is effectively zero after
  warmup.

## Phase 4: Reduce GPU render passes

Render-pass reduction should focus on full-screen work and render-target
switches. A lower number of Go `DrawImage` calls does not necessarily mean
fewer GPU passes because Ebitengine already batches compatible submissions.

### Lighting and final world composition

The current shader-lighting path copies the world into a lighting texture,
runs the lighting shader back into the world target, and then scales the world
into the game image.

Investigate drawing the world texture through the lighting shader directly into
the game image at final scale. If coordinate and filtering behavior can be made
pixel-equivalent, this can replace:

1. The world-to-lighting temporary copy.
2. The lighting write-back pass.
3. The ordinary world-to-game composite.

with one final lighting-and-scale composite pass.

Retain the existing path when shader lighting is disabled or when exact output
cannot be preserved.

### Character shadows

- When shader lighting and Accurate Character Shadows are both enabled,
  investigate sampling the accumulated character-shadow mask in the fused final
  lighting pass instead of compositing it separately.
- Keep direct inexpensive shadows for modes that do not require overlap-safe
  accumulation.
- Do not change shadow appearance merely to remove a pass in Full Quality.

### Thought and ponder bubbles

- Replace the screen-sized mask clear, fill, and composite sequence with a
  tight local cached mask.
- Composite each static bubble once per frame rather than rebuilding and
  compositing its individual parts.

### Clears and intermediate targets

- Identify surfaces that are cleared immediately before being completely
  overwritten.
- Replace full-target clears with batched letterbox or uncovered-region fills
  when that performs better.
- Reuse grow-only intermediate targets to avoid allocation and target churn.
- Skip lighting and shadow intermediates entirely when their active counts are
  zero.

## Phase 5: Culling and batching

### Culling

Before loading textures or submitting draws, reject fully invisible:

- Pictures and mobiles.
- Name tags and bubbles.
- Character and contact shadows.
- Lights, dark sources, and light/caster interactions.
- Script overlays and UI elements.

Account for interpolation distance, glow radius, projected shadow reach, and
bubble tails so culling does not create edge popping.

### Batching

- Batch adjacent compatible sprites only when painter order is unchanged.
- Batch health bars, status bars, solid UI rectangles, and similar primitives.
- Reuse vertex and index buffers.
- Measure actual GPU flushes before and after each batching change.
- Do not reorder zero-plane scenery and mobiles merely to improve batching.

### Stable scene layers

Investigate caching stable negative/background and positive/foreground picture
layers between actual game updates. Zero-plane scenery must retain its ordering
with mobiles and may need to remain dynamic.

Layer caching should proceed only if target-switch and fill-rate measurements
show a net benefit after accounting for the extra intermediate textures.

## Phase 6: Shader and fill-rate optimization

- Select only visible and relevant lights, dark sources, and shadow
  interactions before uploading uniforms.
- Test lighting shader variants sized for small light-count buckets such as
  4, 8, 16, and 32 rather than paying for maximum capacity in every scene.
- Skip unused shader branches and full-screen effects where active counts are
  zero.
- Test a lower-resolution lighting buffer while leaving artwork at its selected
  2x-or-higher scale. Adopt it only where comparisons show no objectionable
  quality loss.
- Measure whether window shadows and UI opacity layers cause additional
  full-screen or target-switch costs.
- Preserve the 2x minimum artwork scale in the wizard.

## Phase 7: Preset policy

The iGPU preset should retain its current measured tradeoffs:

- 2x Balanced artwork upscaling.
- Shader lighting off.
- Character shadows off.
- Window shadows off.
- Character and world animation blending off.
- Animated chat bubbles off.
- Audio enhancement off.
- High-quality audio resampling off.
- Potato GPU remains an independent, off-by-default unmanaged-texture option.

Fade Obscuring Pictures should remain available in iGPU mode because its costly
overlap detection now runs only on actual game updates.

Do not add more iGPU quality reductions until profiling shows a meaningful gain
and the visual tradeoff is understood.

## Validation gates

Every optimization group must pass:

- `go test ./...`
- A five-minute tour profile using the same configuration as its baseline.
- Average FPS and p95/p99 frame-time comparison.
- CPU, allocation, GPU-pass, and target-switch comparison.
- Screenshot or render-test comparison for appearance-neutral changes.
- Manual inspection of motion, shadows, bubbles, lighting, name tags, and scene
  edges.
- Long-session checks for cache growth and stale GPU resources.

General optimizations should be pixel-equivalent where practical. Intentional
iGPU differences must be documented and tested separately.

## Recommended implementation order

1. Use PGO/pprof for CPU attribution, and add only frame-time, draw, GPU-pass,
   and target-switch instrumentation that the CPU profile cannot provide.
2. Reuse audio enhancement scratch buffers and cached coefficients.
3. Completed: add spatial indexing and metadata caching to picture obscuring.
4. Cache static bubble surfaces and tight bubble masks.
5. Prototype the fused lighting-and-final-composite pass.
6. Add conservative off-screen culling.
7. Batch solid rectangles and adjacent compatible draws.
8. Evaluate stable scene layers and lighting shader variants.
9. Reprofile all three standard settings profiles and tune presets only from
   measured results.

## Success criteria

- At least another 20-30% reduction in client CPU use during the tour scene.
- Meaningfully lower GPU pass and target-switch counts in shader-lighted scenes.
- No steady-state texture-atlas growth after warmup.
- Improved p95 and p99 frame times, not only average FPS.
- No visible regression in normal rendering from appearance-neutral changes.
- Documented and deliberate differences for iGPU mode.
