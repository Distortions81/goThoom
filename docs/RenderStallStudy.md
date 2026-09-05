# Rendering stall study — 2026-09-05

The strongest leads are bubble geometry and bursts of UI repainting. The tested
lighting and character-shadow switches produced much smaller changes.

The implementation experiment and Ryzen 5 5500U laptop procedure are documented
in [LaptopRenderingBenchmark.md](LaptopRenderingBenchmark.md). These original
desktop timings are retained as the starting point, rather than overwritten by
later runs. The [first optimized 4x capture](benchmarks/render_stalls_4x_first_optimized.json)
reduced Draw submission p95 to 1.48 ms but left frame p95 near 6.44 ms.

## Method

- AMD Radeon RX 7900 XT, Mesa 26.1.3, accelerated OpenGL on the normal desktop
  display. Xvfb was checked and uses llvmpipe; it was not used for these timings.
- `tour-2025.08.02.clMov`, frozen at its busiest scene by picture + mobile count:
  146 pictures and 30 mobiles. The console and Players panes contain movie data;
  Chat and Inventory are also open. This is a controlled stress scene, not a
  complete movie replay or a measurement of typical gameplay.
- Production `Game.Draw`, bubble, lighting, shadow, and EUI rendering paths.
  Defaults except 50% night, shadow level 50/azimuth 135, split Chat/Console,
  manual pane placement, and disabled VSync/power saving. Replacement effects
  and light-cone shadows remain off, as in the defaults.
- 2x: 1920x1080 render surface, 1094x1080 world, UI scale 1.
  4x: 3840x2160 render surface, 2188x2160 world, UI scale 2.
  Both are presented in a 960x540 desktop window. This exercises 4K rendering
  but does not measure fullscreen presentation on a 4K monitor.
- Each case discards 90 warmup draws, then samples 600 draws. Assets and shader
  compilation are warmed; no movie parsing, Game.Update, input processing, or
  intentional texture uploads run in the measured loop. Cases run sequentially
  in one process, with a repeated baseline to expose drift.
- Frame intervals are wall time between Draw starts, including engine/driver
  submission and presentation wait. They are **not GPU timer queries**.
  Submission time measures `Game.Draw` only. A verification screenshot is read
  back after the first measured case, outside samples and before the next warmup.

## Results

Milliseconds; lower is better. Raw measurements:
[2x JSON](benchmarks/render_stalls_2x.json),
[4x JSON](benchmarks/render_stalls_4x.json).

| Case | 2x frame p95 | 4x frame p95 | 4x frame p99 | 4x Draw submission p95 |
| --- | ---: | ---: | ---: | ---: |
| Baseline | 2.10 | 6.43 | 6.47 | 2.45 |
| Lighting off | 2.16 | 6.00 | 6.05 | 2.28 |
| Faster character shadows | 2.09 | 6.53 | 6.63 | 2.49 |
| Character shadows off | 2.07 | 6.49 | 6.54 | 2.18 |
| Bubble animation off | 2.23 | 6.50 | 6.53 | 2.45 |
| All speech bubbles off | 1.43 | 5.46 | 5.50 | 0.52 |
| Four UI panes refreshed at 5 Hz | 2.15 | 12.28 | 12.74 | 2.11 |
| Four UI panes hidden | 2.11 | 6.31 | 6.36 | 2.43 |
| Baseline repeated | 2.55 | 6.54 | 6.59 | 2.44 |

The 2x repeated baseline rose from 2.10 to 2.55 ms; small differences there are
not reliable wins. The 4x baseline stayed within 6.43–6.54 ms. The UI refresh
case has a 6.26 ms median but a 12.28 ms p95: repaint bursts hurt the tail even
when the usual frame remains inexpensive. Refreshing all four panes together
at 5 Hz is an intentional stress case, not a claim that the client always does it.

## Findings in the original implementation

1. **Speech bubble geometry deserves the first implementation experiment.**
   Turning all bubbles off lowered 4x Draw submission p95 from 2.45 to 0.52 ms,
   and frame p95 from 6.43 to 5.46 ms. A separate preceding 4x CPU profile
   attributed 7.56 of 9.38 seconds sampled under `Game.Draw` to
   `drawSpeechBubbles`; 6.07 seconds ran through Ebitengine's vector fill path.
   Those cumulative numbers overlap and must not be added together.
   `drawSpikes` emits a separate antialiased path per triangle. Thought/ponder
   bodies bypass the ordinary rounded-body image cache, render to a mask,
   and composite back. Disabling animation freezes phase but still rebuilds
   geometry. Try combining spike geometry, reusing meshes, and caching static
   styled bodies. Keep changing tails and placement separate from body geometry.
2. **Avoid simultaneous full-pane repainting.** Existing caches make unchanged
   UI inexpensive: hiding the four side panes only changed 4x p95 from 6.43
   to 6.31 ms. Repainting those same panes together produced the 12.28 ms tail.
   Add per-window repaint counts, pixel area, and invalidation reasons before
   choosing between finer invalidation and spreading independent repaints.
   This experiment does not distinguish rasterization, atlas migration, and
   render-target dependency costs, so it does not justify switching to unmanaged.
3. **Lighting is a secondary steady-state target in this scene.** There are 46
   light sources and five dark sources, selecting the existing 64-light tier.
   Turning lighting off saved about 0.43 ms at 4x. Faster/disabled character
   shadows did not produce a clear gain. Other scenes with many overlapping
   shadows or enabled light-cone shadows can behave differently. Cold shader
   variants and optional replacement effects need separate first-use traces.
4. **Name tags still have allocation and eviction work to investigate.**
   `buildNameTagImage` creates unmanaged images, and `sharedNameTagImage` clears
   its shared map at 4,096 entries. Existing mobiles retain their references;
   this is not an immediate destruction of every visible tag. Revisited tags
   can nevertheless require rebuilding. A bounded managed cache with proper
   ownership would align with the sprite/bubble work; this fixed-scene test
   does not measure its potential benefit.

There is no ordinary texture readback in the inspected warmed draw path.
`readImageRGBA` has no callers, and `nonTransparentPixels` uses decoded CPU
artwork when CL_Images is present. `ebiten.ReadDebugInfo` sums atlas backend
sizes under a mutex rather than reading GPU pixels. Ebitengine 2.9.10 already
backs off source-atlas promotion exponentially for repeatedly modified render
targets; managed UI textures are not blindly repacked on every repaint.

Relevant code: `source/bubble.go`, `source/eui/render.go`,
`source/lighting_shader.go`, `source/mobile_shadows.go`,
`source/name_tag_cache.go`, and `source/stats_ui.go`.

## Reproduce

Use a hardware-backed display and run this opt-in test alone. The normal Go
suite skips it. It deliberately changes process-wide renderer globals and
opens a temporary window. The fixture uses the standard bundled movie.
`GOTHOOM_PERF_IMAGES` optionally overrides `source/data/CL_Images` for this and
the existing tour benchmarks.

```sh
cd source
GOTHOOM_PERF_IMAGES=/path/to/CL_Images \
GOTHOOM_RENDER_SCALE=4 \
GOTHOOM_RENDER_STUDY=/tmp/render-4x.json \
go test . -run '^TestRenderStallStudy$' -count=1 -v
```

Repeat with scale 2. The JSON also records actual world bounds/scale, and the
baseline screenshot is written beside it with `.png` appended. For CPU
profiling, add `-cpuprofile=/tmp/render.cpu -o /tmp/render-study.test` and inspect
with `go tool pprof`. Profiled timings should be kept separate from the final
unprofiled comparisons above.
