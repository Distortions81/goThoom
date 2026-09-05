# Settings coverage and hidden options

goThoom stores persistent settings in the categorized version 4
`settings.json` file in the user data folder. Most user preferences have a
control in Settings, Quality, Mixer, Notifications, Speech
Bubbles, Controller, or Tiled Layout. This document lists the exceptions.

## Settings navigation

The Settings window replaces the former Advanced window with ten topic tabs:

| Tab | Controls |
| --- | --- |
| Display | Window layout, UI scale, fullscreen, themes, and appearance |
| World | Status bars, visibility, character names, and player-list grouping |
| Text | Font sizes, chat timestamps, timestamp format, and text colors |
| Bubbles | Speech bubble appearance, lifetime, and message-type options |
| Audio | Mixer, sound processing, text to speech, and notifications |
| Controls | Movement, keyboard bindings, hotkeys, gamepad, and script limits |
| Performance | Graphics quality, sprite cache, artwork loading, and power saving |
| Network | Server address, latency adjustment, and network timing |
| Files | File paths, downloaded assets, user data and diagnostics folders, and recording |
| Tools | Setup wizard, debug settings, and resetting all preferences |

Tabs use two rows so every category remains visible in a compact window.
Detailed editors such as Quality, File Paths, and Mixer still open separately.

## Audit summary

The current internal `settings` structure contains 180 exported fields:

- 175 are mapped directly by the v4 JSON schema.
- `BarPlacement` and `SpriteUpscaleMode` are persisted separately as readable
  string values.
- `SpriteUpscale` and `SpriteUpscaleFilter` are derived rather than persisted.
- `Version` is document metadata at the JSON root.

That accounts for every exported field. The same structure also contains nine
unexported debug/session fields, all listed below. NLSPT safety is separate atomic
session state rather than a field in `settings`; it is listed as well.

The safest way to inspect or change a persistent option is with the local
`/setting` command:

```text
/setting search <text>
/setting get <category.name>
/setting set <category.name> <value>
/setting reset <category.name>
```

Boolean values accept `true`, `false`, `on`, `off`, or `toggle`. Strings and
structured values use JSON syntax. If editing `settings.json` by hand, close
goThoom first so the running client does not overwrite the edit when it exits.

## Persistent options without a dedicated control

These options are saved in `settings.json` and available through `/setting`,
but do not currently have a dedicated checkbox, slider, or input in a settings
window.

| JSON setting | Default | Purpose and use |
| --- | --- | --- |
| `rendering.pin_world_objects` | `true` | Lets smooth movement recognize small moving pictures attached to a mobile, such as chains and effects, and interpolate them with that mobile. It only has an effect while `rendering.smooth_movement` is enabled. |
| `windows.snapping` | `false` | Makes independently positioned windows snap to nearby window and screen edges. Change it with `/setting set windows.snapping true` or `false`. |
| `windows.auto_resize` | `true` | Reapplies the managed window layout after the application size changes. Tiled mode always manages its layout regardless of this value. |
| `chat.text_to_speech_blocklist` | `[]` | Names whose messages should not be spoken. Prefer `/notts add <name>`, `/notts remove <name>`, and `/notts list`; those commands update the active list immediately. |
| `interface.show_clan_lord_splash` | `true` | Shows the classic Clan Lord splash artwork when it is available. |

`rendering.night_effect` is also present in the v4 JSON schema with a default
of `true`, but the current renderer does not read it. It is a dormant
compatibility key, not an effective user option. Night rendering is controlled
by the shader, lighting, and maximum-night-darkness options instead.

## Persisted state without preference controls

The following JSON values are saved so the client can restore its state. They
are normally changed by using the application rather than by editing a setting
control:

- `general.setup_wizard_version` and `general.last_character` track setup and
  login state.
- `updates.last_check` and `updates.last_notified_version` prevent redundant
  update checks and notifications.
- `windows.application_width`, `windows.application_height`, and the
  `windows.game`, `windows.inventory`, `windows.players`, `windows.messages`,
  `windows.chat`, `windows.movie`, and `windows.toolbar` objects remember
  application and window geometry.
- `windows.tiled_game_position`, `windows.tiled_left_bottom`,
  `windows.tiled_right_bottom`, `windows.tiled_left_width`,
  `windows.tiled_right_width`, `windows.tiled_side_game_width`, and
  `windows.tiled_side_top_split` are updated by dragging tiled-layout dividers.

These values are exposed by `/setting` because that command reflects the JSON
schema, but direct edits can produce awkward layouts. Use the window controls,
tiled dividers, or **Reset Windows** unless diagnosing a layout problem.

## File paths

**Settings → Files → File Paths** configures four global, restart-scoped
locations: assets and audio (including `CL_Images`, `CL_Sounds`, the SoundFont,
and Piper TTS files), diagnostic and text logs, legacy macros, and Go scripts.
An empty JSON value uses the standard location in the user data folder.

When **Copy existing files to the selected folder** is enabled, goThoom first
checks for conflicting files, copies without overwriting different destination
files, and verifies each copied file with SHA-256. It creates and reads back a
temporary probe in the destination before saving the path. If validation,
copying, checksum verification, or settings persistence fails, the filepath
setting remains unchanged. Restart goThoom after a successful change so open
logs and already-loaded assets keep a consistent location for the whole
session.

## Session-only controls not written to JSON

### Network Latency & Server Phase Timing (NLSPT)

Network Latency & Server Phase Timing is enabled by default. Its persisted
setting is `general.nlspt_enabled`. The
Settings → Network **NLSPT safety (%)** slider controls the internal
`networkAdjustmentSafetyPercent` value. It is deliberately session-only,
accepts 0–50%, and starts at 10% each time goThoom launches. The learned server
phase, lead, reply timing, RTT floor, jitter/loss samples, fallback state, and
cooldowns are also session measurements and are never written to
`settings.json`.

This prevents one server session's timing from becoming a stale or
self-amplifying input to the next session.

### Debug Settings

Every scene or diagnostic override below resets when goThoom exits:

| Debug control | Internal field | Startup value | Effect |
| --- | --- | --- | --- |
| Record script events | `scriptEventDebug` | Off | Captures script callback activity for the Debug window. |
| Record Asset Stats | `recordAssetStats` | Off | Writes image-count diagnostics to `stats.json`. |
| Hide Moving Objects | `hideMoving` | Off | Omits moving pictures, primarily for screenshots. |
| Hide Mobiles | `hideMobiles` | Off | Omits mobiles, primarily for screenshots. |
| Show image planes | `imgPlanesDebug` | Off | Draws sprite layer numbers. |
| Show picture IDs | `pictIDDebug` | Off | Draws picture IDs over sprites. |
| Force Night | `forceNightLevel` | Auto (`-1`) | Overrides the scene night level with Day, 25%, 50%, 75%, or Night. |
| Tint moving objects red | `smoothingDebug` | Off | Highlights moving pictures used by smoothing diagnostics. |
| Tint pictAgain blue | `pictAgainDebug` | Off | Highlights pictures carrying the `pictAgain` state. |

The Debug window may mark settings as dirty after these controls change, but
the v4 schema intentionally omits their internal fields, so saving another
setting does not persist them.

## Derived fields that are not independent JSON settings

The internal `SpriteUpscale` value is derived from
`rendering.artwork_scale`, and `SpriteUpscaleFilter` is derived from
`rendering.artwork_upscale_style`. They are implementation details rather than
additional preferences. Status-bar placement and artwork upscale style use
human-readable JSON strings even though their internal values are enums.

`source/settings_json.go` is the authoritative persistent schema.
`TestSettingsV4SchemaCoversPersistedFields` verifies that every exported field
in the internal settings structure is either represented by that schema or is
explicitly classified as version metadata, a human-readable special case, or
a derived field.

## Sprite cache

Settings → Performance → Sprite cache offers named presets. The
explanation below the selector updates immediately with the selected tradeoff
and the reserve at each sprite resolution. **Balanced** is the default.

| Preset | 2x reserve | 3x reserve | 4x reserve |
|---|---:|---:|---:|
| Minimal | 128 MiB | 288 MiB | 512 MiB |
| Compact | 256 MiB | 576 MiB | 1 GiB |
| Balanced | 512 MiB | 1,152 MiB | 2 GiB |
| Generous | 1 GiB | 2,304 MiB | 4 GiB |
| Maximum | 2 GiB | 4,608 MiB | 8 GiB |

The reserve scales with texture area: the 2x reference amount multiplied by
`effective_factor² / 4`, capped at 8 GiB. Startup allocation uses the effective
screen-capped artwork factor; subsequent uploads use their actual texture factor.
Restart after changing the preset to rebuild the preallocated reserve.

The persistent setting remains `performance.sprite_cache_mib`, now interpreted
as the reference amount at 2x. Numeric values that do not match a named preset
appear as **Custom** and are preserved until another preset is selected.

This is a soft allocation target, not a limit on total VRAM. The pool can grow
when the current scene needs more slots or different sizes. Source sheets, UI,
bubbles, render targets, and Ebitengine's atlas overhead are additional.
When no free slot fits, occupied slot area is compared with the scaled target to decide
whether to reclaim an old sprite. Idle preallocated space does not create eviction
pressure. Already-free slots remain available even if the live cache exceeds the target.
Old sprite IDs are reclaimed by last game frame seen, with all poses and recolor
masks invalidated together. Current and previous scene sprites stay pinned
through interpolation. Free larger slots can be reused up to twice the requested
area; evictions still require a matching size. The Debug window reports allocated
slot bytes split into live and spare space, slot reuse, first sprite IDs, and ID reloads after eviction. These
counters reset on cache clear and do not measure total GPU memory.

See [Sprite cache measurements](SpriteCacheStudy.md) for reload pressure across
the bundled movies and instructions for repeating the comparison.

The default leaves room for non-game textures. Bubble text and bodies recycle
managed allocations, retaining at most 16 MiB and 8 MiB of idle slots respectively.
Their active caches are limited to 32 MiB and 16 MiB, including slot padding.
Cached UI windows and color wheels share a pool with up to 64 MiB of idle slots;
open windows keep their active allocations. These reserves fill on demand.
Potato GPU compatibility mode continues to use standalone textures.

The Debug window shows bubble and UI pool sizes and reuse counts. The Stats
window's GPU memory figure uses Ebitengine's total image-memory counter, including
atlas space and render targets, rather than estimating from artwork dimensions.
