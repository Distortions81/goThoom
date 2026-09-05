# Sprite cache measurements

Measured 2026-09-05 using all nine bundled CLMov files: 195,327 accepted game updates, zero rejected packets, and 3,284 first sprite-ID encounters (each movie starts with a fresh cache).

## Method

`TestSpriteCacheMovieReloadPressure` replays the production movie parser, draw-state handler, scene artwork requests, preallocation distribution, slot selection, and game-frame LRU. It decodes encountered sheets and enumerates every nonempty pose, matching first-use batch preparation. It includes recolor masks and interpolation pins, assumes GPU recoloring and blending enabled, and disables denoising. Default procedural-effect replacements are excluded from sprite demand.

This is a metadata replay, not a rendered frame-time benchmark. No upscaling or GPU upload is performed. Upload figures model full RGBA slot overwrites, including padding. Allocated slot area excludes atlas borders, packing gaps, base sheets, and non-game textures; actual VRAM is higher. The reserve is soft, so the pool can grow beyond it. Incompatible sizes are retained, even when unused. Only occupied slot area contributes to the eviction threshold; idle reserve does not.

A reload batch is a previously prepared sheet/palette requested again after eviction. ID reloads count a new residency of a previously evicted sprite ID, once for all its poses and masks. Slot reuse alone is not a reload: a new sprite may occupy an old allocation.

## Final policy across all movies

The matrix below uses absolute reserve sizes, after any preset scaling.

Free slots may serve a smaller sprite up to twice its required area. LRU eviction still requires an exact allocation size, avoiding eviction of a larger animation batch merely to supply a small slot.

| Upscale | Reserve MiB | ID reloads | Game updates with reloads | Reload upload MiB | Largest reload/update MiB | Maximum allocated slot MiB |
|---|---:|---:|---:|---:|---:|---:|
| 2x | 128 | 2,053 | 0.958% | 1,573.6 | 15.9 | 390.1 |
| 2x | 256 | 801 | 0.393% | 440.0 | 8.1 | 493.6 |
| 2x | 512 | 34 | 0.015% | 10.2 | 2.0 | 750.6 |
| 2x | 1024 | 0 | 0.000% | 0.0 | 0.0 | 1240.7 |
| 2x | 2048 | 0 | 0.000% | 0.0 | 0.0 | 2248.2 |
| 4x | 128 | 3,495 | 1.610% | 8,088.3 | 46.8 | 1365.2 |
| 4x | 256 | 2,747 | 1.287% | 6,810.7 | 46.3 | 1417.4 |
| 4x | 512 | 1,330 | 0.637% | 3,101.8 | 35.1 | 1576.7 |
| 4x | 1024 | 560 | 0.258% | 869.0 | 30.5 | 2070.2 |
| 4x | 2048 | 6 | 0.003% | 11.7 | 7.5 | 3194.4 |

These corrected results show a substantial reduction in reload work from 256 to 512 MiB at 2x, and zero reloads at 1 GiB for these inputs. Larger reserves also materially help at 4x. The Balanced preset uses 512 MiB at 2x, 1,152 MiB at 3x, and 2 GiB at 4x. Its 2x and 4x reserves nearly eliminate reloads in this study; 3x follows the same area-based scaling rule but was not replayed here. Smaller reserves remain available to leave more room for other textures. These percentages do not establish a p95 frame-time improvement.

The percentage above divides updates containing any reload by all 195,327 game updates. It is not the fraction of sprite requests that missed. Each reserve/scale combination receives 6,658,758 nonempty sheet requests: at 2x, 256 MiB has 802 reload-batch misses (0.01204% of requests), 512 MiB has 34 (0.00051%), and 1 GiB has none. First encounters are counted separately.

## 4x with a 2 GiB reserve

Extending the replay through 2 GiB reproduced every lower-reserve result exactly.
At 4x/2 GiB, eight movies had no reloads. `lore1` had six ID reloads across five
game updates, totaling 11.7 MiB of modeled uploads; its largest reload update
was 7.5 MiB. This nearly eliminates reload work for these inputs, compared with
869.0 MiB at 1 GiB and 3,101.8 MiB at 512 MiB.

The largest allocated pool was `newTest` at 3,194.4 MiB (3.12 GiB), including
880.4 MiB of idle slots. Atlas overhead and non-game textures are additional.
The 2 GiB value remains a soft reserve and cache-pressure target, not a VRAM cap.

## 2x with a 256 MiB reserve

| Movie | ID reloads | Game updates with reloads | Reload upload MiB | Final slot MiB |
|---|---:|---:|---:|---:|
| 2004 | 0 | 0.000% | 0.0 | 268.2 |
| 2025a | 352 | 1.617% | 154.0 | 473.7 |
| chain | 13 | 0.482% | 9.5 | 401.0 |
| chaintest | 0 | 0.000% | 0.0 | 261.3 |
| concert1 | 0 | 0.000% | 0.0 | 257.5 |
| lore1 | 258 | 0.535% | 150.2 | 442.5 |
| newTest | 83 | 0.186% | 51.6 | 493.6 |
| test | 0 | 0.000% | 0.0 | 324.2 |
| tour-2025.08.02 | 95 | 0.263% | 74.6 | 406.5 |

Of the 3,284 first ID encounters, 98 appeared in only 1–5 accepted game updates and another 480 in 6–25. This measures total presence within each movie, not distinct area visits.

## Corrected eviction-pressure bug

Earlier results incorrectly counted all allocated slot area, including idle preallocation, against the eviction threshold. A full startup reserve could therefore force eviction while the live cache was nearly empty. Increasing the reserve mainly added more idle space, explaining the misleading plateau in reloads. LRU ordering by last game update was correct; eviction pressure was not.

The threshold now uses occupied slot area. The allocator still borrows suitable free slots and can grow when no evictable slot fits, so this is not a hard memory limit. A regression test reserves an incompatible large slot and verifies that adding two tiny sprites does not evict the first one.

The previous 2x results of 1,968 / 1,835 / 1,827 ID reloads for 256 / 512 / 1,024 MiB are superseded by 801 / 34 / 0. The earlier conclusion that larger reserves barely helped was incorrect.

## Repeat the study

Run separately from other tests because the probe replaces global game state. It skips during the normal suite. Paths should be absolute.

```sh
cd source
GOTHOOM_SPRITE_CACHE_STUDY=/tmp/gothoom-sprite-reloads \
GOTHOOM_SPRITE_CACHE_IMAGES=/absolute/path/to/CL_Images \
xvfb-run -a go test . -run '^TestSpriteCacheMovieReloadPressure$' -count=1 -v
```

The matrix covers 2x and 4x with 128, 256, 512, 1,024, and 2,048 MiB reserves.
Optionally set `GOTHOOM_SPRITE_CACHE_MOVIE` to one movie path. Each movie produces a JSON report including batch requests and hits, first/reload batch and slot counts, byte volumes, retained idle space, and presence-frequency bins.

Archive SHA-256: `2d82311acec4d27259c32120c982e3e1ec7f05685d91c1b073253e2758872726`.
