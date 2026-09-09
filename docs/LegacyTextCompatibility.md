# Legacy text compatibility

Clan Lord sends text using the classic MacRoman character set. goThoom keeps
text as normal Unicode internally, then converts it only when communicating
with the server.

## Sending text

Use emoji shortcodes such as `:smile:`, `:thumbs_up:`, or `:rocket:` in outgoing
messages, including speech commands such as `/think :rocket:`. Shortcodes are
sent unchanged. Pasted emoji are converted to names when recognized: 😄 becomes
`:smile:` on the wire. Scripts and macros follow the same rule. Message history
keeps the text you typed.

In chat and speech bubbles, both Unicode emoji and recognized shortcodes display
as emoji, whether the message is yours or another player's. Names are
case-insensitive; underscores and spaces are interchangeable. Unknown names
and names without both colons stay unchanged. Noto Color Emoji is bundled with
the client, so no system emoji font is needed. Clients without shortcode support
see the readable names.

To keep shortcodes visible, turn off **Settings → Text → Chat & Messages →
Show :smile: as emoji**. This affects chat and bubble display; literal emoji
still render normally and outgoing encoding stays the same.

Characters available in MacRoman are sent as their original single byte.
Unicode characters that MacRoman cannot represent are sent as readable ASCII
escapes instead:

| Text | Sent through the MacRoman connection |
|---|---|
| `café` | `caf` followed by the MacRoman byte for `é` |
| `☺` | `\u263A` |
| `🚀` | `\U0001F680` |

Literal backslashes are sent unchanged. Unknown or malformed sequences such as
`\q` remain literal text. A literal sequence that exactly matches `\uXXXX` or
`\UXXXXXXXX` is interpreted as a Unicode escape when received.

This keeps every wire message valid MacRoman without losing Unicode. A client
that understands these escapes restores the original text. An older client
will display the readable escape instead of mojibake.

Messages are limited to 511 encoded bytes. Emoji names count toward that limit
as ordinary text. Truncation preserves complete Unicode escapes for characters
sent using that form.

## Receiving text

goThoom first decodes the MacRoman bytes, then recognizes only these forms:

- `\\` from older clients becomes one literal backslash.
- `\uXXXX` becomes one Unicode character using four hexadecimal digits.
- `\UXXXXXXXX` becomes one Unicode character using eight hexadecimal digits.

Unknown or malformed escapes are left unchanged. Examples such as `\q`,
`\u12G4`, incomplete escapes, surrogate values, and code points outside the
Unicode range remain literal text.

## Macro files

Macro files are separate from server messages. goThoom accepts both original
MacRoman `.mac` files and modern UTF-8 files, including UTF-8 files with a byte
order mark. Macro source is not treated as escaped wire text, so existing macro
syntax such as `\r` and `\\` continues to work normally.

## Go functions

The compatibility boundary is provided by:

```go
func EncodeMacRomanEscaped(s string) ([]byte, error)
func DecodeMacRomanEscaped(b []byte) (string, error)
```

Normal application and macro text should remain UTF-8 Go strings. Use these
functions only where text enters or leaves the legacy MacRoman protocol.

The focused local round-trip test is:

```bash
go -C source test . -run '^TestMacRomanEscapedRoundTrip$' -count=1 -v
```
