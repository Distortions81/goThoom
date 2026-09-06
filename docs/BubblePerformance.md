# Bubble rendering and layout, September 2026

The busy-scene CPU profile identified speech-bubble decoration as a major
Draw cost: spikes and ponder lobes submitted many individual vector paths.
The renderer now batches each decoration into one path and caches complete
static decorations by dimensions, scale, shape, and colors. Moving a bubble
or changing its text does not invalidate the decoration surface.

The cache shares the existing plain-body limits: 256 entries and 16 MiB of
active surfaces, plus up to 8 MiB of idle pooled targets. Cold ponder and
telepathy cache misses use small pooled masks instead of resizing the
full-world mask. Text remains in its separate existing cache. Animation is
off for new settings by default; saved preferences and the animation option
remain available. Animated decorations still benefit from path batching.

## Measurements

Hardware display on Linux, AMD Radeon integrated graphics (radeonsi/renoir),
Go 1.26.6, OpenGL, VSync disabled. The warmed tour fixture contains 146
pictures and 30 mobiles at 50% night, 2x artwork, UI scale 1, and a 1920x1080
render surface presented at 960x540. Each case has 90 warmup frames and at
least 600 samples over at least 10 seconds. Rendering continues when the
test window loses focus.

| Measurement | Before | Final | Reduction |
| --- | ---: | ---: | ---: |
| Frame interval p95 | 6.437 ms | 5.079 ms | 21% |
| Draw submission p95 | 5.494 ms | 2.726 ms | 50% |
| Frame interval p99 | 7.301 ms | 5.331 ms | 27% |

Both sides have bubble animation disabled, so the comparison does not credit
the new default for this improvement. The baseline uses the previous bubble
renderer, with the same harness and defaults. A reverse-order confirmation
(after, then before) measured 5.055 ms after versus 6.437 ms before at frame
p95. The final run above includes the subsequent caption and inset changes,
but predates the later tail-join outline refinement.
Neither confirmation nor the final run had frames above 16.67 ms.

These are warmed rendering measurements, not live gameplay latency. The
harness does not exercise normal Game.Update, network traffic, or asset
loading. Longer exploratory sequences showed wall-time drift, so this
comparison uses fresh-process single-case runs; expect variation on other
hardware and under different desktop load. Draw submission includes CPU
work and driver waits, not a pure GPU timestamp measurement.

Raw reports: [before](benchmarks/bubble_before.json),
[after confirmation](benchmarks/bubble_after_confirmation.json),
[final](benchmarks/bubble_after_final.json).

Reproduce the final case from `source/` on a hardware display:

```sh
go test -c -o /tmp/gothoom-render.test
GOTHOOM_PERF_IMAGES="$HOME/.local/share/goThoom/CL_Images" \
GOTHOOM_RENDER_SCALE=2 GOTHOOM_RENDER_SAMPLES=600 GOTHOOM_RENDER_SECONDS=10 \
GOTHOOM_RENDER_CASES=baseline GOTHOOM_RENDER_STUDY=/tmp/bubble-render.json \
/tmp/gothoom-render.test -test.run '^TestRenderStallStudy$' -test.v
```

## Appearance and validation

CL_Images 1–3 inform paired light/dark treatments: quieter dotted whispers,
larger and fewer yell/growl spikes, simpler ponder clouds, and a small
sunstone marker for tail-free telepathy. Actions and narration are rectangular
caption boxes with regular-weight text and no pointers. Spoken pointers aim
at a face estimate derived from frame size and facing; bodies avoid their
own speaker's frame. Artwork does not supply exact mouth coordinates.

Settings tabs use measured label widths and fit on one row at the tested
default settings. Fixed panels still wrap tabs when space is insufficient.
Tab height accommodates font metrics, and button width checks include icons
and markers.

The opt-in rendered tests check cached versus direct pixels (at most one
channel level of rounding difference), cache reuse and both cache limits,
as well as tab/button fit across styles and scales:

```sh
GOTHOOM_RENDER_STATIC_BUBBLES=1 xvfb-run -a go test . -run '^TestRenderStaticBubbleCache$' -count=1
GOTHOOM_RENDER_BUBBLE_CACHE_TEST=1 xvfb-run -a go test . -run '^TestRenderCachedBubbleBodyMatchesDirectPaths$' -count=1
GOTHOOM_RENDER_TEXT_FIT=1 xvfb-run -a go test . -run '^TestRenderControlTextFit$' -count=1
```

These checks and `go build ./...` pass. At the time of the measurements, the
full suite had a pre-existing `TestVersionEntriesAndChangelogsStayInSync`
failure because changelog `51.txt` lacked a versions.json entry. The version
51 metadata update subsequently resolved that mismatch.

## Moving bubble torture follow-up

The actual `-bubbleTorture` client mode was also measured for 30 seconds
after five seconds of scene warmup, with the normal Game.Update and Game.Draw
loops running. This covers moving speakers, camera motion, changing text and
bubble types, layout work, and cache misses. Audio was disabled; the synthetic
scene does not use a server connection. Both fresh-process runs used 2x
artwork/game scale, UI scale 1, dark bubbles, animation off, and a 1920x1000
window on the same hardware display. VSync was explicitly disabled after
startup and confirmed off through Ebitengine's own query at completion.

| Measurement | Before | Current | Reduction |
| --- | ---: | ---: | ---: |
| Frame interval p95 | 4.876 ms | 2.849 ms | 42% |
| Frame interval p99 | 5.342 ms | 4.077 ms | 24% |
| Draw submission p95 | 3.874 ms | 0.843 ms | 78% |
| Update p95 | 0.158 ms | 0.129 ms | 18% |

There were 8,422 samples before and 12,229 after. The current run still had
two intervals over 16.67 ms (0.016%); its maximum was 20.00 ms, versus one
interval over 16.67 ms and a 17.92 ms maximum before. The p95 improvement
therefore does not imply every worst-case spike improved.

The baseline substitutes bubble.go and game.go from `1f062ce` while retaining
the current UI and identical instrumentation. The current build includes
the tail-outline gaps. Temporary Go build overlays collected timings in
memory through the frame-pacing hooks and wrote JSON only after sampling;
the normal per-frame pacing logger was replaced for these runs. Initial
setup runs were discarded because startup changed the requested VSync
setting. The reported pair ran before then after, with both the settings
and engine flags confirmed false and the setup wizard closed.

Raw reports: [before](benchmarks/bubble_torture_before.json) and
[current](benchmarks/bubble_torture_after.json). This is one controlled pair,
not a claim about all scenes or hardware.
