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
3. Select the script in the Scripts window and enable it globally or for the
   current character.
4. Save your changes. goThoom notices file changes and reloads enabled scripts
   automatically. **Refresh** forces a rescan.

The first time `data/Scripts` has no script packages, goThoom copies its
embedded examples there. Your existing scripts are never replaced.

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
