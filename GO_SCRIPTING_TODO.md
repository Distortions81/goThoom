# Go Scripting Redesign

Goal: make Go scripts clear and approachable for nontechnical users while
keeping Go's types, editor support, and ability to build larger tools. Legacy
macros are a usability benchmark, not an API or compatibility constraint.

This is Go scripting 2.0. The previous scripting API was broken and scarcely
used, so compatibility is not a goal. Remove or break old behavior whenever it
makes the final system clearer, smaller, or easier to use.

## Product direction

- Optimize first for clarity and ease of use by nontechnical script authors.
- Provide one obvious way to perform an action. Keep a second API only when it
  represents a genuinely different capability, not as an alias or convenience
  spelling.
- Do not spend time on a v1 compatibility or deprecation layer.
- Do not load v1 and v2 side by side or rewrite user scripts automatically.
  A removed API should fail clearly so the author can update the script once.
- Bundled scripts are part of the 2.0 product and must use the canonical API.
- Go already gives advanced authors general programming tools. Add lower-level
  game access only as a deliberate, coherent API rather than accumulating
  overlapping helpers.

## Design rules

- A useful script should need only `package main`, `import "gt2"`, and `Init()`.
- The filename is the default script name. Author, category, and description
  are optional metadata, not requirements for loading a local script.
- Script callbacks run in a predictable order. Users should not need to know
  about goroutines, locks, or the renderer's thread rules.
- Waiting must be as simple and safe as legacy `pause`, without blocking the
  game.
- Common tasks use short APIs. A lower-level API exists only for capabilities
  the normal API intentionally does not cover.
- Errors are visible in the Scripts window with the filename and source line.
- Reloading is atomic: a broken edit must not stop the last working version.
- Every public runtime symbol must exist in the `gt2` editor package and API
  tests. The runtime, stubs, and documentation cannot drift independently.
- Script-owned commands, bindings, events, timers, overlays, and settings must
  all disappear when the script stops or reloads.
- Keep the first version small. Add an abstraction only when real scripts need
  it.
- Delete redundant public symbols as soon as bundled scripts and tests no
  longer need them.

## Confirmed problems to fix first

These are present in the current implementation and should be covered by
regression tests before the API grows.

- [x] Make the script integration suite compile and run. Currently
  `go test -tags integration . -run '^TestScriptAPI'` fails to compile because
  several integration tests refer to removed fields, functions, and mutable
  variables.
- [x] Make Refresh and filesystem change detection actually reload enabled
  scripts. `rescanscripts` updates metadata but leaves an existing enabled
  script running its old interpreter and callbacks.
- [x] Replace the newest-modification-time watcher with a file snapshot that
  detects edits, additions, deletions, renames, and timestamps moving backward.
- [x] Make per-script Reload atomic: compile and initialize a candidate first,
  then replace the running script only after success.
- [x] Fix script hotkey persistence. The JSON loader's `script` field is
  unexported, disabled states are not preserved, and reload recreates bindings
  as enabled.
- [x] Make `gt2.Print` always print. It currently becomes silent unless the
  global script-output debug setting is enabled, despite being the documented
  way for a script to communicate with its user.
- [x] Replace the placeholder configuration system. `gt2.AddConfig` records only
  a name and type; its widgets have no values, callbacks, defaults, or
  persistence.
- [x] Stop shipping/scanning `scripts/api_test.go` as a user script. Test-only
  sources must not be embedded into the user-facing script directory.
- [x] Make the `gt2` editor package match runtime types and functions. Examples
  of current drift include the reduced `Player` stub, the unusable lowercase
  `clVersion`, the unused `Stats` type, and the click button type mismatch.
- [x] Flush dirty script storage on script stop, reload, and application exit;
  do not rely only on the one-minute timer.
- [x] Report unsupported storage values immediately instead of accepting them
  and failing later during JSON serialization.
- [x] Detect duplicate commands and bindings deterministically before scripts
  start. Current winner selection can depend on map iteration/load order.
- [x] Audit every callback path for panic recovery and ownership cleanup,
  including commands, chat, console, player events, input handlers, hotkeys,
  timers, and `Terminate`.
- [x] Remove direct UI and game-state work from arbitrary script goroutines.
  Route it through one owned dispatcher.

## Runtime foundation

- [x] Give each script one serialized, cancellable event queue.
- [x] Run callbacks sequentially by default so script-local variables are safe
  without locks.
- [x] Cancel queued callbacks, waits, timers, and repeating work on disable or
  reload.
- [x] Add execution limits per callback and per script without using one global
  goroutine count as the primary safety mechanism.
- [x] Recover script panics and show a useful error containing script name,
  callback/event name, and source location.
- [x] Keep the last working interpreter alive when a new version fails to load.
- [x] Separate compile, initialize, activate, deactivate, and dispose stages so
  partial initialization can be rolled back cleanly.
- [x] Give registrations opaque handles internally so cleanup does not depend
  on searching unrelated global slices and maps.
- [x] Define command ordering and throttling once. A script should not have to
  choose between confusing `Run`, `Cmd`, `RunCommand`, and `EnqueueCommand`
  variants.
- [x] Return visible errors when a command is rejected, rate-limited, or cannot
  find its target instead of silently doing nothing.

## Go scripting 2.0 API

Keep `Init()` and make metadata optional. A basic script should look like:

```go
package main

import "gt2"

func Init() {
	gt2.Command("hello", func(args string) {
		gt2.Print("Hello " + args)
	})

	gt2.Bind("Shift-H", func(event gt2.InputEvent) {
		gt2.Send("/think Hello!")
	})
}
```

### Core registration

- [x] Add `gt2.Command(name, func(args string))` as the command API.
- [x] Add `gt2.Bind(combo, func(InputEvent))` for keys, clicks, mouse chords,
  modifiers, and wheel input through one API.
- [x] Add `gt2.OnChat(filter, func(ChatEvent))` with structured speaker and
  message data.
- [x] Add `gt2.OnServerMessage(filter, func(ServerMessage))` so scripts do
  not have to reverse-engineer formatted console text.
- [x] Add lifecycle events for login, logout, character change, and script
  stop.
- [x] Add change events for inventory/equipment, selected player/item,
  health/spirit/balance, and world/location state.
- [x] Have every registration return a removable subscription, while normal
  scripts can ignore the return value.

### Commands and sequences

- [x] Add one `gt2.Send(command)` operation with ordered, rate-limited
  delivery.
- [x] Add `gt2.WaitTicks(n)` and `gt2.Wait(duration)` that suspend only the
  current script task and are cancelled on reload.
- [x] Add `gt2.Repeat(interval, func())` returning a stoppable timer.
- [x] Remove old timing and command aliases. Keep only `Send`, `Wait`,
  `WaitTicks`, and `Repeat`, whose names represent distinct operations.
- [x] Add a safe equipment helper that restores the prior slot after a task,
  covering the common dice, bard, pet, and weapon-swap patterns.
- [x] Allow a sequence to wait for an inventory/equipment state change with a
  timeout instead of guessing a fixed delay.
- [x] Add small command helpers only for repeated real-world needs; avoid one
  wrapper for every game slash command.

### Structured input events

- [x] Include key/button, modifiers, chord, screen/world position, clicked
  mobile, player name, simple name, and whether normal input may continue.
- [x] Let handlers explicitly consume or pass through an input event.
- [x] Cover printable keys, mouse buttons, wheel directions, and modifier
  combinations through `Bind`.
- [x] Add hover and current-selection queries without requiring scripts to poll
  every frame.

### Read-only game state

- [x] Add `gt2.Self()` returning a snapshot with name, health, spirit, balance,
  location, and equipped slots; remove the old string-name aliases.
- [x] Expose immutable `Player`, `Mobile`, `Item`, `Click`, and `World`
  snapshots from a real public `gt2` package rather than reflecting main-package
  implementation structs.
- [x] Add exact and case-insensitive item lookup, partial lookup, all matching
  instances, equipped-slot lookup, and stable per-instance identity.
- [x] Expose the selected player and selected inventory item.
- [x] Expose the latest typed server message data needed by scanners without
  requiring formatted-text parsing.
- [x] Document snapshot lifetime and make it impossible for scripts to mutate
  client-owned state accidentally.

## Settings and storage

- [x] Replace stringly typed `AddConfig(name, type)` with typed options:
  boolean, integer, decimal, text, choice, key binding, and item selector.
- [x] Give each option a stable key, label, help text, default, current value,
  validation, and optional change callback.
- [x] Persist settings automatically and show them in a functional Configure
  window.
- [x] Support global and per-character option scopes.
- [x] Add typed storage helpers for string, bool, integer, decimal, string
  slices, and JSON-safe structs.
- [x] Use a stable script ID for storage, with the filename as a convenient
  default for local drafts. Display metadata edits must not orphan saved data.
- [x] Add a storage schema version and a migration hook.
- [x] Make writes crash-safe and flush at lifecycle boundaries.

## Scripts window and authoring experience

- [x] Show the exact load/runtime error and source line instead of only
  `Invalid script`.
- [x] Show script path, description, API version, commands, bindings, events,
  timers, and settings in the details panel.
- [x] Add Copy Error, Open File, Open Folder, Reload, and Stop actions.
- [x] Keep Refresh, but make it rescan and atomically reload changed enabled
  scripts.
- [x] Add a New Script action with tiny templates for command, hotkey/click,
  chat event, and equipment sequence scripts.
- [x] Treat bundled examples as an opt-in library, similar to Legacy Macros,
  instead of copying every embedded source into the live scripts directory.
- [x] Never overwrite a locally edited script when bundled examples update.
- [x] Clearly distinguish Disabled, Running, Reload Failed (old version still
  running), and Stopped After Error.
- [x] Put ordinary script messages in the console and reserve debug logging for
  registration/event traces.

## 2.0 cleanup and cutover

- [x] Migrate every bundled script to the canonical 2.0 API before finalizing
  the public surface.
- [x] Remove public v1 aliases and redundant entry points, including old
  command, timing, hotkey, chat-trigger, input-text, inventory, and storage
  spellings.
- [x] Remove raw polling APIs where a structured subscription or snapshot is
  the normal solution.
- [x] Remove internal compatibility wrappers and tests that exist only to
  preserve the abandoned API.
- [x] Use `gt2` as the only package/import name so old and new scripts cannot
  be confused; do not retain a `gt` import alias.
- [x] Never rewrite user scripts automatically. Example installation is
  explicit and refuses to replace an existing file.
- [x] Set and document the finished scripting API as version 2 after the
  surface and bundled examples are settled.

## Editor and validation tooling

- [x] Create one canonical API definition used to generate or verify runtime
  exports, `gt2` editor stubs, and the API reference.
- [x] Ship the `gt2` editor module and workspace configuration beside user
  scripts so `import "gt2"` resolves without manual setup.
- [x] Add a Validate action that compiles a script without activating it.
- [x] Make the validator report unsupported imports and API-version problems
  clearly.
- [x] Move critical Yaegi smoke tests into the normal `go test ./...` suite.
- [x] Add a contract test that fails when runtime exports and `gt2` stubs differ.
- [x] Add deterministic event simulation for command, key/click, chat, server
  message, inventory, login, and timer handlers.
- [x] Add reload tests proving that old registrations disappear exactly once
  and failed replacements leave the old script running.
- [x] Run examples through both Yaegi and the editor stub type checker.

## Proof scripts

The new API is not ready until these can be written simply and tested without
sleeping the test process:

- [x] Dice roll: command arguments, temporary equipment, wait, and restore.
- [x] Bard instrument case: multi-step inventory changes with timeouts.
- [x] Shift-click pull/push: structured click context and pass-through control.
- [x] Quick reply: structured speaker data rather than parsing displayed chat.
- [x] Rangery: keys, wheel input, player state, and equipment actions.
- [x] Coin/last-hit counter: server events, counters, configuration, and
  persistent per-character state.
- [x] Scanner: typed server messages and several independent subscriptions.
- [x] Long-running timer: cancellation and cleanup during reload.

## Completed implementation phases

1. [x] Deleted internal v1 compatibility code and tests.
2. [x] Converted and tested the proof scripts without real-time sleeps, using
   them to simplify the public API before declaring its surface finished.
3. [x] Established one canonical API definition that generates or verifies
   runtime exports, editor stubs, and reference documentation.
4. [x] Shipped editor setup and a clear Validate action for ordinary script
   authors.
5. [x] Added deterministic event simulation, reload coverage, and example
   checks to the normal test suite.
6. [x] Documented the final surface and declared scripting API version 2.

## Definition of done

- A new user can create a working command or click script from the UI without
  knowing repository layout, build tags, Yaegi, goroutines, or JSON files.
- Dice roll, Bard instruments, Rangery, quick reply, and a counter/scanner are
  short, readable examples for nontechnical authors.
- Saving a valid edit reloads it; saving a broken edit reports the line and
  leaves the prior version running.
- Disabling or reloading a script leaves no commands, callbacks, timers,
  overlays, settings controls, or goroutines behind.
- Script hotkey and configuration choices survive reload and restart.
- `gt2.Print` is always visible, and failures are never silently discarded.
- Runtime exports, editor stubs, reference documentation, and examples agree.
- Each normal action has one documented public API; no v1 aliases remain.
- The normal test suite exercises script loading, core API calls, events,
  cleanup, reload failure, and persistence.
