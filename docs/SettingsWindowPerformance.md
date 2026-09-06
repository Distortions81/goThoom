# Settings window performance, September 2026

Idle scrollbar hit testing measured nested content and tab labels even when
the pointer was nowhere near a scrollbar. A CPU profile attributed 0.60 s of
the 1.05 s sampled in `eui.Update` to that path. Scrollbar hit testing now
rejects pointers outside the scrollbar strips before measuring content.
Tab bounds measure their labels once, and click-feedback cleanup visits only
the active tab's contents. Hidden click feedback expires before its next paint.

## Measurements

One sequential, unprofiled comparison using `-bubbleTorture`, with five seconds
of warmup and 30 seconds of sampling per process. Linux hardware display,
AMD Radeon integrated graphics (Renoir), OpenGL, Go 1.26.6, 1920x1000 window,
2x game/artwork scale, UI scale 1, VSync and bubble animation disabled, audio
disabled. The pointer was fixed outside Settings; the open window showed
Display. A temporary build overlay collected normal Game.Update and Game.Draw
timings and Update-to-Update frame intervals, including presentation waits.

| p95 measurement | Before, open | After, open | After, closed |
| --- | ---: | ---: | ---: |
| Update | 0.237 ms | 0.174 ms | 0.133 ms |
| Draw submission | 0.881 ms | 0.893 ms | 0.862 ms |
| Frame interval | 3.136 ms | 3.032 ms | 2.864 ms |

Update p95 fell 27%; frame p95 fell 3%. Frame p99 was essentially unchanged
(4.280 vs 4.287 ms). Settings painted only once in each open-window run,
including startup, so continuous repainting did not explain this case.
There is still overhead with Settings open. These single-run stress samples
do not establish a general FPS improvement or isolate the remaining GPU cost.

Raw reports: [before, open](benchmarks/settings_open_before.json),
[after, open](benchmarks/settings_open_after.json),
[after, closed](benchmarks/settings_closed_after.json).
