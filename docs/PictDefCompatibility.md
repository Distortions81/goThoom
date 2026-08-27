# PictDef compatibility work

This document tracks only picture-definition behavior present in the original
Clan Lord client that goThoom needs to add or adjust. It is limited to behavior
driven by `PictDef` flags and closely related lighting metadata.

The original definitions are in
[`Public_cl.h`](https://github.com/YappyGM/ClanLordClient/blob/6ba334cfb3fb779ecfe37e0b635fac476cb73a5e/mac_client/client/public/Public_cl.h#L604-L678).
The original drawing behavior is primarily in
[`GameWin_cl.cp`](https://github.com/YappyGM/ClanLordClient/blob/6ba334cfb3fb779ecfe37e0b635fac476cb73a5e/mac_client/client/source/GameWin_cl.cp).

## Outstanding flags

The counts below came from the bundled `source/data/CL_Images` file on
2026-08-22. A picture can have more than one flag, so the rows overlap. Flags
that are already supported are intentionally omitted.

| Flag | Value | Asset count | Required work |
|---|---:|---:|---|
| `kPictDefIsShadow` | `0x1000` | 50 | Respect area shadow state and night opacity |
| `kPictDefFlagRandomAnimation` | `0x0004` | 147 | Select animation-table entries randomly |

## Work to add

### 1. Random picture animation (`0x0004`)

Original behavior:

- The flag means that animation-table entries are selected randomly rather
  than by the global frame counter.
- Immediately before drawing a flagged world picture, the original client
  calls `UpdateAnim(true)`.
- `UpdateAnim(true)` chooses `GetRandom(pdNumAnims)` and then maps that entry
  through `pdAnimFrameTable`.
- This behavior is used for lightning and other irregular effects.

Relevant original code:

- [`GameWin_cl.cp` random-animation check](https://github.com/YappyGM/ClanLordClient/blob/6ba334cfb3fb779ecfe37e0b635fac476cb73a5e/mac_client/client/source/GameWin_cl.cp#L2994-L2998)
- [`Cache_cl.cp` random frame selection](https://github.com/YappyGM/ClanLordClient/blob/6ba334cfb3fb779ecfe37e0b635fac476cb73a5e/mac_client/client/source/Cache_cl.cp#L594-L625)

Current goThoom behavior:

- `climg.FrameIndex` always indexes the animation table using
  `frameCounter % numAnims`.
- Every instance of the same picture therefore shows the same sequential
  frame.

Implementation notes:

- Add an exported constant for `PictDefFlagRandomAnimation`.
- Apply random selection only to world pictures, matching the original
  `Draw1Picture` path. Mobile pose selection is separate.
- Choose a random animation-table entry, not a raw picture frame.
- Keep the selected frame stable for the duration of one logical game frame.
  `drawScene` can run several times while interpolating the same server frame;
  selecting again on every display refresh would make lightning run too fast.
- Preserve a previous selection if random pictures participate in world-frame
  blending. A simpler first implementation may disable frame blending for
  random-animation pictures, which is closer to the original client.
- Movie playback and seeking should be repeatable. Prefer a deterministic hash
  of the logical frame and picture identity, or save the chosen frame in the
  recorded/draw state, rather than relying on process-global random state.

Tests should verify that:

- Unflagged pictures retain the current sequential animation behavior.
- A flagged picture selects only valid entries from its animation table.
- A flagged picture remains stable across repeated draws of one logical frame.
- Different logical frames are not forced into sequential order.
- Movie seek and replay produce the same selected frames.

### 2. Explicit shadow pictures (`0x1000`)

Original behavior:

- A flagged picture is not drawn when the area's shadow level is zero.
- At night levels above 33, when the effective night limit is also above 33,
  the picture is forced to the least-opaque blend level (`kPictDef75Blend`,
  approximately 25% visible).
- The original OpenGL path can also render these through its shadow/stencil
  path.

Relevant original code:

- [`GameWin_cl.cp` explicit-shadow handling](https://github.com/YappyGM/ClanLordClient/blob/6ba334cfb3fb779ecfe37e0b635fac476cb73a5e/mac_client/client/source/GameWin_cl.cp#L2940-L2953)
- [`GameWin_cl.cp` night blend override](https://github.com/YappyGM/ClanLordClient/blob/6ba334cfb3fb779ecfe37e0b635fac476cb73a5e/mac_client/client/source/GameWin_cl.cp#L2988-L2992)

Current goThoom behavior:

- `PictDefIsShadow` (`0x1000`) is exposed by `climg` and checked by world-picture
  drawing.
- Explicit shadow pictures are skipped at shadow level zero and drawn at 25%
  alpha when both the raw night level and effective user/server night limit are
  above 33.

Implementation notes:

- Keep the night alpha override in draw state rather than changing the cached
  image, because the same asset can be reused under different lighting state.

Tests should cover shadows enabled, `kLightNoShadows`, daylight, and night
levels on both sides of the 33 threshold.

## Recommended implementation order

1. Random animation.
2. Explicit shadow-picture suppression and night blending.

Both items are contained changes with direct asset coverage.

## Completion checklist

- Every outstanding `PictDef` flag has a named constant and implemented
  behavior.
- Flag behavior is tested independently of Ebitengine drawing where possible.
- Random behavior is stable within a logical game frame.
- Recorded movies replay and seek deterministically.
- Existing sequential animations, custom colors, transparency, and blending do
  not regress.
- Lighting is checked with shader lighting both enabled and disabled.
- Shadow behavior is checked at day, night, cloudy, and no-shadow area states.
- Visual results are compared with the original client for representative
  lightning and explicit shadow pictures.
