goThoom Scripts

This folder contains example script files for goThoom.

Getting Started
- Copy or edit any of the example .go files to get started.
- Start each file with `//go:build script`.
- Each script must define an Init() function. The client discovers and calls this function after loading the file.
- Each script must define a unique scriptName, scriptAuthor and scriptCategory string. Changing Name or Author will make the script unable to access old saved data!
- Place .go files in the scripts/ directory next to the game.
- Hotkeys added by scripts appear in a "script Hotkeys" section of the hotkeys window where you can enable or disable them.

Run the client with `go run .` or the built binary and any scripts in this folder load automatically.

API
The interpreter allows only these packages: gt, bytes, encoding/json,
errors, fmt, math, math/big, math/rand, regexp, sort, strconv,
strings, time, unicode/utf8.

Common API calls:
- gt.Print(msg) – write a message to the in-game console.
- gt.ShowNotification(msg) – pop up a notification on screen.
- gt.Command(name, handler) – define a local slash command.
- gt.RegisterCommand is the compatibility name for Command.
- gt.Send("/thing") – preferred FIFO command operation with per-script throttling.
- gt.Run, gt.Cmd, gt.RunCommand, and gt.EnqueueCommand are compatibility aliases.
- gt.WaitTicks(n) and gt.Wait(duration) pause only the current script task and
  are cancelled when the script stops or reloads. SleepTicks is an alias.
- gt.Repeat(interval, fn) runs a serialized callback repeatedly and returns a
  Timer whose Stop method is safe to call more than once.
- gt.WithEquipment(name, task) temporarily equips an item and restores the
  equipment that previously occupied its slot when the task returns or panics.
- gt.WaitForInventory and gt.WaitForEquipment wait for declarative item state
  with a timeout and are cancelled on script stop or reload.
- gt.AddHotkey(combo, "/thing") – bind a combo to a slash command.
- gt.Key(combo, func()) – bind a combo directly to a no-argument function.
- gt.AddHotkeyFn(combo, func(HotkeyEvent)) – bind a combo directly to a
  function that receives the combo, its parts, and its trigger key.
- gt.Bind(combo, func(InputEvent)) – preferred unified key, mouse, modifier,
  chord, and wheel binding API.
- gt.AddShortcut("yy", "/yell ") – expand a short prefix in the input.
- gt.AddShortcuts(map[string]string) – register many shortcuts at once.
- gt.RegisterInputHandler(handler) – inspect/change chat text before sending.
- gt.OnChat(filter, func(ChatEvent)) – receive parsed speaker, message, raw text,
  and chat kind flags.
- gt.OnServerMessage(filter, func(ServerMessageEvent)) – receive unformatted
  server console text and its message type.
- gt.OnLogin, gt.OnLogout, gt.OnCharacterChange, and gt.OnStop receive lifecycle
  events with character transitions and stop reasons.
- gt.OnChange(kind, func(ChangeEvent)) receives structured inventory, equipment,
  selection, vitals, world, and location changes. Use the gt.Change* constants.
- Preferred registration calls return an optional Subscription. Call Remove()
  to unregister it; scripts may ignore the return value.
- gt.PlayerName() – name of your current character.
- gt.Players() – slice of known players with basic info.
- gt.Inventory() – slice of inventory items.
- gt.EquippedItems() – list of currently equipped items.
- gt.HasItem(name) – whether your inventory has an item by name.
- gt.IsEquipped(name) – whether an item by name is equipped.
- gt.MouseWheel() – get scroll wheel movement since last frame.
- gt.KeyJustPressed(name) – check keyboard keys.
- gt.SetInputText(txt) and gt.InputText() – set or read the chat input box.
- Simple text helpers: gt.Lower, gt.Upper, gt.IgnoreCase, gt.StartsWith,
  gt.EndsWith, gt.Includes, gt.Trim, gt.TrimStart, gt.TrimEnd,
  gt.Words, gt.Join, gt.Replace, gt.Split.

Function Anatomy
A minimal script typically looks like this:

    //go:build script
    package main
    import "gt"
    const scriptName = "My Script"
    const scriptAuthor = "You"
    const scriptCategory = "Utilities"
    const scriptAPIVersion = 1

    func Init() {
        // Add a local command you can type as "/hello".
        gt.RegisterCommand("hello", helloCmd)
        // Bind a hotkey to a named function.
        gt.Key("Ctrl-H", helloHotkey)
    }

    func helloCmd(args string) {
        gt.Print("Hello, " + args)
    }

    func helloHotkey() {
        gt.Run("/think Hello!")
    }
    // Or use AddHotkeyFn when you need the full HotkeyEvent:
    // gt.AddHotkeyFn("Ctrl-H", func(e gt.HotkeyEvent) { helloHotkey() })

Where to put files:
- Place .go files in the scripts/ directory next to the game.

Key and Mouse Names
Hotkeys and input functions refer to keys and mouse buttons by specific names.
Combine modifiers with - like Ctrl-Shift-A. Names are case-insensitive.

Modifiers: Ctrl, Alt, Shift

Mouse buttons for hotkeys: LeftClick, RightClick, MiddleClick, Mouse3,
Mouse4, … (`Mouse 3` and `Mouse 4` are also accepted.)

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
