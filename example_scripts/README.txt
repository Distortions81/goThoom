goThoom Scripts

This folder contains example script files for goThoom.

Getting Started
- Copy or edit any of the example .go files to get started.
- Each script must define an Init() function. The client discovers and calls this function after loading the file.
- Each script must define a unique PluginName, PluginAuthor and PluginCategory string. Changing Name or Author will make the script unable to access old saved data!
- Place .go files in the scripts/ directory next to the game.
- Hotkeys added by scripts appear in a "Plugin Hotkeys" section of the hotkeys window where you can enable or disable them.

API
The interpreter allows only these packages: gt, bytes, encoding/json,
errors, fmt, math, math/big, math/rand, regexp, sort, strconv,
strings, time, unicode/utf8.

Common API calls:
- gt.Print(msg) – write a message to the in-game console.
- gt.ShowNotification(msg) – pop up a notification on screen.
- gt.AddHotkey(combo, "/thing") – register a hotkey combo that runs a slash command.
- gt.AddHotkeyFn(combo, handler) – register a hotkey that calls your function; like Ctrl-Shift-D or RightClick.
- gt.Hotkeys() – list hotkeys registered by this script.
- gt.RemoveHotkey(combo) – remove a hotkey this script owns.
- gt.RegisterCommand(name, handler) – define a local slash command.
- gt.RunCommand("/thing") – send a command to the server immediately.
- gt.EnqueueCommand("/thing") – queue a command to send on the next tick, this avoids delaying player commands.
- gt.AddShortcut("yy", "/yell ") – expand a short prefix into a full command.
- gt.AddShortcuts(MapOfShortcuts) – register many shortcuts at once.
- gt.RegisterInputHandler(handler) – inspect/change chat text before sending.
- gt.PlayerName() – name of your current character.
- gt.Players() – slice of known players with basic info.
- gt.Inventory() – slice of inventory items.
- gt.EquippedItems() – list of currently equipped items.
- gt.HasItem(name) – whether your inventory has an item by name.
- gt.MouseWheel() – get scroll wheel movement since last frame.
- gt.KeyJustPressed(name) – check keyboard keys.
- gt.SetInputText(txt) and gt.InputText() – set or read the chat input box.
- Simple text helpers: gt.Lower, gt.Upper, gt.IgnoreCase, gt.StartsWith,
  gt.EndsWith, gt.Includes, gt.Trim, gt.TrimStart, gt.TrimEnd,
  gt.Words, gt.Join, gt.Replace, gt.Split.

Function Anatomy
A minimal script typically looks like this:

    //go:build plugin
    package main
    import "gt"
    const PluginName = "My Script"
    const PluginAuthor = "You"
    const PluginCategory = "Utilities"
    const PluginAPIVersion = 1

    func Init() {
        // Add a local command you can type as "/hello".
        gt.RegisterCommand("hello", helloCmd)
        // Bind a hotkey to a function. e.Combo is like "Ctrl-H".
        gt.AddHotkeyFn("Ctrl-H", helloHotkey)
    }

    func helloCmd(args string) {
        gt.Print("Hello, " + args)
    }

    func helloHotkey(e gt.HotkeyEvent) {
        gt.RunCommand("/think Hello from hotkey " + e.Combo)
    }

Where to put files:
- Place .go files in the scripts/ directory next to the game.

Key and Mouse Names
Hotkeys and input functions refer to keys and mouse buttons by specific names.
Combine modifiers with - like Ctrl-Shift-A. Names are case-insensitive.

Modifiers: Ctrl, Alt, Shift

Mouse buttons for hotkeys: LeftClick, RightClick, MiddleClick, Mouse 3,
Mouse 4, …

Mouse buttons for MousePressed and MouseJustPressed: right, middle,
mouse1, mouse2, mouse3, …

Mouse wheel: WheelUp, WheelDown, WheelLeft, WheelRight

Key names:

A, Alt, AltLeft, AltRight, ArrowDown, ArrowLeft, ArrowRight, ArrowUp, B,
Backquote, Backslash, Backspace, BracketLeft, BracketRight, C, CapsLock, Comma,
ContextMenu, Control, ControlLeft, ControlRight, D, Delete, Digit0, Digit1,
Digit2, Digit3, Digit4, Digit5, Digit6, Digit7, Digit8, Digit9, E, End, Enter,
Equal, Escape, F, F1, F10, F11, F12, F13, F14, F15, F16, F17, F18, F19, F2,
F20, F21, F22, F23, F24, F3, F4, F5, F6, F7, F8, F9, G, H, Home, I, Insert,
IntlBackslash, J, K, L, M, Meta, MetaLeft, MetaRight, Minus, N, NumLock,
Numpad0, Numpad1, Numpad2, Numpad3, Numpad4, Numpad5, Numpad6, Numpad7,
Numpad8, Numpad9, NumpadAdd, NumpadDecimal, NumpadDivide, NumpadEnter,
NumpadEqual, NumpadMultiply, NumpadSubtract, O, P, PageDown, PageUp, Pause,
Period, PrintScreen, Q, Quote, R, S, ScrollLock, Semicolon, Shift, ShiftLeft,
ShiftRight, Slash, Space, T, Tab, U, V, W, X, Y, Z

Examples
Copy the files from this folder into your scripts/ directory and restart the game.

