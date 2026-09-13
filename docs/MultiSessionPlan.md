# Multi-session client plan

## Goal

Allow one goThoom process to maintain multiple independently logged-in game
sessions.  Sessions share application settings, artwork/GPU resources, the
audio device, and the primary UI, while each character keeps independent game,
network, input, and automation state.

The user can work with the selected session directly while retaining live views
of the other sessions.

## Decisions already made

| Area | Decision |
| --- | --- |
| Settings | One application-wide settings object.  Character profiles do not switch rendering, UI, audio, or input settings when sessions change. |
| Rendering resources | Images, textures, shaders, fonts, artwork caches, and the Ebitengine process remain shared. |
| Sound effects | Use the common mixer; effects from every connected session may play. |
| Music | Exactly one selected session feeds the shared music player.  Mixer/Settings shows four mutually exclusive session selectors; changing selection stops current music without resuming or synchronizing a tune. |
| Macros and scripts | Each session has an independent legacy macro runtime and Go-script engine.  No runtime state is shared between characters. |
| Chat and console | Display one combined transcript.  Every entry visibly identifies its source character, with optional character tinting. |
| Input target | The selected session view is the sole target for keyboard input, chat submission, movement, commands, hotkeys, toolbar actions, and shared panels. |
| Shared panels | Chat/console, Players, and Inventory remain one set of UI windows and rebind to the selected session.  Their title/input border visibly show the selected character. |
| Freeform workspace | Multi-session mode opens four movable/resizable game windows, one session slot per window. |
| Tiled workspace | Subdivide the existing game-window area into a 2x2 grid of session slots.  Occupied slots show sessions; unused slots show an add-character/login surface. |
| Session activation | Start every application in single-session mode.  Additional sessions become available only after the client is ready for sessions. |
| Mode control | A toolbar control switches between single-session and multi-session workspaces.  Entering multi-session mode creates four login-ready session slots. |
| Leaving multi-session | The workspace cannot return to single-session while any session is logged in.  The user must log out of every session or quit the application. |
| Quit all sessions | Provide a toolbar action that logs out/closes every session while leaving the application running.  The user may instead quit the application normally. |
| Notifications | Deliver all normal notifications from every session, visibly labeled with their source character. |
| Background automation | Background sessions continue normal network, macro, and script processing.  Only direct user input is restricted to the selected session. |
| Multi-session music default | The user chooses one session as the music source.  Default to the lowest session ID. |
| Prototype modes | PCAP support may be disabled or removed.  Fake mode does not need multi-session support.  Setup wizard runs before the app becomes ready for sessions. |

## Vocabulary and ownership

Do not use `viewport` as the owner of protocol state.  A viewport is a visual
and input surface; a session is the character connection and its mutable game
state.  The initial UI attaches one session to one viewport, but this
separation allows a replay/spectator view later without another connection.

```text
App
|- shared settings, assets, GPU caches, fonts, audio device, global UI theme
|- audio policy: common effects; one music-source session
|- combined console/chat transcript
|- Session A: connection, decoded state, input, players, inventory, automation
|- Session B: connection, decoded state, input, players, inventory, automation
`- Viewport manager
   |- viewport A -> Session A
   `- viewport B -> Session B
```

### App-owned state

- `gs` and settings persistence, themes/styles, keybinding definitions, and
  general UI configuration.
- CL image/sound data, decoded-image and GPU caches, shader instances, fonts,
  and shared render pools.
- The Ebitengine `Game`/application loop, shared EUI windows, and the viewport
  manager.
- The shared sound device/mixer, one active or pinned music source, and the
  combined log display.
- Cross-session layout persistence: freeform viewport geometry or tiled slot
  order, selected viewport, and music-source pin.

### Session-owned state

- Login state, credentials reference, TCP and UDP connections, cancellation,
  read/dispatch/send goroutines, reconnect status, and per-session protocol
  encryption/transport state.
- Character identity, game mode, commands, input queue, mouse/key walking
  state, server frame timing, packet-loss/PNA state, and recording state.
- Draw state, locking, snapshots/interpolation, bubbles, light/night state,
  health/stamina/balance, and per-session world render generation.
- Player directory/presence, inventory, selected inventory/player rows,
  `/be-who` and info queues, chat history, and unread/event counters.
- Legacy macro sources/runtime and Go script interpreters, event queues,
  timers, stores, registrations, and lifecycle callbacks.
- Parsed music timeline/current tune metadata.  This is data only; playback is
  selected by the app-level music policy.

### View-owned state

- A stable ID, assigned session (or empty slot), selected state, tab/tile
  label, and accent color.
- Freeform EUI game window or tiled rectangle, render target/image, and its
  `Game` render snapshot/cache.
- Pointer hit testing and conversion from screen coordinates to that view's
  world coordinates.
- A non-movable, view-centered login/connecting/reconnecting overlay for
  an empty or disconnected slot.

## Phase 0 state inventory

This table is the extraction checklist for mutable runtime state. Package-level
constants, immutable lookup tables, shared decode scratch pools, and test-only
dependency hooks are not session state.

| Current area | Representative state | Target owner | Extraction notes |
| --- | --- | --- | --- |
| Settings and persistence | `gs`, settings/profile files, theme and keybinding definitions | App | Character profiles remain login metadata; they must not replace app settings when session selection changes. |
| Assets and rendering resources | CL archives, decoded images/sounds, shaders, fonts, sprite/name-tag caches, render pools | App | Shared by every viewport; cache keys must not contain implicit current-character state. |
| Audio output | audio context, mixer levels, sound/TTS players | App | Decoded sound requests now carry a source session ID and use per-session deduplication before entering the shared mixer. Notifications and music-source routing remain to move. |
| Game view | `gameWin`, `gameImage*`, `worldViewRect`, hover/render caches | Viewport | One copy per session slot; pointer coordinates resolve the viewport before dispatch. |
| Draw model | decoded state, initial snapshot, bubbles, vitals, lighting flags, logical frame and world generation | Session | The canonical model is extracted behind a session lock. Protocol decode, render-cache preparation, snapshot capture, bubbles, sounds, inventory commands, and visible-player observations accept an explicit session. Interpolation settings and viewport caches remain to move. |
| Transport and login | TCP/UDP connections, login cancellation/progress, reconnect state | Session | Socket pairs, connection status, cancellation, generation-safe cleanup, network loops, immutable per-attempt login requests, and pending password updates are session-owned. The existing login-window fields remain a primary-session UI adapter; per-slot login surfaces, protocol encryption state, and reconnect policy remain to move. |
| Network timing | ack/resend values, frame statistics/timing, PNA controller/fallback, command reply samples | Session | Extracted behind session-owned locks and wake channels so one connection cannot tune or wake another. Existing live call sites still target the primary session. |
| Direct input and commands | mouse/key walking state, input queue, command number/pending command/queue/tickets, who/info queues | Session | Command streams, tickets, and background-session input queues are isolated. The visible UI still feeds the primary input adapter; viewport routing and who/info scheduling remain to move. |
| Character data | `playerName`, player index/directory, inventory model, selections and presence scans | Session | Inventory and decoder-observed player presence are session-owned. The full Players backend, identity, selection, `/be-who`, and info scheduling remain primary-session adapters. |
| Chat and logs | chat/console models, text-log path, local history and unread state | Session plus App aggregate | Decoded chat and console output is retained as source-tagged session events and copied to an app aggregate. The visible combined window, persistence, unread state, and script dispatch still use the primary adapter. |
| Automation | script engine/session snapshots/resources and legacy macro program/runtime | Session | Secondary sessions now load, advance, and tear down independent legacy macro runtimes. Their macro variables, text log, commands, and movement resolve through the owning session. Script candidates bind their Self, Players, Inventory, CurrentWorld, and LatestServerMessage APIs to one session before evaluation. Each secondary runtime owns its interpreter, serialized callback queue, chat/server/change/login/logout/stop subscriptions, and change/message snapshots. Every runtime queue uses its session-owned `Repeat`/`After` timers, tick waiters, and tasks, so disconnecting one session cancels only that runtime. Commands and input bindings, UI registrations, player-change subscriptions, inventory waiters, selected-row change sources, and the primary macro UI remain to move. |
| Music data | parsed tune queue/timeline and current tune metadata | Session | The synthesizer/player stays app-owned and follows the explicit music-source selection. |
| UI windows | Settings, shared Chat/Console, Players, Inventory, toolbar and dialogs | App | Shared panels bind to one session snapshot atomically; callbacks capture their originating session ID. |
| Recording/replay | recorder, movie state, seek/timeline state | Session or dedicated replay source | Live session recordings cannot share mutable buffers. Fake mode stays single-session and PCAP may remain unsupported. |

The first implemented boundaries are `SessionID`, the app-owned session
registry, Inventory, and the ordered command stream. IDs map permanently to the
four slots and do not depend on character names. The registry starts with the
primary session and materializes all four login-ready slots when multi-session
mode is enabled. Inventory contents, command numbering, pending sends,
acknowledgement/resend state, packet-loss and cadence measurements, command
reply timing, PNA learning/fallback, and script command-ticket cancellation are
independently locked and owned by each session. The decoded draw model, initial
snapshot, logical frame, world generation, draw packet frame tables, and render
snapshots are isolated as well. State-data parsing now routes inventory,
bubbles, sound deduplication, visible-player observations, and tagged
chat/console/audio events through the originating session. The app retains a
bounded combined event model, and sound events from every session enter the
shared mixer with their source ID. Existing entry points retain primary-session
wrappers so the visible one-client application remains unchanged during
extraction. Secondary BEPP/backend commands are retained as tagged events until
the player, automation, music, and notification backends become session-aware.
Each session now also owns its TCP/UDP socket pair, connection status,
cancellation boundary, transport generation, network dispatch loops, command
packet construction, a background-safe input queue, selected server and
credentials, and its pending password update. Login attempts capture immutable
request snapshots, demo-character retries update only their owning session,
and simultaneous session logins cannot exchange hosts, character names, or
passwords. Disconnect and stale-loop cleanup operate only on the owning
session. Secondary-session login now also loads a private legacy macro runtime;
its `@login` work advances with that session's server frames, queues only that
session's commands and movement, and is cancelled when the session resets.
Script data snapshots are now parameterized by session and candidate exports
bind to that session before source evaluation. Each session also owns a
serialized script-event-queue registry, so queues can run and be torn down
without crossing a second connection. Secondary sessions can now independently
start, stop, and reset an interpreter with isolated chat and lifecycle
subscriptions. Timer registries are also session-owned: wall-clock callbacks,
server-tick waits, task cleanup, and task command cancellation remain isolated
when another session disconnects. Commands and input bindings, UI
registrations, player-change subscriptions, inventory waiters, selected-row
change sources, and the primary Scripts UI remain to move.
The existing login-window fields now copy into the primary session at
connect time and remain only a UI adapter pending per-slot login surfaces.

## UI behavior

### Select and route

Selecting a viewport is one operation, completed before any resulting action
is dispatched:

```text
pointer press in viewport B or click Character-B tab
-> select Session B
-> rebind shared chat/console, Players, and Inventory views to B
-> update panel titles, input label, and accent border to B's color
-> route the input action (if it was a world press) only to B
```

A click in a world view both selects it and performs its normal world action,
unless an overlay or UI control consumed the press.  Keyboard and raw mouse
bindings go only to the selected session; background sessions still receive
their own network and script events.

### Combined chat and console

Keep a session-local log for scripts, macros, filtering, and persistence;
publish a copy to an app-level combined display model.  Each displayed entry
has session ID, character name, time, type, text, and an optional tint.

Use a clear source treatment rather than color-only identification:

- a character-name prefix on every entry;
- a subtle tinted left rail or background around each entry;
- a colored frame around the shared log and input area matching the selected
  session; and
- an input label such as `Hardia > Say something...`.

Ordinary chat and server commands entered there target the selected session.
Local client commands need an explicit app-level syntax or command palette
path so they cannot accidentally be sent to a server.

### Players and Inventory

The rows and selections are session-owned.  The EUI windows are shared views:
on every session selection, `BindSession(session)` replaces the rows from a
consistent session snapshot, restores that session's own selection if valid,
sets `Hardia - Inventory` / `Hardia - Players`, applies its accent, and redraws.

Context menus and delayed/double-click actions must capture the originating
session ID when opened.  They must not act on whichever session happens to be
selected later.

### Music

Default the music source to the lowest session ID.  Mixer/Settings presents
four session selectors (shown as checkboxes if that fits the existing control
style, but mutually exclusive in behavior).  Label each with its session or
character name and disable selectors for unused slots.

Switching the selected source immediately stops current music.  Do not resume,
seek, or synchronize the newly selected session's current tune; it may start
music only when it later receives a normal music event.  This intentionally
keeps handoff simple and prevents two songs from playing at once.

### Layouts

Freeform multi-session mode creates four titled game windows, one per session
slot.  These windows are movable/resizable; the other UI windows stay shared.

Tiled multi-session mode subdivides the existing game-window area into a 2x2
session grid; the surrounding shared Inventory, Players, and combined
Chat/Console windows retain their existing tiled placement.  The selected tile
has the accent outline.  Tiles retain the normal world aspect ratio; on a
smaller display the user accepts the reduced rendering size or uses a
higher-resolution display.

In every multi-session layout, each empty slot shows the normal login screen
as its entire session view.  It is not a floating dialog: login, connecting,
and reconnecting take over the full assigned session area until that session
is playing.  Entering multi-session mode creates all four empty/login-ready
slots at once.

## Ready-for-sessions gate

The app always starts with one session.  It becomes **ready for sessions** only
after startup loading, asset availability/compatibility checks, and the setup
wizard have completed.  Until then, the toolbar's multi-session control is
hidden or disabled and the startup/wizard/fake paths retain their current
single-session assumptions.

Passing this gate enables the toolbar's multi-session control.  Activating it
creates four slots and opens the normal, existing login flow inside each slot.
Login itself does not need a reduced or alternate form.

While any slot is logged in, switching back to the single-session workspace is
unavailable.  After every session has logged out, single-session mode may be
selected again.  The toolbar also provides **Quit All Sessions**, which closes
every session connection and returns all slots to their login screens without
quitting the application.

## Migration plan

### Phase 0: establish safety nets

1. Inventory all mutable globals and label each as app-, session-, or
   viewport-owned.  Include test hooks and state reset helpers.
2. Add focused tests for two independent state objects: draw decoding,
   inventory, players, command queues, and macro/script dispatch must not
   cross-contaminate.
3. Define a stable `SessionID`; never use display name alone as an internal
   identity.

### Phase 1: introduce `Session` without changing the visible UI

1. Create `Session` and move draw state, its mutex, snapshots, frame timing,
   and world-state generation into it.
2. Make protocol decode and dispatch methods take `*Session`; retain one
   default session and preserve the present one-client behavior.
3. Move connection lifecycle, input/commands, player/inventory/chat data, and
   night state into `Session` in small compilable slices.
4. Convert session reset functions to methods.  Do not keep a mutable
   package-global alias to the active state; temporary compatibility wrappers
   should be read-only or short-lived and removed before multi-session UI.

### Phase 2: isolate automation and audio data

1. Give every session its own script manager and legacy macro runtime; pass
   session context through all API calls and events.
2. Move music parsing/timeline ownership into the session and add the
   app-level music source selector.
3. Tag effect and notification requests with the session ID for diagnostics
   and future routing, while retaining the common mixer.

### Phase 3: add the session manager and shared-panel binding

1. Add an app-level session manager with create/select/disconnect/remove
   operations and a single authoritative selected-session ID.
2. Convert combined chat/console into an aggregate view over session-local
   entries.
3. Implement `BindSession` for Inventory and Players, including source-ID
   capture for callbacks/context menus.
4. Make titles, command targets, toolbar state, notification labels, and the
   native window title selected-session-aware.

### Phase 4: introduce viewports

1. Extract the present game image/window and per-render cache into a
   `Viewport` bound to one session.
2. Implement viewport hit testing, selection, coordinate conversion, and
   viewport-centered login overlays.
3. Implement four freeform session views first.  Verify that they render,
   accept only their own input, disconnect independently, and keep shared
   panels correctly bound.
4. Add the tiled 2x2 grid by subdividing the existing game-window area, then
   add layout persistence.  The four slots already provide the explicit
   initial maximum.

### Phase 5: harden and document

1. Exercise two live sessions plus a movie/replay if supported together.
2. Run race detection on session/model tests; check disconnect/reconnect,
   rapid selection changes, and callbacks queued after a session closes.
3. Document focus, music source, background macro behavior, combined-log
   filtering, and layout behavior for users.

## Non-negotiable invariants

- Every command, click, context-menu action, script API call, and macro action
  has exactly one target session.
- Every displayed session-originated event visibly identifies its session.
- No goroutine may mutate or read another session's data through an implicit
  global "current character".
- Closing a session cancels and joins its goroutines before its resources are
  removed; queued callbacks verify the session is still live.
- Shared settings and GPU/audio resources cannot make one session's state
  appear in another session's viewport.
- A selected-session swap rebinds all shared panels as one UI transaction;
  stale rows and callbacks cannot act on the new session.

## Resolved session-exit behavior

- Logging out of a session always returns that existing slot/window/tile to its
  normal full-area login screen.  It does not remove or rearrange slots.
- Multi-session mode cannot be left while any session is logged in.  The user
  must first log out of every session, use **Quit All Sessions**, or quit the
  application.
- **Quit All Sessions** is a toolbar action that cancels, disconnects, and
  joins all session goroutines, then returns every slot to login.  It does not
  exit the application.

The following are deliberately non-blocking for the first implementation:

- Concurrent sessions using different credentials/accounts and duplicate
  character selection need no special product handling beyond correct
  per-session isolation and normal server behavior.
- Existing layout/profile migration does not need new work.  Preserve current
  single-session behavior and save only new multi-session metadata when used.
- PCAP is outside multi-session scope and can be disabled or removed; fake
  mode remains single-session only.
- Music handoff intentionally stops playback and waits for a future normal
  music event from the newly selected source; it does not attempt seeking or
  synchronization.
- Tabbed session views are deferred and intentionally outside the first
  multi-session implementation.
- The user-facing term is **session**.  Internal rendering code may use a
  small view object where needed, but UI labels and documentation should say
  session.

## Existing-code implications

The current renderer has a useful `drawState` boundary and snapshot mechanism,
but it is package-global.  Login assigns the global character, switches the
global character profile, starts global automation, installs one `tcpConn`,
and starts loops that feed global protocol handling.  Those must become
session methods before a second connection can be correct.

Similarly, the current tiled workspace explicitly manages one `gameWin` plus
Inventory, Players, Console, and Chat.  Multi-session tiled mode needs an
intentional workspace redesign, not only a second render image.
