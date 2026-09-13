# Multi-session guide

Use **Multi-session** on the toolbar after startup and asset checks finish.
goThoom opens four session views. A disconnected view contains its own server,
saved-character selector, manual character and password fields, and connection
controls, so each slot can connect or disconnect independently.

Click a view or use **Select** in the Sessions window to make that session
active. Keyboard input, chat submission, movement, commands, hotkeys, toolbar
actions, Inventory, and Players target the selected session. Its title includes
**Selected**; in the tiled layout its border also uses the current theme accent.

## Layout

The Sessions window offers two layouts:

- **Freeform** keeps four titled views that can be moved and resized
  independently.
- **Tiled 2×2** fits all four views inside the area normally occupied by the
  game window. Shared Inventory, Players, Chat, and Console panes keep their
  existing placement.

Switching back to Freeform restores the four saved view rectangles. After
multi-session has been used, `multi_session.json` in the user data folder saves
the preferred layout, Freeform rectangles, selected session, and music source.
The normal single-session window layout remains in `settings.json` and is
restored when multi-session closes.

## Background sessions and messages

Every connected session continues receiving network updates and running its
own macros and Go scripts while another session is selected. Sound effects and
notifications from all sessions can play through the shared mixer. Direct user
input always has one target: the selected session.

Chat and Console combine session messages in their shared transcripts and
identify the originating character. Inventory and Players show only the
selected session and rebind together when selection changes.

## Bard music

Only one session supplies bard music to the shared player. Choose it in Mixer
or **Settings → Audio**. Changing the source stops the current tune; playback
waits for the next normal music event from the newly selected source rather
than resuming or synchronizing an earlier tune.

## Leaving multi-session

Use **Quit All Sessions** to disconnect every slot without closing goThoom.
**Return to Single Session** becomes available after every slot is fully
disconnected. Closing multi-session restores the ordinary game-window layout;
opening it later restores the saved multi-session workspace.
