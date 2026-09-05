# Laptop rendering benchmark

> Fixture correction: the [GPU audit](GPUStallAudit.md) found a game-window
> background/shadow absent from the client and missing docking setup. The
> current fixture shares the client's game-window rendering policy and applies
> production pane chrome. Reports now record `gameWindowNoBackground` and
> `docked`; historical reports without these fields used the old fixture.

> Status: The uncommitted bubble rendering experiment was reverted on 2026-09-05.
> Bubble behavior and the animation default are back to the committed implementation.
> The experiment descriptions and before/after captures below are historical; they
> do not describe the current bubble renderer. Name-tag and UI work remains.

Target: the user's Linux laptop with a Ryzen 5 5500U and integrated graphics.
Use its real desktop session. The RX 7900 XT desktop results are useful for
finding CPU work, but do not establish whether the changes help an iGPU.

## What we are testing

Keep animated yells, growls, and ponders, while simplifying their rendering:

- Yells pulse outward; growls have a slower jagged ripple; ponders retain a
  rolling cloud edge. Spikes and cloud lobes use combined vector paths. These
  reduce application submissions, but do not guarantee fewer GPU passes:
  Ebitengine already batches compatible `FillPath` calls internally, and a
  combined path changes the packing and area of intermediate stencil images.
- Bubble size still follows the displayed character scale. Curves and text
  render at final resolution. Spatial animation phase uses relative distances,
  so increasing display scale does not introduce more ripples.
- Static thought bodies and nonanimated ponder bodies use pooled managed
  textures. Animated ponder bodies blend directly with one nonzero-fill path,
  avoiding the additional application mask/composite pass for that body.
  Thought/ponder tails still use the existing mask path.
- Name tags use a managed LRU cache with a 64 MiB active allocation budget and
  an 8 MiB idle reserve. Borrowed images cannot be recycled until their draw
  has been submitted. The active budget can be exceeded while entries are held.
- UI diagnostics record per-window repaint counts, pixel area, deferrals,
  invalidation reason, and CPU submission duration. An experimental scheduler
  spreads background repaints using a one-million-pixel budget and a 50 ms
  deadline; initial paints, resizes, and interaction bypass it.

**UI repaint staggering is not enabled in normal window constructors.** The
desktop experiment reduced its worst spikes but made more frames moderately
slow. It remains an explicit benchmark case until iGPU evidence supports it.
No custom bubble shader has been introduced. Ebitengine 2.9's vector renderer
already uses a shader and intermediate stencil texture; a specialized analytic
bubble shader remains a possible later experiment, rather than a prerequisite.
See the [Ebitengine 2.9 rendering notes](https://ebitengine.org/en/documents/2.9.html).

## Preliminary desktop evidence

The first optimized 4x run reduced `Game.Draw` submission p95 from 2.45 to
1.48 ms, approximately 40%, with animated bubbles enabled. Frame p95 was
essentially unchanged: 6.43 versus 6.44 ms. This is a CPU submission improvement,
not a demonstrated GPU or end-to-end p95 improvement.

These short runs sampled 600 frames per case in a 960x540 presentation window.
They are exploratory. A 5 Hz repaint burst can affect p95 or only p99 depending
on how many frames it occupies, especially with an uncapped fast GPU. The laptop
runner therefore samples each case for at least ten seconds and 600 frames,
repeats the study, and records the percentage of frames exceeding 16.67/33.33 ms.
Original measurements and limitations are in [RenderStallStudy.md](RenderStallStudy.md).

## Before starting

1. Plug in the laptop. Keep its power profile, display resolution, refresh rate,
   desktop scaling, and compositor settings identical between comparisons.
   Record whether an external display is connected. Do not change profiles
   automatically between runs.
2. Stay logged into the graphical desktop, with the benchmark visible. Close
   other GPU-heavy applications and avoid screen recording during timing.
3. Use the same `CL_Images` file and bundled
   `source/clmovFiles/tour-2025.08.02.clMov.zip` for both variants. The runner
   records asset and executable hashes.
4. The source build requires the project resources and official Go 1.26.6.
   Follow the repository setup instructions for a clean checkout. A supplied
   Linux benchmark binary can run without installing Go on the laptop.
5. `glxinfo` must be available (the Debian/Ubuntu `mesa-utils` package), along
   with Python 3. The runner selects Ebitengine's OpenGL backend, records
   `glxinfo -B`, and refuses llvmpipe/softpipe software rendering.

The test opens its own window, defaults to VSync off, and disables power-saving frame limits,
and uses temporary application data. It does not connect to the game server.
It does not change the user's saved settings.

## Run from a checkout

From the repository root, in a desktop terminal:

```sh
build-scripts/render_stall_study.sh /tmp/gothoom-laptop-current
```

Defaults: fullscreen presentation, render and artwork scale 2, three repetitions, ten seconds
minimum per case. Allow roughly five minutes plus startup/warmup time; a slow
system can take longer because every case also requires at least 600 frames.
Each repetition starts a fresh process. Alternate repetitions reverse the
interior case order; baseline remains at the beginning and end to reveal drift.

To locate game assets outside the default Linux data directory:

```sh
GOTHOOM_PERF_IMAGES=/path/to/CL_Images \
build-scripts/render_stall_study.sh /tmp/gothoom-laptop-current
```

Compare VSync in separate output directories, keeping all other settings the
same. The runner defaults to `GOTHOOM_RENDER_VSYNC=0`; each JSON report records
the selected setting. Use `1` for VSync on:

```sh
GOTHOOM_RENDER_VSYNC=1 \
build-scripts/render_stall_study.sh /tmp/gothoom-laptop-vsync-on
```

Keep windowed and fullscreen comparisons separate. The desktop compositor
can affect presentation even when the application requests VSync off. See
[GPUStallAudit.md](GPUStallAudit.md) for the measured 5500U comparison and draw-call audit.

The corrected fixture defaults to docked panes. Set `GOTHOOM_RENDER_DOCKED=0`
for a separate floating-window run. This also sets the corresponding client
layout preference; compare reports' world bounds when changing layout.

After the primary 2x comparison, repeat at 3x; 4x is an optional high-load case:

```sh
GOTHOOM_RENDER_SCALES='3 4' \
build-scripts/render_stall_study.sh /tmp/gothoom-laptop-high-resolution
```

Here **render scale is the fixed scene's render resolution**, not solely the sprite
upscaler preference. Scale 2 renders 1920x1080 with UI scale 1; scale 3 renders
2880x1620 with UI scale 1.5; scale 4 renders 3840x2160 with UI scale 2. Fullscreen
presentation fits that surface to the physical panel. A 1080p panel presenting
the 4x case therefore exercises 4K rendering downsampled to 1080p. The JSON
records monitor size, device scale, and actual world bounds. Do not describe
this as native 4K fullscreen unless the panel is actually displaying 4K.
Artwork scale now defaults to the selected render scale and can be overridden
independently with `GOTHOOM_RENDER_ARTWORK_SCALE=2`, `3`, or `4`. Both are recorded
in JSON. The original desktop study and first laptop pair inherited the app's
4x artwork default even when the render surface was 1920x1080; their old JSON
does not contain the new `artworkUpscale` field.

Use `GOTHOOM_RENDER_FULLSCREEN=0` only for a separate, explicitly labeled
960x540-window comparison. Do not compare windowed and fullscreen results as
if presentation were unchanged.

## Run a supplied before/after package

The prepared package contains `render-before.test`, `render-after.test`, the
movie fixture, and `build-scripts/render_stall_study.sh`. Both binaries use the
same benchmark and UI instrumentation. The before variant restores
`bubble.go`, `name_tag_cache.go`, `draw.go`, and `game.go` from commit
`4e37f53a0fc99acaedab1cb46453bc6152e8731a`; the after variant uses the working
changes. This isolates the bubble/name-tag work while retaining identical
repaint-scheduler cases. It is not a comparison of two complete release builds.

From the extracted package directory:

```sh
export GOTHOOM_PERF_MOVIE="$PWD/tour-2025.08.02.clMov.zip"
GOTHOOM_RENDER_BINARY="$PWD/render-before.test" \
build-scripts/render_stall_study.sh /tmp/gothoom-laptop-before

GOTHOOM_RENDER_BINARY="$PWD/render-after.test" \
build-scripts/render_stall_study.sh /tmp/gothoom-laptop-after
```

Repeat in the opposite before/after order if results drift as the laptop heats
up. Compare the separate repetitions, not just the best run. No profiling or
race instrumentation should be enabled for the reported timing comparison.

## Running over SSH

SSH is sufficient, but the process must draw on the laptop's local desktop,
not an SSH-forwarded display or Xvfb. In a terminal on that desktop, inspect:

```sh
printf 'DISPLAY=%s\nXAUTHORITY=%s\n' "$DISPLAY" "$XAUTHORITY"
glxinfo -B
```

Use those exact environment values in the SSH session. Do not guess `:0` if
the local desktop is using another display. `XAUTHORITY` is a path, not the
cookie contents; never copy authentication cookies into benchmark reports.
First verify `glxinfo -B` over SSH reports the expected integrated Radeon GPU.
Avoid `ssh -X`/`ssh -Y`, which measure a forwarded display instead.

The laptop must remain logged in and awake. The benchmark takes over its
fullscreen window for each run. Results stay in the specified output directory;
they can be collected over SSH afterward.

## Cases and results

- `baseline` / `baseline_repeat`: all default scene effects, animated bubbles.
- `all_shaders_off`: disables the application's custom shader master switch,
  including GPU mobile recoloring; Ebitengine's own rendering shaders remain.
- `lighting_off`, `fast_shadows`, `shadows_off`: isolate lighting/shadow options.
- `bubble_animation_off`: freezes decoration and enables static ponder caching.
- `bubbles_off`: removes bubble geometry **and text**, providing an upper bound
  on the potential bubble-related saving, not a geometry-only measurement.
- `ui_refresh_5hz`: four panes refreshed together, experimental staggering on.
- `ui_refresh_5hz_unbatched`: same invalidations with immediate cached repaints.
- `ui_hidden`: four side panes hidden.

The runner writes `summary.md`, per-case JSON, a baseline screenshot per run,
logs, GPU/system information, hashes, and checkout status when run from a repo.
JSON also includes per-window repaint statistics and the actual graphics backend.

Evaluate frame p95/p99 and the percentage exceeding 16.67 ms first, then CPU
submission time and image-memory usage. GPU image bytes are Ebitengine's
allocation estimate, not total shared-memory consumption or observed hardware
residency. A lower CPU number alone does not prove smoother presentation.

This study freezes the tour movie's busiest scene (146 pictures, 30 mobiles)
and calls production `Game.Draw`. It excludes movie parsing, network/game
updates, cold shader compilation, and transient sprite paging. After selecting
the better renderer, confirm it during a full tour replay and the bubble torture
scene. Use `-uiRepaintTrace 1ms`, `-assetLoadTrace 8ms`, or
`-framePacingTrace 20ms` for a separate diagnostic run; logging should remain
disabled during clean timing comparisons. Short isolation probes can set
`GOTHOOM_RENDER_SAMPLES=120` and `GOTHOOM_RENDER_SECONDS=2`; label these as
exploratory and retain the 600-sample, ten-second defaults for confirmation.
