# Multi-session client design

## Product model

goThoom runs one application with up to ten independent character sessions.
The game window contains one row of session tabs and one shared playfield. One
tab is always open.

- **+** opens and selects the first available stable session slot.
- A tab **X** confirms, disconnects, and closes that slot.
- The active tab is selected with the mouse, the default customizable Ctrl-1
  through Ctrl-9 and Ctrl-0 hotkeys, or customizable forward/backward cycling
  hotkeys.
- Hotkeys address the visible tab order. Internal session IDs remain stable for
  ownership, logging, and persistence.
- `multi_session.json` stores open slots and the active slot.

## Ownership

Each `Session` owns its connection, reconnect supervisor, decoded world,
character identity, inventory, player directory, command queue, scripts,
macros, recording, text log, lighting inputs, notification state, and assembled
bard-music timeline.

The application owns shared settings, assets, sprite caches, shaders, audio
devices, utility windows, and the physical game window. Shared Inventory,
Players, Scripts, Chat, and Console controls bind to the active session.

The `viewportManager` retains per-session draw snapshots and lighting state,
but marks only the active session viewport renderable. The active state borrows
the one game-window image and backing allocation. Inactive states own no game
window or target image.

## Rendering

Every draw performs one ordinary scene pipeline for the active session:

1. Capture or reuse that session's draw snapshot.
2. Draw its scene into the shared game image.
3. Apply its lighting and shadow pipeline.
4. Draw status, speech, script, recording, and diagnostic overlays.

There is no multi-viewport atlas, tiled session grid, secondary game window,
or copy from per-session completed frames. Background sessions continue model
updates without issuing scene, lighting, or presentation work.

This preserves single-view GPU behavior while avoiding the render-target and
shader-context churn caused by drawing several session views in one frame.

## Audio

Only the active session may queue game sound effects or synthesize bard music.
Inactive sessions still assemble complete music jobs and timestamp their starts
using wall time. On a tab switch, the shared music player is stopped and any
still-active jobs from the new session are rebuilt at their elapsed sample
offset, using the same start-frame synthesizer path used for CLMov seeking.

## Input and lifecycle

Pointer, keyboard, command, hotkey, toolbar, and shared-window actions resolve
the active `Session` before dispatch. Closing a tab cancels its reconnect
supervisor and disconnects its transport. Application shutdown cancels and
joins all open sessions.

Fake, PCAP, CLMov, and setup-preview modes keep their specialized single-view
surfaces and do not display live-session tabs.

## Validation

- Unit tests cover the ten-tab cap, sparse slot reuse, close-last rejection,
  visible-order shortcuts, workspace migration, one shared render surface,
  session-local audio routing, and wall-time music offsets.
- The full Go suite covers connection, script, recording, persistence, input,
  and rendering isolation.
- Live testing should compare one active tab with the same scene in a separate
  single client, especially with shader lighting and sun shadows enabled.
