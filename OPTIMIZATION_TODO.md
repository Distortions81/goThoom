# Optimization TODO

Baseline benchmarks were collected on an AMD Ryzen 9 7950X with Go 1.26.6.
Keep changes small, benchmarkable, and behavior-preserving.

- [x] Avoid per-frame player palette allocations.
  - Reuse immutable appearance color data while drawing mobiles.
  - Preserve player-color overrides and blending behavior.
- [x] Reduce per-frame draw snapshot allocations.
  - Remove unused snapshot fields.
  - Reuse owned snapshot maps and slices between sequential draws.
- [x] Reduce image denoising allocation and worker overhead.
  - Baseline, 64x64: about 115-120 us/op, 119.7 KB/op, 69 allocs/op.
  - Reuse HSV scratch storage and avoid excess workers for small sprites.
  - Result, 64x64: about 103-104 us/op, 18.4 KB/op, 35 allocs/op (about 12% less runtime and 85% less memory).
  - Result, 256x256: about 1.4-1.7 ms/op, 0.33-0.36 MB/op, 35 allocs/op (about 81% less memory; no clear timing change).
- [x] Index previous picture positions used by object pinning.
  - Replace repeated scans with exact-position lookups.
  - Dense lookup result: about 12.7 us/op down to 1.2-1.4 us/op, 0 allocs/op (about 90% less runtime, or 9.8x faster).
- [x] Cache mobile dimensions and stationary hover queries.
  - Avoid repeated sprite-sheet lookups and unchanged world scans.
  - Cached CL_Images size lookup: about 4.1 ns/op, 0 allocs/op.
- [x] Normalize chat trigger text once per message.
  - Avoid repeated lowercase conversions for each trigger phrase.
- [x] Run final tests, race checks, cross-platform compilation, and benchmarks.
  - Normal package tests, race tests, and `go vet` pass.
  - Linux, Windows, and WebAssembly compilation checks pass.

## Existing baselines

- Dense picture shift: about 54-55 us/op, 4 KB/op, 1 alloc/op.
- Draw-state parsing: about 119 ns/op, 0 B/op, 0 allocs/op.
- Reliable input send: about 89-96 ns/op, 32 B/op, 2 allocs/op.
- Unreliable input send: about 112-129 ns/op, 52 B/op, 2 allocs/op.
- Compiled script call: about 180-185 ns/op, 0 B/op, 0 allocs/op.
- Yaegi script call: about 18.9-19.6 us/op, 320 B/op, 8 allocs/op.
