# Settings coverage and hidden options

goThoom stores persistent settings in the categorized version 4
`settings.json` file in the user data folder. Most user preferences have a
control in Settings, Mixer, Notifications, Speech
Bubbles, Controller, or Window Layout. This document lists the exceptions.

`multi_session.json` stores the open session tabs and the selected tab. The
normal Window Layout preference positions the single shared game view and the
other application windows.

## Settings navigation

Settings and configuration controls with hover help display a small circled
“i”. Hover over the control to read its tooltip.

The Settings window has twelve topic tabs:

| Tab | Controls |
| --- | --- |
| Display | Window layout, UI scale, fullscreen, themes, and appearance |
| World | Status bars, visibility, character names, and player-list grouping |
| Text | Font sizes, chat timestamps, timestamp format, and text colors |
| Bubbles | Speech bubble appearance, lifetime, and message-type options |
| Audio | Music SoundFont and buffering, sound throttling/staggering/resampling, and notification preferences |
| TTS | Speech enablement, voice downloads, voice and speed, test phrase, and corrections |
| Controls | Movement behavior, keyboard walk speed, and gamepad |
| Performance | Quality preset plus artwork, effects, rendering, caching, and power-saving subtabs |
| Experimental | Replacement-effect selection and previews, mobile light-cone shadows, and HD sprite packs |
| Network | Server address and NLSPT safety margin |
| Files | File paths, downloaded assets, user data and diagnostics folders, and recording |
| Tools | Setup wizard, debug settings, and resetting all preferences |

Settings uses a fixed 730 × 700 logical-pixel window (scaled with the UI and
clamped to smaller screens). Switching tabs or Performance subtabs keeps the
window size stable. Main Settings tabs wrap into balanced rows, while Performance
subtabs use a single row.
Detailed editors such as File Paths still open separately.

**Text → Editor Text Size** sets the text size for all file and personal note
editors (8–48, default 11). The editor’s gear shortcut opens this page.
**Ctrl+- / Ctrl++** changes the same saved preference. **Ctrl+=** works without Shift; keypad +/−
works too. On Mac, Command is also supported. **Ctrl+scroll up/down** over editor
text changes its size without scrolling. Text, selection, and undo history stay
intact. The preference is stored as `interface.editor_font_size`.

**Word wrap** in the file editor toolbar fits long lines to the window without
inserting line breaks into the file. It starts enabled and applies to all file
editors, including tunes and personal notes. Turning it off restores horizontal
scrolling. The preference is stored as `interface.editor_word_wrap`.

**Text → Editor Colors** opens syntax colors for macro, Go script, TTS, and
theme/style editors. **Use custom syntax colors** switches between the active theme and a
shared custom palette. The choice is saved as
`interface.editor_use_custom_colors` (default `false`), with custom RGBA colors
in `interface.editor_syntax_colors`. Turning the checkbox off retains the custom
palette. **Copy Theme Colors** replaces it with the active theme's syntax colors.


**Spellcheck** underlines misspelled words in message input. Right-click an
underlined word to choose a correction; moving away dismisses the suggestions.

**UI Scale** uses 0.1 steps, with 0.75 retained as the minimum compact setting.
Automatic Retina and HiDPI display scaling is applied on top of that preference.

Toolbar controls have one home: **Settings → Display → Show / Hide Windows** for window
visibility and reset, **Actions** for Hotkeys/Keybindings, **Audio** for
notification sound and audio enhancement, **Tools** for Stats, command
search, Personal Notes, Help, and snapshots, and **Record** for session recording. Stats also contains NLSPT enablement
and live network timing.
TTS has its own **Settings → TTS** tab with enablement, file downloads, voice
and speed controls, a test phrase, and pronunciation corrections. **Open Voices
Folder** creates and opens the active `piper/voices` directory. **Browse More
Voices** opens the Piper voice catalog; install both the `.onnx` model and its
matching `.onnx.json` configuration file. The **Spoken
Messages** section separately controls speech, whispers, yells, thoughts,
actions, ponders, monster speech, and whether the current character's own
messages or in-game notifications are read aloud. Settings omits other duplicate launchers
and controls. The command palette continues
to provide searchable access to settings and actions.

Chat, Console, Inventory, and Players omit title-bar close buttons. In floating
mode, open or close them with the **Windows** toolbar selector or the matching
**Show / Hide Windows** controls in Settings. In tiled mode, **Windows** opens
the layout editor directly.

**Display → Windows & Toolbar → Window Layout** shows a clickable workspace
preview. **Start with** offers centered, side, message-row, and side-column
arrangements. Turn off **Combine chat + console** to place the message panes
separately. **Move selected pane** offers Swap, Left of, Right of, Above, and
Below: choose the operation, click a source pane, then click its target to apply
the change immediately. Clicking the selected pane again cancels the selection.
**Undo** reverses moves, starting-arrangement choices, and Even splits in the
editor. **Even splits** rebalances pane sizes while allowing extra space for the
game and toolbar; it does not give every pane identical dimensions. The setup
wizard provides the same editor.

Custom arrangements are saved automatically as `windows.custom_tiled_layout`
and shown as **Current arrangement** in Start with. They use manual sizing.
Combining Chat and Console hides Chat without discarding its saved position;
separating them restores it. Drag actual workspace dividers to adjust row heights, message splits,
and list widths. **Messages left, lists right** starts with a full-height left
message column and stacked lists on the right; the two height splits are independent.
Turn off **Use tiled window layout** to move and resize the main windows freely,
and use **Snap floating windows** to align their edges.

For supported starting arrangements, **Auto-size side panels**
adjusts the side panel widths to use empty space beside the game, sizing the game
column for the playfield proportions and available height. Dragging either
vertical game divider moves the game sideways and adjusts both side panels.
Turning it off keeps the current size and unlocks the dividers for resizing.
Turning it back on fits the game around its current position, within the list
width limits. This option is unavailable for custom arrangements and Game on a side.

The **Performance** tab contains the quality preset and five subtabs:

| Tab | Controls |
| --- | --- |
| Artwork | Scale override, upscale style, pixel alignment, foreground fading, gamma, and dither cleanup |
| Motion | Movement smoothing, subpixel movement, and character/world animation blending |
| Lighting & Effects | Lighting and window/character shadows |
| Caching | Batch room artwork loading, sprite cache, sound precaching, activity dots, and GPU compatibility |
| Power Saving | Background/focused power saving, power-saving FPS, VSync, and the separate 250 FPS limit |

### Color theme

**Display → Appearance → Color Theme** defaults to **Follow system**, using
AccentLight for light mode and AccentDark for dark mode. The client checks at
startup and every 10 seconds, updating the palette only when the system
appearance changes. Automatic switches keep your style and accent color.
Choose a named palette to keep it fixed. If system appearance is unavailable,
Follow system uses AccentDark.

**Edit Color Theme** and **Edit Style Theme** open the active theme's JSON.
Built-in files get a user copy with the same name in `themes/palettes/` or
`themes/styles/`. **Check** validates without applying changes; **Format**
indents the JSON. Valid JSON also formats on open and save, with opening changes
kept in the draft. **Save** writes the file, and **Save & Apply** selects it.
Applying a color theme selects that named palette instead of Follow system.
Hex values appear as clickable color swatches. The picker changes the draft as
one undoable edit; **Save & Apply** activates it. Selecting a value or moving the
caret into it reveals its hex code. Copying still copies the underlying JSON.

### Notes and text files

**Tools → Personal Notes** stores local notes with subjects and comma-separated
tags. Choose **Global note** to make a note available with every character,
or assign it to one player. The library filters by player and scope, and its
titlebar search matches subjects and tags. **Details** edits that metadata;
**Open** edits the body. Notes preserve whitespace and require **Save**.
Trashed notes remain in the user data folder's `Notes/Trash/` directory.

**TTS → Edit TTS corrections** opens the substitutions file in the client.
Use one `original=replacement` per line and `#` at the start of comment lines.
**Check** validates it; **Save & Apply** saves and uses it for future speech.

Notes, Go scripts, TTS substitutions, and theme/style files require UTF-8.
Existing UTF-8 BOMs and newline styles are preserved. Only legacy macro files
support MacRoman. File editors warn before discarding unsaved body/source edits
and refuse to overwrite files changed externally since opening or saving.

### Smooth nametag motion

**World → Character Names → Smooth nametag motion** defaults off for crisp,
pixel-aligned labels. Enable it to let names and their health bars use fractional
positions and smooth edges as characters move. Text may look slightly softer. The option
uses the existing movement interpolation and does not enable Motion Smoothing
if that setting is off.

The preference is saved as `interface.smooth_nametag_motion`.

### Alternating row colors

**Display → Appearance → Alternating row colors** has independent Inventory,
Chat, Console, and Players checkboxes. Inventory defaults on; the others default
off. Search matches and selected rows retain their highlights.

The four preferences are saved under `interface` as
`inventory_alternating_row_colors`, `chat_alternating_row_colors`,
`console_alternating_row_colors`, and `players_alternating_row_colors`.
Older `alternate_row_backgrounds` preferences migrate to Inventory only; the
other windows use their new defaults until explicitly changed.

### Emoji names

**Text → Chat & Messages → Show :smile: as emoji** defaults off. Turn it on to
display names such as `:smile:` and `:thumbs_up:` as emoji in chat and speech
bubbles. The choice updates displayed messages immediately. Literal emoji still
display as emoji, and outgoing messages continue to use readable names on the
wire.
The preference is saved as `chat.expand_emoji_names`.

Each chat input bar has a command icon that lists input-bar shortcuts plus
available client, script, macro, and server commands by source, with a short
explanation of each. The window widens and grows to fit the display; long
shortcut help wraps instead of being clipped. Choose a command to start it in
the draft. Commands show their argument syntax in dimmed text, and the same
guide appears in the input bar after an exact command name. It is not inserted
when you press Tab.
When emoji display is enabled, its picker appears beside the command icon. The
picker keeps group names on the left while emoji scroll on the right. Search
finds emoji across groups. Choosing one appends its shortcode to the draft and
closes the picker; press Enter in the input bar when you are ready to send the
message.

## Persistent settings coverage

The v4 JSON schema covers persistent preferences from the internal `settings`
structure:

- Most fields map directly to categorized JSON values.
- `BarPlacement`, `BarStyle`, and `SpriteUpscaleMode` are persisted separately
  as readable string values.
- `SpriteUpscale` and `SpriteUpscaleFilter` are derived rather than persisted.
- `Version` is document metadata at the JSON root.

The schema regression test checks that every exported field is accounted for.
Debug/session controls are listed below. NLSPT safety is separate atomic session
state rather than a field in `settings`; it is listed as well.

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

### Status bars

**Settings → World → Status Bars** offers Regular, Modern -- thin, and Hidden styles.
Modern -- thin uses thin fills, minimal frames, and tight spacing near the selected
screen edge. Grouped placements share one frame, while Along Bottom keeps the
three frames separate. It is the default for new settings and in the setup wizard.
The saved settings are `interface.status_bar_style`, with values `regular`,
`compact`, or `hidden`.

### Window layout

**Snap floating windows** is available under **Settings → Display → Window Layout**.
It aligns floating windows with nearby window and screen edges while they are
moved or resized, and is saved as `windows.snapping`.

## Persistent options without a dedicated control

These options are saved in `settings.json` and available through `/setting`,
but do not currently have a dedicated checkbox, slider, or input in a settings
window.

| JSON setting | Default | Purpose and use |
| --- | --- | --- |
| `rendering.pin_world_objects` | `true` | Lets smooth movement recognize small moving pictures attached to a mobile, such as chains and effects, and interpolate them with that mobile. It only has an effect while `rendering.smooth_movement` is enabled. |
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

### Bard part receiving

**Bard → Duet / Trio → Receive parts from partners** defaults off. It accepts
private music parts only from full names in **Play with**, saving and selecting
them without starting playback. Each session has its own partner list and
receiving choice, which stay active when switching tabs. Reconnecting that
character or closing Bard turns receiving off. See the [bard guide](Bard.md) for
sharing and playback.

### Network Latency & Server Phase Timing (NLSPT)

Network Latency & Server Phase Timing is enabled by default. Its persisted
setting is `general.nlspt_enabled`; toggle it in **Stats → Enable NLSPT**. The
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

**Experimental → View HD Sprite Replacements** opens a scrolling gallery of
installed replacements. Choose one ZIP from the Sprite pack menu, or choose
Loose files to browse PNGs stored directly in the `hdimg` folders. Choose HD,
original, or side-by-side display, and use the zoom slider to resize the
thumbnail grid. Use the checkbox on each card to decide whether the game uses
that picture ID.
**Reload HD Sprites** rescans installed packs. **View Experimental Effects**
provides a scrolling original/new comparison gallery with resizable thumbnails, an animation-
rate control, shader reload, and per-effect switches. Display and zoom controls
affect only the previews; card checkboxes select the replacements used by the game. See the
[artwork authoring guide](../website/help/artwork.html) for these workflows.

### Scripts

**Actions → Scripts → Auto-kill spammy scripts** controls whether scripts that
spam output are stopped automatically. It is enabled by default and saved as
`scripts.stop_spamming_scripts`.

### Diagnostic windows

**Stats → Memory Use → Cache Statistics** shows live image, sound, sprite-slot,
and render-pool cache usage and provides **Clear All Caches**.

**Actions → Scripts → Script Events** opens the script callback event log.
Enable **Record script events** (`scriptEventDebug`, off by default) to capture
activity. Recording continues while the event window is closed and resets to
off when goThoom exits.

## Artwork scale

**Settings → Performance → Artwork → Artwork scale override** selects a fixed 2x, 3x, or 4x
artwork texture scale. The default is 2x. Resizing the game window and changing
quality presets preserve this override; existing saved values are retained.
The setup wizard offers the same control.

Suggested starting points are 2x for screens up to 1080p, 3x for 1440p, and 4x
for 4K or large game views. Smaller game windows may look just as good at 2x.
Higher scales use more GPU memory. These are guidelines, not automatic rules.

## Sprite packs and replacement effects

**Settings → Experimental → Use sprite pack files** defaults off and
is saved as `rendering.use_sprite_pack_files`. It loads static-picture PNG
replacements from `data/hdimg` in the game directory and `hdimg` in the user
data directory. Subfolders and ZIP files are supported. These are external
runtime files, not embedded assets or automatic downloads. The Assets & Audio
override in File Paths does not relocate the user-data `hdimg` folder.

Use the numeric picture ID as the PNG filename. Game-folder copies take
priority over user-folder copies. Within either folder, a root PNG wins over
a subfolder PNG, then a ZIP entry; ties use alphabetical path order. Replacements
keep the original picture rectangle and placement. Animated pictures and
mobile pose sheets are not replaced by this loader.

Use **Settings → Experimental → View HD Sprite Replacements** to compare the
installed art with the originals and enable or disable individual pictures.
Its Sprite pack menu browses each ZIP independently and groups PNGs outside
ZIPs under Loose files.
Use **Reload HD Sprites** after editing pack files; the Console reports the
discovered files and source folders.

**Experimental → Replacement Effects** separately enables procedural shaders
for supported effects and scenery. It also defaults off and does not need a
sprite pack. **View Experimental Effects** opens a scrolling preview gallery;
each card's checkbox enables that effect family.
Quality presets preserve these choices and the artwork scale override. See the
[performance and visuals guide](../website/help/performance.html) for installation,
comparisons, reflections, and troubleshooting.

## Frame-rate controls

**Performance → Power Saving** contains three independent limits:

- **VSync** follows the display refresh rate.
- **Power-save FPS** accepts 1–250 FPS and applies while **Always power save**
  is on, or while **Power save in background** is on and the client lacks focus.
- **Limit to 250 FPS** defaults on and applies when VSync is off and no
  power-saving cap is active.

The power-saving interval includes rendering and presentation time. Quality
presets preserve these choices; check them before treating a steady frame-rate
cap as a graphics-performance problem.

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

Settings → Performance → Caching → Sprite cache offers named presets. The
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
