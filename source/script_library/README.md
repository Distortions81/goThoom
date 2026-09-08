# Using and writing goThoom scripts

Scripts add commands, hotkeys, notifications, automation, and small interface
tools to goThoom. They are ordinary Go files run by goThoom's restricted script
engine; you do not need to compile the client.

> The easiest way to find the active folder is **Actions -> Scripts -> Open
> scripts folder**. On macOS, use this button instead of the documentation copy
> beside the app.

## Install and use a script

1. Open **Actions -> Scripts**.
2. Choose **Examples** to install a bundled example, or place a `.go` file in
   the folder opened by **Open scripts folder**.
3. Select the script and enable it for **All** players or the selected **Player**.
   Review its permissions. Enabled scripts run after successful login.
4. Save your changes. goThoom notices file changes and reloads enabled scripts
   automatically. **Refresh** forces a rescan.

The first time `Scripts` in the user data folder has no script packages,
goThoom copies its embedded examples there. Your existing scripts are never
replaced.

The Scripts window shows load and runtime errors. If a reload fails, the last
working copy keeps running when possible.

For a separate VS Code project, download `goThoom-Script-Template.zip` from the
goThoom release. It includes a starter script, VS Code settings, and the same
`gt2` editor stubs and API reference as that client release.

Do not edit `go.mod`, `go.work`, or files under `gt2/`. goThoom manages those
files so editors can understand the scripting API. Your script files are never
rewritten.

## Create a script

Use **Actions -> Scripts -> New Script** for a working command, hotkey, chat
event, or equipment example. You can also create a `.go` file yourself:

```go
//go:build script

package main

import "gt2"

const scriptID = "my-hello-script"
const scriptName = "Hello Script"
const scriptAuthor = "Your Name"
const scriptCategory = "Utilities"
const scriptDescription = "Adds a /hello command and Ctrl-H hotkey."
const scriptAPIVersion = 2

func Init() {
	gt2.Command("hello", func(args string) {
		gt2.Print("Hello " + args)
	})

	gt2.Bind("Ctrl-H", func(event gt2.InputEvent) {
		event.Consume()
		gt2.Send("/think Hello!")
	})
}
```

Enable the script, then type `/hello world` or press Ctrl-H.

Every script should have:

- `//go:build script` as its first line.
- `package main`.
- An `Init()` function.
- A permanent, unique `scriptID`. Keep it unchanged after sharing the script;
  saved settings and storage use this ID.
- `scriptAPIVersion = 2`.

The other metadata fields are optional but make a shared script easier to
understand.

## Useful API calls

- `gt2.Print(text)` writes to the in-game console.
- `gt2.ShowNotification(text)` displays an on-screen notification.
- `gt2.Send(command)` sends an ordered, rate-limited game command.
- `gt2.Command(name, handler)` adds a local slash command.
- `gt2.Bind(keys, handler)` binds a key, click, chord, or mouse wheel action.
- `gt2.OnChat(filter, handler)` listens for matching chat.
- `gt2.OnServerMessage(filter, handler)` listens for server messages.
- `gt2.OnChange(kind, handler)` listens for inventory, equipment, vitals,
  selection, world, and location changes.
- `gt2.Self()`, `gt2.Players()`, and `gt2.Inventory()` return state snapshots.
- `gt2.CurrentWorld()` and `gt2.OnWorld(handler)` expose mobiles, scenery,
  sprite planes and sizes, your on-screen character, frame timing, lighting,
  and estimated camera motion.
- `gt2.Move(x, y)`, `gt2.StopMoving()`, and `gt2.Movement()` steer through the
  normal game input loop and report script/manual movement state.
- `gt2.Wait(...)` and `gt2.WaitTicks(...)` pause only the current script task.
- `gt2.Repeat(...)` runs a serialized callback repeatedly.
- `gt2.Store(...)` and the `gt2.Load*` functions keep private script data.

Open `gt2/API_REFERENCE.md` inside the active scripts folder for every type,
function, constant, and example supported by your installed goThoom version.

Scripts can import only `gt2` and these standard packages:

```text
bytes, encoding/json, errors, fmt, math, math/big, math/rand,
regexp, sort, strconv, strings, time, unicode/utf8
```

## Hotkeys

Combine modifiers and keys with hyphens. Names are case-insensitive:

```go
gt2.Bind("Ctrl-Shift-A", handler)
gt2.Bind("Shift-LeftClick", handler)
gt2.Bind("WheelUp", handler)
```

Common modifiers are `Ctrl`, `Shift`, `Alt`, and `Meta`. Mouse names include
`LeftClick`, `MiddleClick`, `RightClick`, `Mouse4`, and `Mouse5`. A handler can
call `event.Consume()` to prevent the same input from reaching the game.

Script hotkeys also appear in the Hotkeys window, where they can be enabled or
disabled.

## Script folders and ZIP packages

A script can be distributed as:

- One `.go` file.
- A folder containing exactly one `.go` file at its root plus assets in
  subfolders.
- A ZIP containing exactly one `.go` file at its root plus assets.

Folder and ZIP scripts can load package-relative assets and add toolbar
buttons. Asset paths cannot be absolute or escape the package. ZIP packages are
read directly and do not need to be extracted.

## Permissions

The first time a script is enabled, the client opens a permission review before
executing globals or `Init`. Requested capabilities start checked. Uncheck any
you do not want, then choose **Grant**, or choose **Block all** to deny every
capability and leave the script disabled. Closing the review leaves it waiting.
This also applies to bundled examples and previously installed scripts.

Use **Actions -> Scripts -> Info -> Permissions** to edit the decision later.
Approved parts of a script can run even when other capabilities are declined.
Denied API calls do nothing and return zero/empty values or inactive handles;
check results such as `Move` and `Subscription.Active` before relying on them.
Validation also requires a completed review because it executes globals and
`Init`.

The client finds API references automatically, including aliased imports and
functions saved in variables. Authors do not need a permission declaration.
Unused permissions are greyed out; references in unused helper functions still
count. Scripts cannot grant themselves access.

| Permission | API access |
| --- | --- |
| Register commands and shortcuts | `Command`, `AddShortcut` |
| Bind keys and mouse buttons | `Bind`, toolbar hotkeys and their input events |
| Send server commands | `Send`, `Equip`, `Unequip`, `WithEquipment` |
| Game data and events | `Self`, `Players`, inventory queries, selections, `CurrentWorld`, `OnWorld`, `OnChange`, image/world sizes, inventory/equipment waits, `WithEquipment`, `ItemSelector` |
| Chat and server messages | `OnChat`, `OnServerMessage`, `LatestServerMessage`, including private messages |
| Automatic movement | `Move`, `Movement` |
| Windows, toolbars and overlays | `CreateWindow`, `AddToolbar`, overlay drawing |
| Input box and pointer | `InputText`, `SetInputText`, `LastClick`, `Hover` |
| Notifications and sound | `ShowNotification`, `PlaySound` |
| Persistent script storage | `Store`, all `Load*` helpers, `DeleteStored`, `MigrateStorage` |
| Background timers | `Repeat` |
| Session events | `OnLogin`, `OnLogout`, `OnCharacterChange`, `OnStop` |

Console output, basic configuration, and `Wait`/`WaitTicks` remain available
after review without extra grants. `WithEquipment` needs both game data and
server commands. Toolbars request hotkey access; declining it leaves their
buttons available without bindings. Cleanup helpers such as `StopMoving` and
`OverlayClear` remain available to release resources.

Grants belong to a stable script ID and apply across characters. They are saved
in the client's `Scripts/permissions.json`, separate from the script's private
storage. Updating a script retains existing grants, but newly referenced
capabilities require review. Changing grants restarts an enabled script,
clearing its movement, windows, subscriptions, and pending actions. Previously
declined access stays declined on reload. An update requesting additional
capabilities waits for another review, while the previous running version stays
active. Granting starts the updated version with the reviewed access.

## A few important rules

- Scripts run locally, but they can send commands as your character. Read code
  from people you do not trust before enabling it.
- Prefer `gt2.Send` over trying to write directly to the network.
- State values are detached snapshots. Call the API again when you need current
  information.
- Use `gt2.Wait` rather than blocking loops or sleeps.
- Use `gt2.MigrateStorage` if a new script version changes its saved-data
  format.
- Registration functions return a `Subscription`. Calling `Remove()` stops
  that handler; stopping or reloading the script cleans up its registrations.

## Timers and app relaunches

All enabled scripts get a fresh interpreter at successful login and stop at
logout, including scripts enabled for **All** players. Login-screen selections
only configure enablement and permissions. Package variables, callbacks, timers,
and script-created windows never survive a session; use explicit storage for
data that should persist. `OnLogout` runs before `OnStop` and `Terminate`.

`gt2.Repeat` and `gt2.Wait` exist only while the script is running. Their
countdowns are cancelled when the script reloads, stops, or goThoom exits.

Storage belongs to the script and persists across app launches, but it is
shared by every character using that script. It is not automatically scoped to
the current character. For character-specific state, wait until a character is
known through `gt2.Self().Name` or `gt2.OnLogin`, normalize that name, and
include it in every related storage key.

For a task that must survive an app relaunch, store the last completed date or
the next due time with `gt2.Store`. In `Init` or the login handler, load the
current character's value, handle an overdue task, and then start a repeating
timer for checks during the current session. Do not treat a long `gt2.Repeat`
interval as persistent scheduling.

The bundled **Daily Reminder** example uses this pattern: it records the last
calendar day separately for each character, catches up once when that character
logs in on a later day, and checks periodically for a date change while the app
remains open.

For more examples, open **Actions -> Scripts -> Examples**. Those bundled
examples always match the scripting API in the current release.

## World data and movement

`CurrentWorld()` and `OnWorld` return detached snapshots. Mobile `H,V` positions
are sprite centers in unscaled world pixels, relative to the center of the
playfield; picture `H,V` positions are their top-left corners in that same
coordinate system. Overlay drawing instead uses a top-left origin: add half
`World.Width` and half `World.Height` when drawing these positions on an overlay.

Use `World.Self` only when `HasSelf` is true. `Mobile.Index` is a reusable
server descriptor slot, not a permanent player ID. Track a player by name and
ignore `Stale` mobiles retained briefly for rendering. `State` is the raw
animation byte; `Dead` is the client's fallen-pose interpretation. Sprite sizes
and picture planes describe artwork and drawing order, not collision bounds.
Scenery can include ground tiles, roofs, effects, shadows and retained artwork.

`World.Frame` is the logical server frame and `ReceivedAt` is when the client
accepted the update, independent of animation smoothing. Camera shifts are
estimates between adjacent frames. `OnWorld` provides the latest scene when
its serialized callback runs; multiple callbacks can see the same frame, and
slow handlers can miss intermediate frames. Use frame numbers to avoid double
counting movement. No world coordinates beyond the visible scene or server
collision map are available.

`Move(x, y)` supplies the same target as holding the movement mouse at those
centered coordinates. It does not warp the desktop cursor or find a route.
Refresh it more frequently than every 500 milliseconds. Each request replaces
that script's previous request and expires after 500 milliseconds, so there is
no movement backlog. `Move` returns false during Init/validation, without a
live session or recent world data, while another script owns movement, and
during manual movement plus a one-second grace period. Manual mouse, keyboard,
gamepad and legacy-macro movement take priority. `Movement().LastManualInput`
lets scripts cancel an ongoing task even while their movement is idle.

`StopMoving()` releases only the calling script's movement. The client also
releases it when the script stops or reloads, or the character logs out or
changes. Script movement uses the ordinary server input cadence.

## Follow Player example

Install **Follow Player** from **Actions -> Scripts -> Examples**. In
**Info -> Permissions**, grant Register commands and shortcuts, Game data and events, Automatic movement,
Windows/toolbars/overlays, Background timers, and Session events. Enable it
and its **Follow Player** window opens. Select a visible player in Players and
press **Follow**. The window shows the selected player, current target, and
activity: Following, Staying, Routing, Giving space, Wiggling, Waiting for space,
or Stopped. **Stop Follow** or manual movement cancels following. Closing the
window also stops following; `/followui` reopens it.

`/follow Player Name`, `/follow` for the selected player, `/follow off`, and
`/stopfollow` remain available. Following is session-only and never starts
automatically after a reload or login. The window, button callbacks, and status
logic are defined entirely in `follow_player.go`.

Normally the script just aims the movement mouse 24 pixels behind the visible
target, on the side nearest you. Normal mouse-distance speed control handles
catch-up. It starts following beyond 72 pixels and rests within 44 pixels;
these separate thresholds prevent repeated starts and stops near one distance.
Both distances are configurable in the Scripts window.

Every fresh scene update checks the direct path against other standing mobiles.
The script prefers 34 pixels of clearance (configurable), but treats that as a
soft preference so tighter passages remain possible. It rejects local paths
that approach within 18 pixels; if already closer, it permits moving away.
It checks the entire short path segment and prefers the same passing side to
reduce weaving, while reconsidering routes as mobiles move. When resting, it
also gives nearby mobiles space. If surrounded with no local exit, it waits
for an opening instead of pushing farther into a mobile.

When forward progress stalls for about 750 milliseconds, it tries a short
wiggle: backward to one side, backward to the other, then forward on each side.
Each pulse lasts about 350 milliseconds, followed by a return to normal
steering before another attempt. Recovery paths use the same mobile checks.
Lateral wiggle motion does not reset the retry budget; eight seconds of failed
recovery stops following. Camera-shift estimates keep a centered character
sprite from looking stationary during normal travel.

During a stall or when well behind, the script also considers estimated bases
of plane-zero scenery. Those hints are optional in the script settings.

This is a local steering example, not a complete pathfinder. Artwork hints can
be wrong, and complex walls, doorways, or moving crowds may require manual
repositioning. It stops when the target leaves view, your character falls, the
reported location changes, or world updates go stale; it does not chase across
unseen areas. The steering tests use synthetic scenes; real-world obstacle
clearance still needs in-game tuning.

## Script-owned windows

`gt2.CreateWindow` creates an independent, movable client window with wrapped
status text and up to eight buttons. It returns a `gt2.Window` handle. Use
`SetText`, `SetButtonEnabled`, `Show`, `Hide`, and `Remove` on that handle; updates
are sent to the client UI thread and do not recreate the window. Button IDs
must be nonempty and unique within the window.

```go
var panel gt2.Window

func Init() {
    panel = gt2.CreateWindow(gt2.WindowOptions{
        Title: "My Tool", Width: 340, Text: "Ready",
        Buttons: []gt2.WindowButton{
            {ID: "run", Label: "Run", OnClick: func() {
                panel.SetText("Working")
                panel.SetButtonEnabled("run", false)
            }},
        },
    })
    gt2.Command("mytool", func(args string) { panel.Show() })
}
```

`OnClose` in the options is optional and runs when the user closes the window
or `Hide()` is called. Closing hides the window; `Show()` can reopen it.
`Remove()` permanently removes it without firing `OnClose`. `Active()` remains
true while a live window is hidden. Validation and Init stage changes until
script activation succeeds. Stopping or reloading a script removes its windows
and prevents their old callbacks from running.
