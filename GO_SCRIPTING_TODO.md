# Go Scripting Redesign

Goal: make Go scripts as quick to write and reason about as legacy macros,
while keeping Go's types, editor support, and ability to build larger tools.

This is a redesign of the scripting experience, not a promise to preserve the
current API exactly. Existing scripts should continue to work through a small
compatibility layer while bundled scripts move to the new API.

## DESIGN NOTES

Don't waste time with legacy support, break/remove anything needed to get to end goal of new better scripting system.
In general lets avoid multiple way to do things unless there is a good reason.
I would go for maximum clarity and ease of use for non-tech users, break/remove anything needed to make this amazing.
It's golang not a macro, so power users will be able to do whatever they like... I think for them we make eventual goal of a dedicated simple API that has direct access to whatever they like.
The old system was broken and not really used so yeah I would treat this as golang scripts 2.0.

## Design rules

- A useful script should need only `package main`, `import "gt"`, and `Init()`.
- The filename is the default script name. Author, category, and description
  are optional metadata, not requirements for loading a local script.
- Script callbacks run in a predictable order. Users should not need to know
  about goroutines, locks, or the renderer's thread rules.
- Waiting must be as simple and safe as legacy `pause`, without blocking the
  game.
- Common tasks use short APIs; lower-level APIs remain available when needed.
- Errors are visible in the Scripts window with the filename and source line.
- Reloading is atomic: a broken edit must not stop the last working version.
- Every public runtime symbol must exist in the `gt` editor package and API
  tests. The runtime, stubs, and documentation cannot drift independently.
- Script-owned commands, bindings, events, timers, overlays, and settings must
  all disappear when the script stops or reloads.
- Keep the first version small. Add an abstraction only when real scripts need
  it.

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
- [x] Make `gt.Print` always print. It currently becomes silent unless the
  global script-output debug setting is enabled, despite being the documented
  way for a script to communicate with its user.
- [x] Replace the placeholder configuration system. `gt.AddConfig` records only
  a name and type; its widgets have no values, callbacks, defaults, or
  persistence.
- [x] Stop shipping/scanning `scripts/api_test.go` as a user script. Test-only
  sources must not be embedded into the user-facing script directory.
- [x] Make the `gt` editor package match runtime types and functions. Examples
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

## Simple v2 API

Keep `Init()` and make metadata optional. A basic script should look like:

```go
package main

import "gt"

func Init() {
	gt.Command("hello", func(args string) {
		gt.Print("Hello " + args)
	})

	gt.Bind("Shift-H", func(event gt.InputEvent) {
		gt.Send("/think Hello!")
	})
}
```

### Core registration

- [x] Add `gt.Command(name, func(args string))` as the preferred command API.
- [x] Add `gt.Bind(combo, func(InputEvent))` for keys, clicks, mouse chords,
  modifiers, and wheel input through one API.
- [x] Add `gt.OnChat(filter, func(ChatEvent))` with structured speaker and
  message data.
- [x] Add `gt.OnServerMessage(filter, func(ServerMessageEvent))` so scripts do
  not have to reverse-engineer formatted console text.
- [x] Add lifecycle events for login, logout, character change, and script
  stop.
- [x] Add change events for inventory/equipment, selected player/item,
  health/spirit/balance, and world/location state.
- [x] Have every registration return a removable subscription, while normal
  scripts can ignore the return value.

### Commands and sequences

- [x] Add one preferred `gt.Send(command)` operation with ordered, rate-limited
  delivery.
- [x] Add `gt.WaitTicks(n)` and `gt.Wait(duration)` that suspend only the
  current script task and are cancelled on reload.
- [x] Add `gt.Repeat(interval, func())` returning a stoppable timer.
- [x] Keep the old timing and command names as compatibility aliases.
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
- [x] Support the same printable keys, mouse buttons, wheel directions, and
  modifier combinations as the legacy macro runtime.
- [x] Add hover and current-selection queries without requiring scripts to poll
  every frame.

### Read-only game state

- [x] Add `gt.Self()` returning a snapshot with name, health, spirit, balance,
  location, and equipped slots; remove the old string-name aliases.
- [x] Expose immutable `Player`, `Mobile`, `Item`, `Click`, and `World`
  snapshots from a real public `gt` package rather than reflecting main-package
  implementation structs.
- [x] Add exact and case-insensitive item lookup, partial lookup, all matching
  instances, equipped-slot lookup, and stable per-instance identity.
- [x] Expose the selected player and selected inventory item.
- [x] Expose the latest typed server message data needed by legacy-style
  scanners without requiring formatted-text parsing.
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
- [x] Use an explicit stable script ID for storage. Display name and author
  edits must not orphan saved data.
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
- [ ] Treat bundled examples as an opt-in library, similar to Legacy Macros,
  instead of copying every embedded source into the live scripts directory.
- [ ] Never overwrite a locally edited script when bundled examples update.
- [ ] Clearly distinguish Disabled, Running, Reload Failed (old version still
  running), and Stopped After Error.
- [ ] Put ordinary script messages in the console and reserve debug logging for
  registration/event traces.

## Editor and validation tooling

- [ ] Create one canonical API definition used to generate or verify runtime
  exports, `gt` editor stubs, and the API reference.
- [ ] Ship the `gt` editor module and workspace configuration beside user
  scripts so `import "gt"` resolves without manual setup.
- [ ] Add a Validate action that compiles a script without activating it.
- [ ] Make the validator report unsupported imports and API-version problems
  clearly.
- [ ] Move critical Yaegi smoke tests into the normal `go test ./...` suite.
- [ ] Add a contract test that fails when runtime exports and `gt` stubs differ.
- [ ] Add deterministic event simulation for command, key/click, chat, server
  message, inventory, login, and timer handlers.
- [ ] Add reload tests proving that old registrations disappear exactly once
  and failed replacements leave the old script running.
- [ ] Run examples through both Yaegi and the editor stub type checker.

## Proof scripts

The new API is not ready until these can be written simply and tested without
sleeping the test process:

- [ ] Dice roll: command arguments, temporary equipment, wait, and restore.
- [ ] Bard instrument case: multi-step inventory changes with timeouts.
- [ ] Shift-click pull/push: structured click context and pass-through control.
- [ ] Quick reply: structured speaker data rather than parsing displayed chat.
- [ ] Rangery: keys, wheel input, player state, and equipment actions.
- [ ] Coin/last-hit counter: server events, counters, configuration, and
  persistent per-character state.
- [ ] Scanner: typed server messages and several independent subscriptions.
- [ ] Long-running timer: cancellation and cleanup during reload.

## Compatibility and migration

- [ ] Freeze and document API v1 behavior before introducing v2.
- [ ] Keep v1 symbols as adapters where they can be supported safely.
- [ ] Emit one clear deprecation message per script, not one per call.
- [ ] Migrate bundled scripts first and use the migration to refine the API.
- [ ] Provide a short v1-to-v2 mapping guide.
- [ ] Do not modify user scripts automatically.
- [ ] Do not change a script's storage identity without an explicit migration.

## Suggested implementation order

1. Repair the integration tests and add API/stub contract coverage.
2. Fix reload, cleanup, hotkey persistence, output, and storage flushing.
3. Introduce the serialized per-script dispatcher and atomic lifecycle.
4. Define the minimal v2 surface: `Command`, `Bind`, `Send`, `WaitTicks`,
   structured events, and read-only state.
5. Implement typed settings and storage.
6. Rebuild the Scripts window around validation, errors, configuration, and
   atomic reload.
7. Convert the proof scripts, then document and freeze API v2.

## Definition of done

- A new user can create a working command or click script from the UI without
  knowing repository layout, build tags, Yaegi, goroutines, or JSON files.
- Dice roll, Bard instruments, Rangery, quick reply, and a counter/scanner are
  shorter or clearer than their legacy equivalents.
- Saving a valid edit reloads it; saving a broken edit reports the line and
  leaves the prior version running.
- Disabling or reloading a script leaves no commands, callbacks, timers,
  overlays, settings controls, or goroutines behind.
- Script hotkey and configuration choices survive reload and restart.
- `gt.Print` is always visible, and failures are never silently discarded.
- Runtime exports, editor stubs, reference documentation, and examples agree.
- The normal test suite exercises script loading, core API calls, events,
  cleanup, reload failure, and persistence.
