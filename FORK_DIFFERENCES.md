# Macro-system differences: goThoomMacro vs goThoom

This is a macro-only comparison of goThoomMacro `4ce704b` and goThoom
`3402fb3`, with later goThoom macro work included where it changes the result.
It describes implementation and observable macro features, not unrelated client
UI changes.

| Area | goThoomMacro fork | This goThoom repository |
| --- | --- | --- |
| Implementation | `macro_parser.go`, `macro_exec.go`, `macro_files.go`, `macro_types.go`, and `macro_vars.go`, built around linked `Macro` nodes and one global `MacroState`. | `legacy_macros.go`, `legacy_macro_runtime.go`, `legacy_macro_input.go`, and `legacy_macro_variables.go`, built around parsed source lines/declarations and a separate runtime. |
| Macro roots | Creates `Macros/Default`, then loads one case-insensitive character-named file. The character file is expected to include other sources it needs. | Loads the character-named file plus selected library sources. Includes are resolved under `Macros/` and may not escape that directory. |
| Bundled macros | Ships a `Macros/Default` starter source. | Ships the public legacy-macro library in `data/Macros/Library/`, including an official example and the published Gorvin/clump collections. |
| Library management | No source-library selection or management layer. | **Legacy Macros** window manages Global and Player selections in `Macros/Library/enabled.json`, installs bundled files without overwriting local edits, reloads sources, shows diagnostics, and opens local files for editing where supported. |
| Source decoding | Uses the fork parser's file reader. | Accepts UTF-8 and original MacRoman macro files; preserves unsupported backslash escapes instead of consuming them. |
| Parse diagnostics | Stores source file/line information in macro nodes and reports parser messages. | Preserves source path, line, and column in diagnostics; reports malformed braces, unsafe/missing includes, and runtime errors without retaining stale programs. |
| Execution model | Linked active-macro list advanced each game frame; pause, movement, text output, function calls, labels/goto, conditionals, and random branches. | Frame-driven active executions with step, call-depth, and history limits; pause does not block the client. Supports the same core controls plus explicit execution diagnostics. |
| Movement | Macro `move` plus fork-local `/move` client command. | Legacy macro `move` drives native walk/run/stop input. |
| Triggers | Expression, replacement, function, key, and click macros; wheel clicks are represented as click codes. | Expression, replacement, function, key, click, and wheel declarations; keyboard/mouse consumption prevents a macro trigger from also firing the ordinary hotkey/input path. |
| Variables | Global/local variables and `@env`, `@my`, `@selplayer`, and `@click` namespaces, including text word/letter helpers. | The same legacy-style variable families, resolved from a captured execution context so player, selected-player, clicked-player/item, equipment, sharing, and input state do not drift during a running macro. |
| Input integration | Macro processing runs before console input and can suppress it. | Trigger processing is integrated with player, inventory, world, and input events; modifier-click behavior and movement are tracked per input frame. |
| UI | A simple Macro information window and Reload Macros button; parsed bindings are listed through the shortcuts UI. | Legacy Macros library UI, per-source metadata/info, Global/Player enable controls, Reload, diagnostics, and editor buttons. Shortcuts also show active bindings. |
| Compatibility coverage | Parser/file unit tests for the fork implementation. | Unit tests plus the embedded public legacy macro corpus, including published complex examples such as clump, chess, and tetris macros. |

## Compatibility note

Both clients aim at the original Clan Lord macro language, but their parsers,
runtime state, source-loading rules, and settings are independent. A macro file
can often be copied between them, but it should be tested in the destination
client—especially when it relies on source load order, includes, player/input
state, or local client commands.
