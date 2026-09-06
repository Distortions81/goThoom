# Sprite upload padding, September 2026

Sprite slots round dimensions up to 32-pixel classes and reserve at least one
transparent texel beyond the sprite view. Frame extraction removes source-sheet
padding; the upload must still provide the slot's sampling gutter.

Packed sources now upload their content directly when a slot is fresh or its
previous occupant had the same dimensions: `width*height*4` bytes, without an
application staging allocation, copy, or clear. Ebitengine still owns its
internal copying and GPU transfer. Reusing an unchanged view size preserves
the already-transparent sampling border.

Resized or strided sources write only `(width+1)*(height+1)*4` bytes instead of
the entire allocation. Stale pixels farther outside the view remain untouched. Each
sprite still uses one rectangular WritePixels call in the same artwork batch;
there is no separate GPU clear or work deferred to later frames. The CPU copy
overwrites the content directly; only the right and bottom gutter is zeroed.

The GPU reuse test compares against the previous full-slot upload for six
successive occupants of a 64x64 slot: 48x8, 8x48, 36x36, 2x33, 50x50, and
8x33. The submitted buffers total **21,040 bytes versus 98,304 bytes**, a
**78.6% reduction in this synthetic fixture**. Actual savings depend on sprite
sizes and slot reuse. This counts API-submitted pixel bytes, not measured GPU
transfer time or an FPS improvement.

Checks cover nonzero source origins, strided source images, retained stale
pixels outside the sampling area, a clean gutter, earlier queued draws,
preallocated slots, and nearest/linear filtering with fractional placement,
rotation, and scaling. Filtered results match an equivalent full-slot upload
within one channel value.

Run from `source/` on a display:

```sh
GOTHOOM_RENDER_SPRITE_SLOT_TESTS=1 go test . -run '^TestRenderSpriteSlotReuse$' -count=1 -v
```

The direct-path GPU test checks fresh and same-size reuse, a resize that must
fall back, then direct reuse of that resized slot. It verifies packed images
with nonzero origins and immediate recycling of the source buffer, comparing
filtered output against the former full-slot upload.
