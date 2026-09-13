# Multi-session guide

Use **Multi-session** on the toolbar after startup and asset checks finish.
goThoom opens four session views. A disconnected view uses the same saved
character list, Add/Edit/Delete actions, server list, password prompt, and
connection controls as the normal Login window, bound to that session slot.
Each slot can therefore connect or disconnect independently.

Click a view to make that session active. Keyboard input, chat submission,
movement, commands, hotkeys, toolbar actions, Inventory, and Players target the
selected session. Its title includes **Selected** and its title bar uses the
current theme accent; in the tiled layout its border uses the accent as well.

## Layout

The existing **Settings → Display → Window Layout** choice also controls the
session views:

- **Freeform** keeps four titled views that can be moved and resized
  independently.
- **Tiled 2×2** fits all four views inside the area normally occupied by the
  game window. Shared Inventory, Players, Chat, and Console panes keep their
  existing placement.

Turning off **Use tiled window layout** restores the four saved Freeform view
rectangles. After multi-session has been used, `multi_session.json` in the user
data folder saves those rectangles, the selected session, and the music source.
The normal single-session window layout remains in `settings.json` and is
restored when multi-session closes.

## Background sessions and messages

Every connected session continues receiving network updates and running its
own macros and Go scripts while another session is selected. A globally enabled
Go script gets an independent interpreter in every connected session;
character-enabled scripts run only for matching characters. Script callbacks,
commands, movement, windows, output, and cleanup stay with the session that
started them. Sound effects and notifications from all sessions can play
through the shared mixer. Direct user input always has one target: the selected
session.

Chat and Console combine session messages in their shared transcripts and
identify the originating character. Inventory and Players show only the
selected session and rebind together when selection changes.

## Bard music

Only one session supplies bard music to the shared player. Choose it in Mixer
or **Settings → Audio**. Changing the source stops the current tune; playback
waits for the next normal music event from the newly selected source rather
than resuming or synchronizing an earlier tune.

## Leaving multi-session

Use the small logout icon in the bottom-right corner of a connected session
view to disconnect that character. After every session is fully disconnected,
use **Sessions** on the toolbar to return to single-session mode. This restores
the ordinary game-window layout; opening multi-session later restores the saved
workspace.
