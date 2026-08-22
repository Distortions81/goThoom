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
| `kPictDefFlagUprightShadow` | `0x0800` | 515 | Add generated mobile shadows and upright projection |
| `kPictDefFlagOnlyAttackPosesLit` | `0x0100` | 30 | Restrict mobile lighting to attack poses |
| `kPictDefFlagLightFlicker` | `0x0080` | 96 | Add positional light jitter |
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

### 2. Attack-pose-only lighting (`0x0100`)

Original behavior:

- This modifier applies to light-emitting mobiles.
- A flagged mobile casts light only when `state < 32` and `state % 4 == 3`,
  which identifies its attack poses.
- World pictures are unaffected by this modifier.

Relevant original code:

- [`GameWin_cl.cp` mobile lighting condition](https://github.com/YappyGM/ClanLordClient/blob/6ba334cfb3fb779ecfe37e0b635fac476cb73a5e/mac_client/client/source/GameWin_cl.cp#L10022-L10034)

Current goThoom behavior:

- `drawMobile` calls `addLightSource` without passing the mobile state.
- `addLightSource` checks `EmitsLight` and `LightDarkcaster`, but not
  `OnlyAttackPosesLit`.
- Flagged mobiles therefore remain lit in every pose.

Implementation notes:

- Pass the mobile state to the mobile-lighting path, or add a small helper that
  decides whether a mobile should emit light.
- Do not apply this condition to ordinary world-picture light sources.
- Keep the pose rule explicit and covered by table-driven tests.

Tests should cover attack states `3`, `7`, `11`, and so on, ordinary movement
states, and states at or above `32`.

### 3. Light flicker (`0x0080`)

Original behavior:

- Both picture and mobile light sources receive independent horizontal and
  vertical jitter.
- Each axis uses `GetRandom(3) - 1`, giving an offset of `-1`, `0`, or `+1`
  world pixel.
- The original comment says only position flicker is implemented; intensity
  does not change.

Relevant original code:

- [`GameWin_cl.cp` picture-light jitter](https://github.com/YappyGM/ClanLordClient/blob/6ba334cfb3fb779ecfe37e0b635fac476cb73a5e/mac_client/client/source/GameWin_cl.cp#L9882-L9890)
- [`GameWin_cl.cp` mobile-light jitter](https://github.com/YappyGM/ClanLordClient/blob/6ba334cfb3fb779ecfe37e0b635fac476cb73a5e/mac_client/client/source/GameWin_cl.cp#L10071-L10079)

Current goThoom behavior:

- The flag is defined but `addLightSource` always uses the unmodified picture
  or mobile position.

Implementation notes:

- Apply the jitter before scaling world coordinates to display pixels.
- Keep it stable within one logical game frame so high-refresh-rate rendering
  does not make the light vibrate faster than the game updates.
- Use repeatable values during movie playback and seeking, following the same
  strategy selected for random animation.

Tests should confirm the allowed offset range, stability within a logical
frame, and no movement when the flag is absent.

### 4. Explicit shadow pictures (`0x1000`)

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

- `NightInfo.Shadows` is calculated, including `kLightNoShadows`, but is not
  used by world-picture drawing.
- Shadow pictures are rendered like ordinary pictures regardless of area or
  night state.

Implementation notes:

- Add exported constants for `PictDefIsShadow` and the other missing flags.
- In `drawPicture`, skip explicit shadow pictures when the current shadow level
  is zero.
- Apply the night alpha override without changing the cached image, because the
  same asset can be reused under different lighting state.
- Confirm how the existing user night-limit setting should map to the original
  effective-night-limit condition.

Tests should cover shadows enabled, `kLightNoShadows`, daylight, and night
levels on both sides of the 33 threshold.

### 5. Generated mobile shadows (`0x0800`)

This is larger than a single flag check because goThoom currently has no
game-world mobile shadow renderer.

Original behavior:

- Mobiles normally cast a drop shadow made from their current pose.
- An `UprightShadow` mobile instead casts a rotated shadow using a pose chosen
  from its facing direction and the sun angle.
- Dead and lying upright mobiles do not cast the rotated shadow. Other special
  poses use their current pose.
- Shadow opacity comes from the area's shadow level, and all generated shadows
  are disabled by the area's no-shadow flag.

Relevant original code:

- [`GameWin_cl.cp` mobile shadow choice](https://github.com/YappyGM/ClanLordClient/blob/6ba334cfb3fb779ecfe37e0b635fac476cb73a5e/mac_client/client/source/GameWin_cl.cp#L3404-L3475)
- [`Shadows_cl.cp` pose selection](https://github.com/YappyGM/ClanLordClient/blob/6ba334cfb3fb779ecfe37e0b635fac476cb73a5e/mac_client/client/source/Shadows_cl.cp#L599-L631)

Suggested staging:

1. Add simple drop shadows for ordinary mobiles.
2. Respect the area shadow level and no-shadow flag.
3. Add upright pose selection based on facing direction and sun angle.
4. Add the rotated/projected upright-shadow rendering.

Keep shadow images out of the normal sprite caches unless the cache key also
contains every input that affects their shape. A small per-frame rendering path
may be simpler initially.

Tests should cover ordinary and upright mobiles, all eight directions, changing
sun angle, prone/dead poses, and an area with shadows disabled.

## Closely related lighting adjustments

These are not additional `PictDef` flags, but they are part of the same original
lighting-data behavior.

### Default radii differ from the original

When `LightingData.ilRadius` is zero, the original uses different defaults:

- World picture: frame width plus frame height.
- Mobile: twice the width of one mobile pose.
- Darkcaster: multiply the resulting radius by four.
- Dead light-emitting mobile: halve radius and intensity.

goThoom currently uses the picture width or one mobile-pose width, does not
apply the darkcaster radius multiplier, and does not reduce dead-mobile light.
These differences should be reviewed visually before changing them because the
current shader has its own global radius scaling.

## Recommended implementation order

1. Random animation.
2. Attack-pose-only lighting.
3. Light-position flicker.
4. Explicit shadow-picture suppression and night blending.
5. Radius parity review.
6. Generated mobile shadows and upright-shadow projection.

The first four items are contained changes with direct asset coverage. Generated
mobile shadows are a separate rendering feature and should not block the other
flag fixes.

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
  lightning, attack-only light emitters, flickering flames, explicit shadow
  pictures, upright exiles, and non-upright creatures.
