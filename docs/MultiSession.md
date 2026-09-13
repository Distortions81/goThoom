# Multi-session guide

goThoom always uses session tabs above one game view. The first tab is ready at
startup. Select **+** to open another independent login, up to ten tabs. A
disconnected tab shows the saved-character list, Add/Edit/Delete actions,
server list, password prompt, connection status, and errors inside the game
view.

Select a tab to make it active. Movement, keyboard input, chat, commands,
hotkeys, toolbar actions, Inventory, Players, Scripts, Chat, and Console all
target the active session. Use the logout icon in the lower-right of the game
view to disconnect that tab without closing it.

The default tab shortcuts are **Ctrl-1** through **Ctrl-9**, with **Ctrl-0** for
the tenth open tab. They select tabs by visible order, so gaps left by closed
sessions do not affect the shortcut number. Edit their keys or modifiers from
**Actions → Hotkeys**.

## Adding and closing tabs

The **+** button opens and selects the first available session slot. It is
disabled when ten tabs are open.

Each tab has an **X**. Closing a tab asks for confirmation and disconnects its
connection. At least one tab must remain open. Open tabs and the active tab are
restored on the next launch; connections still begin logged out.

## Background sessions

Only the active tab is rendered. Other sessions continue receiving network
updates and running their own macros, Go scripts, reconnect supervisors,
recordings, and text logs. A globally enabled Go script has an independent
runtime in every connected session; character-enabled scripts run only for
matching characters.

Chat and Console combine session messages in shared transcripts and identify
their source. Inventory, Players, and Scripts display the active session.

Only the active tab plays sound effects. Every session still tracks assembled
bard performances against wall time. When a different tab becomes active,
goThoom stops the previous audio and resumes any still-running performance from
the new tab at its current point.

## Window layout

There is one game window regardless of the number of sessions. **Settings →
Display → Window Layout** positions that game window together with Inventory,
Players, Chat, and Console. Changing tabs does not create, resize, or switch GPU
render targets for additional game windows.
