# Legacy Macro Compatibility

Goal: load and run existing Clan Lord `.mac` files alongside goThoom's Go
scripts, with behavior matching the reference client where practical.

## Implementation order

- [x] Add the legacy macro runtime state and a loader for
      `Macros/<character name>`, including its legacy `Default` includes.
- [x] Parse files, comments, quoted strings, escapes, `include`, and report
      source file/line errors.
- [x] Parse macro declarations: expression, replacement, function, key, click,
      and wheel triggers; parse `$ignore_case`, `$any_click`, and
      `$no_override`.
- [x] Implement text expansion plus `set`, `setglobal`, `call`, `pause`, and
      `message`.
- [x] Implement `if` / `else if` / `else` / `end if`, `random`, `label`, and
      `goto`, with execution limits for bad loops.
- [x] Provide `@text`, `@my`, `@selplayer`, `@click`, `@env`, and legacy alias
      variables from current game state.
- [x] Integrate expression/replacement macros with the input bar and key,
      click, and wheel macros with existing input handling.
- [ ] Add Ctrl-Escape cancellation and key/click interruption behavior.
- [ ] Add Reload Macros and visible parse/runtime errors.
- [ ] Implement reference modifier-click player interaction: select, insert
      name, label cycle, block/ignore cycle.
- [ ] Add parser and end-to-end compatibility fixtures, then document legacy
      macros next to Go scripts.

## Compatibility boundaries

- Existing `scripts/*.go` behavior remains unchanged.
- Legacy macro files are read from `Macros/` beside the executable/data files.
- Macro execution must be cooperative: pauses cannot block rendering or network
  input, and runaway loops must be stopped safely.
- Do not auto-convert or overwrite users' macro files.
