# Using legacy macros in goThoom

Legacy macros are compatible with the traditional Clan Lord macro language.
They can expand typed text, bind keys and mouse actions, react to game text,
send commands, and run small sequences.

For new automation, the Go scripting system under **Actions -> Scripts** is
usually easier to write and debug. Use legacy macros when you already have a
`.mac` file or prefer the original macro language.

> The easiest way to find the active macro folder is **Actions -> Legacy
> Macros -> Open Macro Folder**. On macOS, use this button instead of the
> documentation copy beside the app.

## Enable a bundled macro

1. Open **Actions -> Legacy Macros**.
2. Click the **i** button to read a macro's description, commands, hotkeys, and
   attribution.
3. Check **Global** to use it for every character, or check your character's
   column to use it only for that character.
4. Use **Reload Macros** after editing a file.

The **Errors** button shows file names, line numbers, parse errors, and runtime
errors. Disabling a macro removes it from the active program without deleting
the file.

Enable **Allow continuous macros** for an intentional loop that keeps scanning
without pausing or producing output. When this is off, goThoom stops that loop
after 10,000 instructions. The classic client instead time-slices busy macros
without imposing an instruction cap.

## Add a downloaded macro

1. In the Legacy Macros window, choose **Open Macro Folder**.
2. Put the `.mac` file in the `Library` folder.
3. Return to goThoom and choose **Refresh List**.
4. Enable the new entry globally or for one character.

goThoom never overwrites files you add or edit. Bundled files are refreshed
only while they are still unchanged from the bundled copy.

Only use macros from people you trust. A macro can send game commands and act
as your character.

## Write a simple macro

Create a UTF-8 text file ending in `.mac` inside `Macros/Library`. This example
adds `hi` as a typed shortcut and binds Ctrl-H:

```text
// Metadata
// Name: My Hello Macro
// Tags: chat, example
// Desc: Adds a hello shortcut and hotkey.
// Author: Your Name

"hi"
{
    "/think Hello, " @text "!\r"
}

control-h "/think Hello!\r"
```

After enabling it:

- Type `hi everyone` to send `/think Hello, everyone!`.
- Press Ctrl-H to send `/think Hello!`.

`\r` submits the text as if Enter were pressed. Without it, text is placed in
the input field but is not sent.

## Common patterns

Send a command when a shortcut is typed:

```text
"ty" "/thank " @text "\r"
```

Run several commands from a named function:

```text
wave
{
    "/action waves.\r"
    pause 1
    "/pose bless\r"
}

"hello"
{
    call wave
    message "The hello macro ran."
}
```

Use variables and conditions:

```text
"count"
{
    set number @text.word[0]
    if number > 0
        message "The number is " number
    else
        message "Type: count 5"
    end if
}
```

Lines beginning with `//` are comments. Braces group multiple instructions.
The bundled **Delta Tao Example Macros** entry contains examples of movement,
loops, random choices, variables, functions, and input bindings.

## Keys and mouse actions

Key names can be combined with traditional modifiers such as `command`,
`control`, `option`, `shift`, and `numpad`:

```text
shift-f5 "/yell Safe!\r"
option-numpad-1 "/pose kneel\r"
```

The bundled macro information panel lists the bindings detected in each file.
Very old `undo` bindings need a different key on modern keyboards, and mouse
buttons beyond the five reported by goThoom cannot trigger a macro.

## Per-character files and Default

The Library checkboxes are the simplest setup. Traditional layouts are also
supported: a file in `Macros` whose name exactly matches your character is
loaded when that character logs in. It can include shared definitions:

```text
include "Default"
```

`Default` is not loaded automatically; include it from the character file when
using this traditional layout. Library selections are stored separately in
`Library/enabled.json` and do not rewrite `Default` or character files.

## Text and metadata

Modern UTF-8 and original MacRoman macro files are accepted. These escapes are
available inside quoted text:

- `\r` sends Enter.
- `\\` produces a backslash.
- `\"` and `\'` produce quote characters.

Optional metadata comments make a macro easier to identify in the library:

```text
// Metadata
// Name: Friendly Name
// Version: 1.0
// Tags: chat, utility
// Desc: A short description.
// Author: Your Name
// License: MIT
// Website: https://example.com/
// Update: https://example.com/my-macro.mac
```

See `Library/METADATA.md` for a copyable template and explanation of each
field.
