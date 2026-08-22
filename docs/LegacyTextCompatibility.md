# Legacy text compatibility

Clan Lord sends text using the classic MacRoman character set. goThoom keeps
text as normal Unicode internally, then converts it only when communicating
with the server.

## Sending text

Characters available in MacRoman are sent as their original single byte.
Unicode characters that MacRoman cannot represent are sent as readable ASCII
escapes instead:

| Text | Sent through the MacRoman connection |
|---|---|
| `café` | `caf` followed by the MacRoman byte for `é` |
| `☺` | `\u263A` |
| `🚀` | `\U0001F680` |

A literal backslash is doubled. For example, text containing the literal
characters `\u263A` is sent as `\\u263A`. This prevents it from being mistaken
for the smiley escape when it comes back.

This keeps every wire message valid MacRoman without losing Unicode. A client
that understands these escapes restores the original text. An older client
will display the readable escape instead of mojibake.

## Receiving text

goThoom first decodes the MacRoman bytes, then recognizes only these forms:

- `\\` becomes one literal backslash.
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
go test . -run '^TestMacRomanEscapedRoundTrip$' -count=1 -v
```
