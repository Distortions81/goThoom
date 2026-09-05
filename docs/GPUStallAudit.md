# GPU stalls and frame-tail root causes

Investigation: 2026-09-05, reference commit `57bca77`, Ryzen 5 5500U / Radeon
renoir, Mesa 25.2.8, Linux X11/KDE, 1920x1080 at 60.05 Hz. Measurements and
transition log excerpts are retained in
[the evidence JSON](benchmarks/gpu_root_causes.json).

## Findings

Normal-speed walking with VSync on did not reproduce the regular scrolling
drops in the final capture. The user also confirmed smoother play. We have not
isolated which earlier change improved it, so this cleanup adds no further
production rendering optimization.

The earlier 38 ms baseline was dominated by a benchmark fixture error.
Floating-window decoration repaints and uncached artwork preparation add
measurable work in other captures, but neither was established as the cause
of the reported regular walking drops. Startup and occasional scene-transition
hitches are outside this task's steady-play scope.

### Normal-speed walking verification

The production movie player replayed a sustained walking section of the tour
at its normal 5 updates per second, with VSync on, tiled windows and power
saving off. A temporary diagnostic overlay sought to frame 12290 and buffered
frame timings. Measurement started at logical frame 12340, excluding startup
and the first 50 updates after seeking, and ended at frame 12434. The window
background-cache experiment was disabled.

| Measured frames | Samples | Frame interval p95 | Frame interval p99 | Intervals over 25 ms |
| --- | ---: | ---: | ---: | ---: |
| All | 1,140 | 17.254 ms | 17.501 ms | 0 |
| Camera moving | 936 | 17.247 ms | 17.469 ms | 0 |
| UI repaint | 114 | 17.247 ms | 17.501 ms | 0 |

The complete measurement spans approximately 19 seconds. These are
Update-to-Update intervals from one instrumented run, not a paired speedup
comparison or direct display-scanout measurements. The 25 ms threshold helps
separate normal 60 Hz scheduling variation from intervals consistent with a
missed refresh. CPU Draw p95 during moving frames was 1.925 ms. The final
sample summary is retained under `native_walking_vsync` in the evidence JSON.

The extra background cache, asynchronous icon-loading experiments, repaint
budget changes and line/border experiments are not included in the final
change. The retained client constructor refactor preserves its existing
rendering flags and makes the benchmark use the same policy.

### 1. The benchmark rendered a background absent from the client

The study created a default EUI game window with `NoCache`, but omitted the
client's `NoBGColor` policy. This drew a large rounded background and drop
shadow every frame underneath the opaque playfield. It also selected tiled
layout without applying the production docking policy, leaving pane shadows
and rounded backgrounds enabled.

The fixture now shares `newGameRenderWindow` with the client and applies the
production pane chrome setup. Production game-window behavior is unchanged.
`GOTHOOM_RENDER_DOCKED` and `GOTHOOM_RENDER_VSYNC` are explicit controls and
appear in each result. Historical reports lacking the corrected fixture
fields must not be used as measurements of normal client frame times.

Clean fixed-scene runs, windowed, scale/artwork 2x, at least 600 samples and
ten seconds per case:

| Fixture / layout | VSync | Baseline frame p95 | Repaint frame p95 | Repaint frame p99 |
| --- | --- | ---: | ---: | ---: |
| Old fixture, artificial game background | Off | 38.25 ms | 61.03 ms | 61.34 ms |
| Only game background corrected | Off | 8.74 ms | 16.48 ms | 17.03 ms |
| Corrected tiled panes | Off | 6.46 ms | 6.69 ms | 11.54 ms |
| Corrected tiled panes | On | 17.25 ms | 17.18 ms | 17.38 ms |
| Corrected floating panes | Off | 8.73 ms | 16.50 ms | 17.01 ms |

A final clean run through the updated runner confirmed tiled VSync-off p95
of 6.56 ms baseline and 6.82 ms repaint (p99 7.42 and 11.65 ms), with no
intervals over 16.67 ms in either case.

The 38-to-6 ms change is a benchmark correction, **not a client speedup**.
The prior pixel-aligned rectangle improvement remains a result from the old
stress fixture, not a demonstrated 19% improvement in gameplay.

Asynchronous GPU timestamps independently localized the artificial workload.
Across warmed submission batches 100–190, removing only the game background
changed median GPU interval from 36.39 to 7.32 ms. Vector scratch-target time
fell from 22.13 to 2.67 ms and UI-target time from 10.07 to 0.45 ms; scene and
lighting/composite targets stayed around 1.53 and 2.44 ms. Driver submission
was approximately 1.89 versus 1.76 ms, while swap time fell from 35.77 to
6.57 ms. The long swap was therefore associated with queued rendering work,
not evidence that VSync alone caused the old 38 ms baseline.

With the corrected tiled fixture, VSync on produces the expected approximately
60 Hz cadence. The above VSync-on cases had no intervals over 33.33 ms.
A lower uncapped frame interval is not itself a recommendation to disable
VSync. Fullscreen/compositor behavior on other configurations remains untested.

### 2. Floating-window background and shadow repaints add real GPU work

`windowData.drawBG` in [EUI rendering](../source/eui/render.go) renders shadows
and rounded backgrounds for floating windows whenever their cached content
is repainted. Docked windows skip the shadow and use rectangular backgrounds.
Frequently updated floating panels, including Movie Controls, exercise this
path even when the main workspace is tiled.

Two independent diagnostics support this attribution:

- A temporary fixed-scene background-cache prototype reduced floating-pane
  repaint frame p95 from 16.50 to 9.79 ms. Chat, Inventory, and Players image
  regions were pixel-identical. Frame p99 remained 16.86 ms versus 17.01 ms,
  so this was not a complete tail fix. The prototype is not shipped: it did
  not implement general resize, theme, scale, or appearance invalidation.
- Actual client movie playback with only window shadows disabled in isolated
  settings reduced diagnostic GPU-interval p95 from 17.00 to 8.37 ms and p99
  from 18.61 to 9.96 ms. This changes appearance and is an attribution probe,
  not a proposed user-facing fix. These are GPU intervals from separate
  instrumented playback runs, not clean frame-time percentiles or a
  frame-for-frame matched replay comparison.

The first experiment preserves the tested panel pixels; the second exercises
production Update/Draw and pane invalidation. Together they identify repeated
window decoration rendering as a real contributor. They do not distinguish
all rasterization, stencil, and target-dependency costs within that work.

### 3. Scene transitions synchronously prepare artwork in Draw

A production playback trace captured a **60.567 ms frame interval** when 14
new artwork sheets entered the scene. Its Draw call took 43.573 ms. The asset
trace attributed 38.244 ms to preparation:

| Preparation stage | Wall time |
| --- | ---: |
| Decode | 2.000 ms |
| CPU artwork processing | 24.358 ms |
| Upload submission | 11.850 ms |

Stage totals have small tracing/rounding differences. Upload submission is
CPU-side elapsed time, not a measurement of GPU transfer execution.

The responsible path is `Game.Draw` → `prepareSceneArtworkFrame` →
`prepareSceneArtwork` in [game.go](../source/game.go), then
`prepareArtworkSheetsInternal` in [images.go](../source/images.go).
Artwork jobs use workers, but the render caller waits for the requested work
and submits the resulting images before continuing.

For this interval, Update took 0.416 ms, snapshot-lock waiting was 0.001 ms,
and no GC occurred. Those explanations do not account for this particular
hitch. The remaining 16.578 ms outside Update/Draw is not fully attributed.
Startup preload and first-scene upload also produced larger pauses, but they
are separate from this later transition and from warmed fixed-scene p95.

The moving runs used the client's 30-second PGO movie mode (30 movie updates
per second), full Update/Draw/UI/audio paths, and isolated settings. This is
accelerated playback, not a clean ordinary-gameplay percentile measurement.
The full asset IDs and both adjacent log lines are retained in the JSON;
the asset-trace and pacing-trace frame counters use different numbering.

## What the earlier API capture did and did not establish

The old fixture submitted 805 draws in a typical warmed frame and 1,237 during
four-pane repaint; 664 and 1,000 were vector stencil draws. These counts explain
why UI rendering was investigated, but are not current tiled-client counts.
Across 119 warmed captured frames there were no texture uploads, texture
allocations, readbacks, or shader compilation calls. That observation applies
only to the warmed fixed scene; the transition trace demonstrates why it must
not be generalized to moving gameplay.

A rectangular-border experiment had no measurable improvement and was
removed. Bubble and lighting feature-off results from the old fixture are
insufficient grounds to prioritize those changes over the causes above.

## Reproduce and extend

Run the corrected clean study on the hardware desktop, one process at a time:

```sh
GOTHOOM_RENDER_FULLSCREEN=0 GOTHOOM_RENDER_VSYNC=0 \
GOTHOOM_RENDER_CASES=baseline,ui_refresh_5hz_unbatched,baseline_repeat \
build-scripts/render_stall_study.sh /tmp/gothoom-tiled-vsync-off

GOTHOOM_RENDER_FULLSCREEN=0 GOTHOOM_RENDER_VSYNC=1 \
GOTHOOM_RENDER_CASES=baseline,ui_refresh_5hz_unbatched,baseline_repeat \
build-scripts/render_stall_study.sh /tmp/gothoom-tiled-vsync-on
```

Use `GOTHOOM_RENDER_DOCKED=0` in a separate run for floating panes. The runner
defaults to three repeats and reverses interior cases; use the full case list
when testing case-order effects. See [the runner guide](LaptopRenderingBenchmark.md)
for assets, display, scale, and power controls. Layout changes slightly affect
world bounds; compare the recorded bounds as well as timings.

For Linux/GLX GPU attribution, build the isolated diagnostic dependency:

```sh
build-scripts/build_gpu_frame_probe.sh /tmp/gothoom-gpu-probe

GOTHOOM_GPU_TIMELINE=/tmp/gothoom-gpu-probe/timeline.csv \
GOTHOOM_RENDER_BINARY=/tmp/gothoom-gpu-probe/render.test \
GOTHOOM_RENDER_REPEATS=1 GOTHOOM_RENDER_FULLSCREEN=0 \
GOTHOOM_RENDER_CASES=baseline,ui_refresh_5hz_unbatched \
build-scripts/render_stall_study.sh /tmp/gothoom-gpu-diagnostic
```

Use a unique timeline path per process; the probe creates/truncates it. The
helper copies Ebitengine v2.9.10, applies the version-specific patch there, and
builds with a temporary modfile. It leaves the checkout's dependencies and Go
module cache untouched. The patch is diagnostic-only and requires the tested
Linux/GLX OpenGL ES timer-query support.

The probe uses asynchronous
[EXT_disjoint_timer_query](https://registry.khronos.org/OpenGL/extensions/EXT/EXT_disjoint_timer_query.txt)
timestamps around framebuffer draw segments, checking availability before
reading results. It does not insert `glFinish` or pixel readbacks. An unsupported
timestamp counter or disjoint clock aborts the probe; discard such a capture.

CSV records:

- `meta,timestamp_bits,bits,error,gl_error`
- `gpu,batch_id,framebuffer_id,elapsed_ns,disjoint`
- `cpu,batch_id,driver_submit_ns,swap_ns,draw_calls,target_segments,batch_start_unix_ns`

Batch IDs identify driver submissions, not Game frame IDs. Multiple batches
can belong to one frame, and GPU records arrive after their CPU records.
Framebuffer numbers are capture-local, not stable pass names. GPU intervals
can include idle gaps while commands arrive, and are not additive to CPU or
presentation durations. Driver timings include diagnostic overhead. Exclude
warmup and incomplete trailing batches and keep instrumented runs separate
from clean comparisons. Early captures used the same CPU record without its
final host-time column.

For normal-speed moving playback, build the client with `go build` and run
`-clmov clmovFiles/tour-2025.08.02.clMov.zip -framePacingTrace 25ms`, using
isolated resource copies and settings with VSync on and power saving off.
Seek to a sustained walking section (the final capture started at movie frame
12290, approximately 40:58 at 5 updates per second). Allow playback to settle
before measuring. The threshold trace reports slow intervals; it does not
produce a complete percentile distribution. The final capture used a temporary
buffered collector for every interval and excluded its startup/seek portion.

Do not use `-pgo` for ordinary-speed walking comparisons: it changes movie
playback to 30 updates per second. The accelerated captures above remain
useful for attribution, but their scene transitions and workload cadence differ
from normal play. This investigation changed no saved user settings.

## Remaining scope

The completed change corrects the fixture, exposes VSync and docking controls,
and retains opt-in diagnostic tooling and measured evidence. No experimental
UI rendering changes remain. The normal client does not use the GPU probe.

If regular walking drops recur, reproduce the same scene and layout at normal
speed with VSync on. Compare frame cadence and scrolling progression, then
correlate slow frames with UI repaints, world rendering and driver submission.
Choose an optimization only after reproducing and attributing the repeated
cost. First-load and rare transition hitches are a separate investigation.

A future production optimization needs matched moving segments and repeated
paired runs, along with visual and interaction checks. The fixed-scene study
can isolate rendering costs, but cannot establish smooth walking on its own.
