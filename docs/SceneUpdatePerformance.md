# Scene updates and chat performance, September 2026

Camera-motion matching stores candidate background indices in reusable slices
and deduplicates only the winning movement. Its lookup tables use integer keys.
Chat classification reads the speaker's presence and NPC status directly from
the owning player directory. Crowded speech-bubble snapshots reuse their
deduplication lookup table.

Regression checks cover weighted camera movement, negative movement, duplicate
matches, small artwork, excluded artwork, movement limits, and ambiguous
movement. Chat checks cover players, NPCs, creatures, self messages, case folding,
descriptor-only NPCs, and isolation between sessions. Snapshot checks cover
allocation reuse, the order of surviving bubbles, custom deduplication IDs,
replacement messages, and expiry.

## Measurements

Linux/amd64, Ryzen 9 7950X, Go 1.27.1, Ebitengine 2.10.2, `CGO_ENABLED=0`.
The existing `default.pgo` profile was used unchanged. Xvfb supplied the display
required by the test process. These benchmarks measure client CPU processing
and allocations; GPU rendering, frame rate, and network latency are outside
the measurements.

The baseline uses production files from commit `7b519c5` with the same benchmark
code as the optimized build. A temporary Go source overlay supplied the original
`draw.go`, `game.go`, and `script.go` without changing the working tree. Both
executables were built before timing. Benchmarks for this change ran sequentially,
with their builds and validation tests outside the timed runs. Desktop activity
outside this task was not controlled.

The first comparison ran optimized code followed by baseline code, with a
one-second minimum per sample. Each result below is the median of three samples.
The tour recording contains 31,908 draw-state packets. Its busiest scene has
146 pictures and 30 mobiles. The chat benchmark creates a directory of 1,000
known players with color palettes.

| Operation | Baseline | Optimized | Result |
| --- | ---: | ---: | --- |
| Dense camera matching, 512 pictures | 69.99 µs | 39.01 µs | 44.3% less time |
| Tour state updates | 731.25 ms | 536.16 ms | 26.7% less time |
| Tour updates including render caches | 944.09 ms | 765.33 ms | 18.9% less time |
| Main chat speaker classification | 135.883 µs | 0.2343 µs | 99.8% less time |
| Session chat speaker classification | 410.717 µs | 0.2304 µs | 99.9% less time |
| Busy-scene render-cache preparation, unchanged control | 13.65 µs | 13.33 µs | Similar timing |

| Allocated memory per operation | Baseline | Optimized |
| --- | ---: | ---: |
| Tour updates including render caches | 31,614,304 bytes / 215,850 allocations | 4,373,100 bytes / 129,625 allocations |
| Main chat speaker classification | 573,153 bytes / 2,005 allocations | 96 bytes / 3 allocations |
| Session chat speaker classification | 1,250,673 bytes / 6,011 allocations | 96 bytes / 3 allocations |
| Busy-scene snapshot after warmup | 1,192 bytes / 3 allocations | 0 bytes / 0 allocations |

A second replay comparison reversed the order: baseline followed by optimized
code, with a two-second minimum and three samples per case.

| Replay confirmation | Baseline | Optimized | Reduction |
| --- | ---: | ---: | ---: |
| State updates | 814.18 ms | 537.66 ms | 34.0% |
| Updates including render caches | 1,095.39 ms | 832.00 ms | 24.0% |

The two comparisons show 27–34% less time for state updates and 19–24% less
with render caches included. Allocated memory remains about 86% lower.
Absolute timings varied between runs. Snapshot timings also varied during
exploration, so the snapshot improvement reported here is allocation removal.
These results do not establish a gameplay FPS improvement.

Raw results: [first baseline](benchmarks/scene_update_before.txt),
[first optimized](benchmarks/scene_update_after.txt),
[confirmation baseline](benchmarks/scene_update_confirm_before.txt),
[confirmation optimized](benchmarks/scene_update_confirm_after.txt).

## Reproduce

From `source/`, using the normal installed artwork:

```sh
GOTHOOM_PERF_IMAGES="${XDG_DATA_HOME:-$HOME/.local/share}/goThoom/CL_Images" \
CGO_ENABLED=0 xvfb-run -a go test -run '^$' \
  -bench '^Benchmark(PictureShiftDense|ClassifyScriptChat|TourDrawState|PrepareBusySceneRenderCache|CaptureBusySceneSnapshot)$' \
  -benchmem -benchtime=1s -count=3
```

The recording fixture is `source/clmovFiles/tour-2025.08.02.clMov.zip`.
The camera and chat benchmarks use synthetic data. To repeat the longer replay
confirmation, select `^BenchmarkTourDrawState$` and use `-benchtime=2s`.
