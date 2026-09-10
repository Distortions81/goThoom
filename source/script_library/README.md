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
   automatically. **Reload** reads the selected script from disk and restarts it.
   **Refresh** rescans the folder.

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
- `gt2.QueueCommand(command)` returns a ticket for delivery status and cancellation.
- `gt2.StartTask(callback)` starts a cancellable sequence.
- `gt2.After(delay, callback)` schedules one cancellable callback.
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
- `gt2.SetMobileTint(id, r, g, b, a)` tints visible mobiles with that sprite ID.
- `gt2.FlashMobile(index, r, g, b, a, duration)` briefly replaces a visible mobile's colors with a flash color.
- `gt2.Move(x, y)`, `gt2.StopMoving()`, and `gt2.Movement()` steer through the
  normal game input loop and report script/manual movement state.
- `gt2.Wait(...)` and `gt2.WaitTicks(...)` pause only the current script task.
- `gt2.Repeat(...)` runs a serialized callback repeatedly.
- `gt2.Store(...)` and the `gt2.Load*` functions keep private script data.
- `gt2.CharacterStore()` provides the same storage helpers scoped to the current character.
- `gt2.OnPlayerChange(handler)` reports observed player presence, fallen, and sharing changes.
- `gt2.HasPermission(id)` checks which capabilities the user granted.

Open `gt2/API_REFERENCE.md` inside the active scripts folder for every type,
function, constant, and example supported by your installed goThoom version.

Scripts can import only `gt2` and these standard packages:

```text
bytes, encoding/json, errors, fmt, math, math/big, math/rand,
regexp, sort, strconv, strings, time, unicode/utf8
```

## Preferences and controls

Open **Actions → Scripts → Info → Settings** while the script is running.
**Preferences** contains the fields supplied by the script. Each field shows
whether it applies to all characters or the current character; changes save
immediately.

**Key bindings** and **Commands** list the script's registered controls, even
when it has no preferences. Change a binding by typing it or using **Record**,
then choose **Apply**. Command names can be changed without changing their
arguments or actions. **Reset** restores a control's script-defined default.
These control overrides apply across characters and survive reloads and client
restarts. Conflicting assignments are rejected. Renaming a command changes the
name you type; update any personal macros or hotkeys that call the old name.

### Add preferences to a script

Register typed options in `Init`. Each call returns the saved value, or its
`Default` on first use. Use `OnChange` to update the running script:

```go
var enabled bool
var interval int

func Init() {
    enabled = gt2.Bool(gt2.BoolOption{
        Key: "enabled", Label: "Show reminders", Default: true,
        OnChange: func(value bool) { enabled = value },
    })
    interval = gt2.Integer(gt2.IntegerOption{
        Key: "interval", Label: "Reminder interval", Help: "Minutes between reminders.",
        Scope: gt2.ScopeCharacter, Default: 10, Min: 1, Max: 60, Step: 1,
        OnChange: func(value int) { interval = value },
    })
}
```

Keep `Key` stable and unique within the script. Omitted `Scope` means
`gt2.ScopeGlobal`; use `gt2.ScopeCharacter` for a separate value per character.
Options are private to the script. They are saved automatically, so there is
no need to call `gt2.Store` for them. `OnChange` runs when an accepted value
changes; assign the registration's return value for initialization.

The available types are `Bool`, `Color`, `Integer`, `Decimal`, `Text`, `Choice`,
`KeyBinding`, and `ItemSelector`. Colors are packed as `0xRRGGBBAA`. Use `Help` to explain a useful tradeoff,
`Min`/`Max`/`Step` for numeric controls, and `Choices` for a dropdown.
`Bool`, `Integer`, `Decimal`, and `Text` also accept a `Validate` function.
A `KeyBinding` preference supplies a string to your script; bindings registered
with `gt2.Bind` appear automatically in the **Key bindings** tab.

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
| Send server commands | `Send`, `QueueCommand`, `Equip`, `Unequip`, `WithEquipment` |
| Game data and events | `OnPlayerChange`, `Self`, `Players`, inventory queries, selections, `CurrentWorld`, `OnWorld`, `OnChange`, image/world sizes, inventory/equipment waits, `WithEquipment`, `ItemSelector` |
| Chat and server messages | `OnChat`, `OnServerMessage`, `LatestServerMessage`, including private messages |
| Automatic movement | `Move`, `Movement` |
| Windows, toolbars and overlays | `CreateWindow`, `AddToolbar`, color pickers, overlay drawing, mobile tints, outlines and flashes |
| Input box and pointer | `InputText`, `SetInputText`, `LastClick`, `Hover` |
| Notifications and sound | `ShowNotification`, `PlaySound` |
| Persistent script storage | `CharacterStore` and its methods, `Store`, all `Load*` helpers, `DeleteStored`, `MigrateStorage` |
| Background timers | `Repeat`, `After`, `StartTask` |
| Session events | `OnLogin`, `OnLogout`, `OnCharacterChange`, `OnStop` |

`HasPermission` accepts these IDs: `commands`, `hotkeys`, `send`, `data`,
`messages`, `movement`, `windows`, `input`, `notifications`, `storage`, `timers`,
and `session`. Unknown IDs return false. Querying a permission does not request
it or open a review; the client discovers requests from the APIs your script uses.

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

`gt2.Repeat`, `gt2.After`, tasks, and `gt2.Wait` exist only while the script is running. Their
countdowns are cancelled when the script reloads, stops, or goThoom exits.

`gt2.Store` and the top-level `Load*` functions share data across the script's
characters. Use `gt2.CharacterStore()` for data belonging to the current character:

```go
saved := gt2.CharacterStore()
if saved.Active() {
    count := saved.LoadInteger("reminders", 0)
    saved.Store("reminders", count+1)
}
```

The handle supplies `Store`, `LoadString`, `LoadBool`, `LoadInteger`,
`LoadDecimal`, `LoadStrings`, `LoadJSON`, and `DeleteStored`. It binds to the
normalized current character. An unknown character returns an inactive handle;
obtain a new one after login. Retain the handle to save during `OnLogout`,
`OnStop`, or `Terminate`. An old handle cannot access another character's
storage or a replacement script session. Denied or inactive reads return their
fallbacks; writes do nothing. Validation stages writes until activation succeeds.

Existing scripts may continue including normalized character names in ordinary
storage keys. `CharacterStore` uses its own namespace and does not automatically
move those values. To adopt it without losing saved state, copy your existing
values explicitly before deleting old keys.

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

## Cancellable tasks and delivery feedback

Use `StartTask` for a sequence that should have a Stop command or button:

```go
var job gt2.Task

func Init() {
    gt2.Command("posecycle", func(args string) {
        job.Cancel()
        job = gt2.StartTask(func() {
            gt2.Send("/pose sit")
            gt2.Wait(2 * time.Second)
            gt2.Send("/pose kneel")
        })
    })
    gt2.Command("stopposes", func(args string) { job.Cancel() })
}
```

Import `time` along with `gt2` for this example. Tasks run serially. During a
task's `Wait`, `WaitTicks`, `WaitForInventory`, or `WaitForEquipment`, ordinary
callbacks can run, allowing commands, hotkeys, and window buttons to cancel it.
Another task waits its turn. Keep ordinary callbacks short; their own long waits
can delay the task and other controls. Shared script variables can change while
a task waits, so read them again when necessary.

`Cancel` interrupts a wait and unwinds the task. Deferred Go functions run;
subsequent API calls still honor cancellation. Cancellation is cooperative at
API calls and waits, and CPU-only loops remain subject to execution limits.
Cancellation removes thetask's unsent commands, including commands from `Send`,
`Equip`, and `Unequip`. It cannot undo a command already being transmitted or
sent, and does not roll back equipment changes already made. Stopping or
reloading a script cancels its tasks and unsent commands. Successful task
completion leaves its queued commands available to send.

Use a ticket when your UI needs delivery feedback:

```go
ticket := gt2.QueueCommand("/pose sit")
// Read again later, for example from a timer or a button callback.
status := ticket.Status()
if status.State == gt2.CommandRejected {
    gt2.Print(status.Reason)
}
```

`Status` returns `CommandQueued`, `CommandSent`, `CommandCancelled`, or
`CommandRejected`. Rejections include empty commands, rate limits, and unavailable
access. A failed network write remains queued for retry. `CommandSent` means the
network write succeeded; it does not confirm that the server accepted or
completed the action. Observe game data or server messages for that confirmation.
`ticket.Cancel()` returns true when it removes unsent work, and false when it is
already finished or being transmitted. Cancelling one ticket preserves other
commands' order. If cancellation meets a write in progress and that write fails,
the command is cancelled before retrying.

`gt2.After(3*time.Second, callback)` returns the same `Timer` handle as `Repeat`.
Call `Stop()` to cancel it, including after the callback has been queued but
before it starts. A one-shot timer becomes inactive when its callback begins.
Zero delay schedules the callback; negative delays and nil callbacks return
inactive handles. Timers registered in `Init` start only after activation, and
all timers are cancelled when the script stops. Persist a due time for reminders
that need to survive a restart.

## Player changes

`gt2.OnPlayerChange(func(event gt2.PlayerChangeEvent) { ... })` returns a removable
subscription. Each event has `Type`, `Previous`, and `Player` snapshots:

- `PlayerDiscovered` and `PlayerRemoved` identify additions/removals from the
  client's known-player data. For removals, use `Previous`; `Player` is empty.
- `PlayerLogin` and `PlayerLogout` report observed changes in `Offline`.
- `PlayerFallen` and `PlayerRecovered` report changes in `Dead`.
- `PlayerSharing` reports a change in either `Sharing` or `Sharee`.

The first client snapshot establishes a baseline without announcing everyone as
newly logged in. A newly discovered player produces a discovery event, without
inventing earlier transitions. A player may produce several event types in one
update. These events describe state the client observes, not a complete server
event history; rapid changes between client checks can be missed. Entering or
leaving the visible scene is distinct from logging in or out; use world snapshots
for visibility. Query `Players()` for initial data.

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

### Mobile sprite tints

`SetMobileTint(id, r, g, b, a)` applies a multiplicative RGBA tint to every
visible mobile using that sprite ID. `ClearMobileTint(id)` removes one tint and
`ClearMobileTints()` removes every tint owned by the calling script. Tints are
released automatically when the script stops or reloads. The values multiply
the sprite's existing color and opacity, so use light colors for a visible
colored cue: `255, 128, 128, 255` makes a bright red-tinted marker without
making the sprite nearly black.

This API is useful when a creature type needs a distinct reminder during a
hunt. It affects the sprite image only; names, shadows, and collision behavior
are unchanged. If scripts tint the same ID, the tint belonging to the
lexicographically last script ID is used. A script can combine this with
`OnWorld` and the regular overlay functions to draw an outline around the same
mobiles.

`SetNamedMobileTint(name, r, g, b, a)` and `SetNamedMobileOutline(name, r, g, b, a)`
match an exact mobile name, ignoring case and surrounding spaces. Name matches
apply regardless of sprite ID and take precedence over sprite-ID marks.
`ClearNamedMobileTint(name)` and `ClearNamedMobileOutline(name)` remove one
name match; `ClearMobileTints()` and `ClearMobileOutlines()` clear both kinds.
These marks also disappear when their script stops or reloads.

Mark Beasts uses name matches when you Alt-click a named mobile. Its Name
field overrides the ID match; the ID supplies the sprite preview. Names,
colors, and notes save automatically, just like sprite entries.

`FlashMobile(index, r, g, b, a, duration)` briefly recolors one visible
mobile. It is useful for responding to an effect picture that appears at a
mobile's center, such as a hit indicator. The flash automatically expires after
`duration`; call it again while the effect remains visible to extend the cue.
At alpha 255 the sprite becomes the solid flash color while retaining its shape
and transparency. Lower alpha values mix the flash with the normal artwork.
As with tints, scripts sharing an
index resolve to the lexicographically last script ID.

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
**Info -> Permissions**, grant Register commands and shortcuts, Bind keys and
mouse buttons, Game data and events, Automatic movement, Windows/toolbars/overlays,
Background timers, and Session events. Enable it, then **Alt-right-click a
player in the game view** to follow them. Alt-right-click another player to
switch targets. The **Follow Player** window shows the current target and
activity, including Following breadcrumbs and Waiting for target. **Stop Follow**
or manual movement cancels following. Closing the window also stops following;
`/followui` reopens it.

`/follow Player Name`, `/follow off`, and `/stopfollow` remain available. A
partial name works when it matches exactly one visible player; a full name
takes precedence. Following is session-only and never starts
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

While the target is visible, the script considers estimated bases of plane-zero
scenery on every update to steer around objects. Those hints are optional in
the script settings.

When an attempted move makes little progress along its commanded direction for
about 750 milliseconds, the script records an approximate blocked spot ahead.
It routes around that spot toward the same player or breadcrumb, favoring a
consistent passing side. This works even when scenery artwork gives no useful
collision hint. Following the detour counts as progress, even if it temporarily
takes you farther from the destination. Temporary blocked spots expire and are
cleared when the scene changes.

If detours still produce almost no movement for two seconds while the target is
visible, the script permits a short wiggle to get unstuck. Movement ends the
wiggle and returns to routing. The stuck check subtracts measured background
movement before judging the response to a movement command.

When the target leaves view, the script follows their recorded breadcrumbs,
then continues up to 96 pixels beyond the last sighting along their recent
travel direction to cross an area edge or doorway. It preserves turns in the
recorded track. If a mobile or an inferred blockage obstructs a breadcrumb,
it detours around the obstruction and continues along the remaining trail.
The window shows **Routing to breadcrumb** during that detour.

Breadcrumbs and blocked spots stay anchored to the scenery as the background
moves. With no recent travel direction, or after reaching the end of the
continuation, it waits for the target to reappear.

After a location or scenery change, it discards the old area's coordinates and
looks for the target by name. It resumes following when they reappear, even if
their mobile index changes. There is no target-loss timeout: the target stays
selected until you cancel following or the session ends. Falling or missing
world updates pauses movement; following resumes when both players are standing
and fresh positions are available.

This is approximate local steering. Artwork does not expose doorways or server
collision boundaries, so complex walls, entrances, or moving crowds may require
manual repositioning. The steering tests use synthetic scenes; area transitions
and obstacle clearance still need in-game verification.

## Script-owned windows

The **Mark Beasts** example shows a sprite preview, editable ID and name, tint
and outline checkboxes with color swatches, notes, and a delete button on each
row. Use it for last-hit reminders, creature identification, or other notes.
New entries use an outline by default. Each row can use tint, outline, both,
or neither; turning an effect off keeps its color and notes.

**Add** creates a blank row. The gear-shaped **Settings** button opens the
script's native Settings page. Its preferences set the effect and color defaults
for new entries and control hit flashes. Changing defaults leaves existing rows
as configured. All edits save automatically, including notes on blank rows.
Alt-click a creature or player to add or remove its mark. Open the window from
**Mark Beasts** on the toolbar or `/marks`. The `/lasties` command also opens it.

`gt2.CreateWindow` creates an independent, movable client window with wrapped
status text, up to eight buttons, and up to 32 controls. It returns a `gt2.Window` handle. Use
`SetText`, `SetButtonEnabled`, `Show`, `Hide`, and `Remove` on that handle; updates
are sent to the client UI thread and do not recreate the window. Button IDs
must be nonempty and unique within the window. Set a button's `Icon` to a
bundled Material icon name such as `"settings"`; `gt2.OpenSettings()` opens the
calling script's native preferences, key bindings, and commands page.

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

### Editable window controls

Add `Controls: []gt2.WindowControl{...}` to `WindowOptions`. Every control needs
a unique `ID`, a `Label`, and a `Kind`. Control and button IDs share one namespace.

| Kind | Initial value | User callback value |
| --- | --- | --- |
| `ControlText` | `Text` | `event.Text` |
| `ControlCheckbox` | `Checked` | `event.Checked` |
| `ControlDropdown` | `Options`, `Selected` | `event.Selected`, `event.Text` |
| `ControlList` | `Options`, `Selected` | `event.Selected`, `event.Text` |
| `ControlColor` | packed `Color` (`0xRRGGBBAA`) | `event.Color` |
| `ControlImage` | sprite ID in `Image` (zero for an empty preview) | None |

`ControlList` shows selectable rows in a scrolling area. Lists and dropdowns
accept up to 256 strings. `Selected` is a zero-based index, or -1 for no selection;
an empty option list always has no selection. Set `OptionImages` to a matching
slice of sprite IDs to show artwork beside list rows. Pass the same slice as the
optional third argument to `SetControlOptions` when replacing the rows.
`ControlColor` uses the client's color swatch and HSV/opacity picker.
`Disabled` and `Tooltip` apply to
every kind. `OnChange` is optional and runs on the serialized script callback queue:

```go
panel := gt2.CreateWindow(gt2.WindowOptions{
    Title: "Reminder",
    Controls: []gt2.WindowControl{
        {ID: "message", Label: "Message", Kind: gt2.ControlText,
            Text: "Time for a break", OnChange: func(e gt2.WindowControlEvent) {
                gt2.CharacterStore().Store("message", e.Text)
            }},
        {ID: "notify", Label: "Show notification", Kind: gt2.ControlCheckbox,
            Checked: true},
    },
})
panel.SetControlText("message", "Rest and recover")
```

Use `SetControlText`, `SetControlChecked`, `SetControlColor`, `SetControlImage`, `SetControlSelected`,
`SetControlOptions`, and `SetControlEnabled` to update controls on an existing
window. Programmatic changes do not fire `OnChange`. Replacing options clears
the selection; set it again explicitly if needed. Invalid IDs, mismatched kinds,
and out-of-range selections are ignored. Stopping the script removes its controls
and discards their callbacks. Control values are session-only unless the script
saves them.

For inline editing, supply `Rows: []gt2.WindowRow{...}`. Each row groups its
`Controls` and `Buttons` horizontally; the first row's control labels become
column headings. Set each control or button's `Width` in logical pixels.
Rows share the window's ID namespace and use a scrolling area. `SetRows` replaces
them without moving the window; appended rows scroll into view. Update individual
fields with the control setters while typing to keep keyboard focus. A window
accepts up to 256 rows, each with up to 32 controls and eight buttons.
