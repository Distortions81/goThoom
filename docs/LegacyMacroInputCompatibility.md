# Legacy macro input compatibility

Reference checked: ClanLordClient commit
[`6ba334cfb3fb779ecfe37e0b635fac476cb73a5e`](https://github.com/YappyGM/ClanLordClient/tree/6ba334cfb3fb779ecfe37e0b635fac476cb73a5e),
September 8, 2026. This inventory covers input names and dispatch, not the
entire macro language.

The reference tables and parser are `mac_client/client/source/Macros_cl.cp`:
`gModNameMap`, `gKeyNameMap`, and `GetKeyByName`. Platform translation is in
`mac_client/dtslib2/source/mac/Shell_mac.cp`, `Mac2DTSModifiers` and `Mac2DTSKey`.

## Modifier names

The classic table contains exactly `command`, `control`, `numpad`, `option`,
and `shift`, matched without regard to case. `Cmd`, `Opt`, and Open Apple are
not entries in that table, even though comments sometimes use abbreviations.

goThoom uses that same modifier table for legacy macros. The existing
`wnumpad` compatibility spelling is evidenced by the bundled official example
at `source/testdata/legacy_macros/web/official-example.mac:40`.

Hotkeys and Go script bindings have a separate, existing vocabulary: `Meta`,
`Alt`, `Ctrl`, and `Shift`, with `Command`/`Cmd` and `Control` aliases. These
are not additional entries in the classic macro table.

The classic parser discards unknown modifier words and empty hyphen segments.
goThoom does not turn these into an unmodified key binding. Caps Lock is
ignored during classic matching; it is not a named macro modifier.

## Named keys and behavior

All entries in the classic named-key table are recognized by the legacy parser:

- `escape`, `f1` through `f16`, `minus`, `delete`, `tab`, `return`, `space`,
  `help`, `home`, `pageup`, `del`, `end`, `pagedown`, `up`, `down`, `left`,
  `right`, `clear`, `enter`.
- `click`, `click2` through `click8`, `right-click`.
- `wheelup`, `wheeldown`, `wheelleft`, `wheelright`.

| Classic spelling or behavior | goThoom handling |
| --- | --- |
| `delete` / `del` | Backspace / forward Delete; separate bindings and input consumption. |
| `return` / `enter` | Main Return / keypad Enter. Keypad Enter does not require `numpad`; existing `numpad-enter` is accepted as an alias. |
| `clear` | Escape alias. Physical keypad Clear carries `numpad`; Ebitengine reports this as NumLock. |
| `help` | Maps to the Insert key reported by Ebitengine. |
| Keypad digits and operators | Carry the `numpad` modifier. |
| Option/Shift character bindings | Use the produced character where available, including MacRoman source decoded to Unicode. Keyboard layout matters. |
| Right click | Physical `click2` first, then classic Control-click fallback when not consumed; Players-list context uses Control-click. |
| Shift + vertical wheel | Maps to horizontal wheel and removes Shift, matching the classic dispatch. |

Parsing a name does not guarantee that the input backend can deliver the
physical key or button. Ebitengine currently exposes five mouse buttons, so
`click6` through `click8` have no physical dispatch path. Native Option dead-key
composition and unusual keyboard hardware still require verification on macOS.

## Regression coverage

`legacy_macro_input_compatibility_test.go` checks the classic modifier and
named-key inventories, rejects unsupported modifier aliases, and verifies
Delete/Backspace and Return/Enter separation, keypad Clear, and editor conflict
checking. `legacy_macro_right_click_test.go` covers classic right-click order,
Players-list context, and `$no_override`. Existing legacy macro tests cover
printed Option characters and Shift-wheel translation.
