# Legacy Macro Compatibility

Goal: load and run existing Clan Lord `.mac` files alongside goThoom's Go
scripts, with behavior matching the reference client where practical.

## Working foundation

- [x] Load `Macros/<character name>` and its legacy `Default` includes.
- [x] Parse declarations, comments, quoted strings, escapes, control flow,
      functions, variables, replacements, and input triggers.
- [x] Run macros cooperatively without blocking rendering or network input.
- [x] Integrate macro input, cancellation, reload, diagnostics, and the bundled
      public macro library.
- [x] Keep Go scripts and each character's macro selection separate.

## Runtime correctness

- [x] Resolve variable indexes such as `chess[index]`, `@text.word[index]`,
      and chained `.letter[index]` lookups.
- [x] Make each return wait one macro frame, while allowing long-running loops
      that regularly return or pause.
- [x] Match legacy text behavior: undefined variables become empty text,
      adjacent values concatenate, and expression `@text` excludes its trigger.
- [x] Match string substring comparisons and case-insensitive variable names.
- [x] Support legacy `equip`, `unequip`, and `msg` commands used by the bundled
      corpus.
- [x] Add missing variable aliases and make letter operations Unicode-safe.

## Input behavior

- [x] Treat punctuation as a replacement boundary, not part of the word.
- [x] Preserve normal unmodified clicks in the Players window.
- [x] Match printed-character key bindings, including shifted symbols, and
      document keys modern keyboards cannot report.
- [x] Handle every mouse button exposed by Ebitengine and shifted wheel input.
- [x] Prevent macro movement and normal movement from overwriting each other.

## Lifecycle and reliability

- [x] Start `@login` only after authentication succeeds.
- [x] Read legacy MacRoman macro files while keeping UTF-8 files working, and
      losslessly escape unsupported Unicode in commands and chat at the server
      boundary.
- [x] Synchronize macro command delivery with the network command queue.
- [x] Bound runtime history and keep diagnostics current while the window is
      open.
- [x] Save library selections atomically and make embedded library Info work in
      WebAssembly.

## Library, documentation, and tests

- [x] Keep one metadata guide and template: `testdata/legacy_macros/web/METADATA.md`.
- [x] Safely deliver updated bundled files without overwriting user edits.
- [x] Correct imported corpus source where HTML escaping changed macro code.
- [x] Execute representative paths from every bundled macro, not only parse
      them, and add focused regression tests for each fix above.

## Remaining known gaps

- [ ] Feed real input-box selections into `@textsel` when the input box gains
      text-selection support.
- [ ] Verify printed keys, shifted wheel input, and extra mouse buttons on real
      desktop hardware across the supported platforms.

## Compatibility boundaries

- Existing `scripts/*.go` behavior remains unchanged.
- Legacy macro files are read from `Macros/` beside the executable/data files.
- Macro execution must be cooperative: pauses cannot block rendering or network
  input, and tight runaway loops must still be stopped safely.
- Do not auto-convert or overwrite users' macro files.
- The published Omega Zu source has one extra closing brace. It is reported as
  a source error while the remaining macros continue to load.
- Modern input APIs do not expose the old `undo` key or mouse buttons beyond
  the five buttons Ebitengine reports.
