goThoom Plugins

This folder contains example plugin scripts for goThoom.

Getting Started
- Copy or edit any of the example .go files to get started.
- Each plugin must define an Init() function. The client discovers and calls this function after loading the script.
- Each plugin must define a unique PluginName, PluginAuthor and PluginCategory string. Changing Name or Author will make the plugin unable to access old saved data!
- Place .go files in the data/plugins/ directory
- Hotkeys added by plugins appear in a "Plugin Hotkeys" section of the hotkeys window where you can enable or disable them.

API
The interpreter allows only these packages: gt, bytes, encoding/json,
errors, fmt, math, math/big, math/rand, regexp, sort, strconv,
strings, time, unicode/utf8.

Common API calls:
- gt.Console(msg) – write a message to the in-game console.
- gt.ShowNotification(msg) – pop up a notification on screen.
- gt.AddHotkey(combo, "/thing") – register a hotkey combo that runs a slash command.
- gt.AddHotkeyFn(combo, handler) – register a hotkey that calls your function; like Ctrl-Shift-D or RightClick.
- gt.Hotkeys() – list hotkeys registered by this plugin.
- gt.RemoveHotkey(combo) – remove a hotkey this plugin owns.
- gt.RegisterCommand(name, handler) – define a local slash command.
- gt.RunCommand("/thing") – send a command to the server immediately.
- gt.EnqueueCommand("/thing") – queue a command to send on the next tick, this avoids delaying player commands.
- gt.AddMacro("yy", "/yell") – expand a short prefix into a full command.
- gt.AddMacros(MapOfMacros) – register many macros at once.
- gt.RegisterInputHandler(handler) – inspect/change chat text before sending.
- gt.RegisterChatHandler(handler) – react to every chat message.
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

Go Syntax Basics
You don’t need to be a programmer to tweak these scripts. Here are the few
bits of Go syntax you’ll see most often:

- Comments: lines starting with // are notes and are ignored by the game.
- Strings: text goes in double quotes, like "hello".
- Assignment: = sets a value. Example: name = "Ada".
- Compare: == means equals, != means not equal.
- And / Or / Not: && means and, || means or, ! means not.
- If blocks: if condition { ... } else { ... } (the braces {} are required).
- Lists: []string{"a", "b"} is a list of strings.
- Maps (key → value lookups): map[string]string{"pp": "/ponder "}.
- Short variables: inside functions you’ll see x := 5 which means “create x
  with value 5”.

Handy string checks:
- Case-insensitive contains: gt.Includes(gt.Lower(text), gt.Lower(word)).
- Starts with: gt.StartsWith(text, prefix); Ends with: gt.EndsWith(text, suffix).

Example (only say “yes” when the message mentions your name and “boats”):

    if gt.Includes(gt.Lower(msg), gt.Lower(gt.PlayerName())) &&
       gt.Includes(gt.Lower(msg), "boats") {
        gt.Run("/whisper yes")
    }


Function Anatomy
A minimal plugin typically looks like this:

    //go:build plugin
    package main
    import "gt"
    const PluginName = "My Plugin"
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
        gt.Console("Hello, " + args)
    }

    func helloHotkey(e gt.HotkeyEvent) {
        gt.Run("/think Hello from hotkey " + e.Combo)
    }

Where to put files:
- Place .go files in the plugins/ directory next to the game or under
  your data directory (created automatically). The client scans both on start.


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

Plugin Tutorials
Each example file below is a complete tutorial. Copy the file into your
plugins/ folder and restart the game to activate it.

Examples (example_ponder.go)
Shows many features at once. Type /rad followed by a word to try a feature:
- /rad notify shows a popup.
- /rad players lists nearby players.
- /rad gear lists equipped items.
It also adds hotkeys:
- Ctrl-D runs a small dance routine.
- Ctrl-N shows a notification.

Default Macros (default_macros.go)
Replaces short text in the chat box with full commands using gt.AddMacros.
1. Type ?? followed by text to open /help.
2. Try typing pphello and it becomes /ponder hello.
Edit the Init function to add your own shortcuts with gt.AddMacro.

Chain Swap (chain_swap.go)
Quickly swap between your chain and the last weapon you used.
1. Have a chain and another weapon in your inventory.
2. Scroll the mouse wheel up or down or type /swapchain.
3. The plugin equips the chain. Scroll again to return to the previous weapon.

Coin Lord (coin_lord.go)
Tracks coins you pick up.
1. Type /cw to start or stop counting.
2. Use /cwnew to reset totals.
3. Use /cwdata or press Shift-C to see your total and coins per hour.

Sharecads (sharecads.go)
Automatically share when you see healing energy.
1. Type /shcads or press Shift-S to toggle the plugin.
2. When someone heals you, the plugin runs /share <name> once per person.

Kudzu (kudzu.go)
Helps with planting and moving kudzu seeds.
1. /zu plants a seed. Hotkey: Shift-K.
2. /zuget adds a seed to your bag.
3. /zustore removes a seed from the bag.
4. /zutrans name transfers seeds to someone else.

Bard Macros (bard.go)
Plays tunes without typing long commands.
1. Use /playsong <instrument> <notes> to play.
2. Press Shift-B to play a sample tune.
The plugin pulls the instrument from your case, plays it, then puts it back.

Dance Macros (dance.go)
Adds a simple dance command using /pose positions.
- Type /dance or press Shift-D to run a short pose routine. The hotkey
  calls the function directly (no extra slash command).

Dice Roller (dice_roll.go) - optional example
Roll virtual dice.
1. Type /roll NdM such as /roll 2d6.
2. The plugin rolls the dice, totals them, and announces the result.
If you have a dice item, it will try to equip it before rolling.

Weapon Cycle (weapon_cycle.go) - optional example
Cycle through a list of weapons with a single key.
1. Edit the cycleItems list in the file to match your weapons.
2. Press F3 or type /cycleweapon to equip the next item in the list.

Quick Reply (quick_reply.go)
Reply to the last exile who thinks to you.
1. Type /r <message> to respond with /thinkto <name> <message>.

Auto Yes Boats (auto_yes_boats.go)
Automatically whisper "yes" when approaching a boat vendor.
1. When the seller says "My fine boats", the plugin replies for you.

Right Click Mode (right_click_mode.go)
Cycle the action performed by right-clicking.
1. Use /pushpull, /trade, /healpotion, or /cadset to change modes.
2. Right-click a target to run the selected command.

Numpad Poser (numpad_poser.go)
Use the numeric keypad to strike poses quickly.
- Numpad1 → /pose leanleft
- Numpad2 → /pose akimbo
- Numpad3 → /pose leanright
- Numpad4 → /pose kneel
- Numpad5 → /pose sit
- Numpad6 → /pose angry
- Numpad7 → /pose lie
- Numpad8 → /pose seated
- Numpad9 → /pose celebrate
These hotkeys use gt.AddHotkeyFn and then send the appropriate /pose command.

Notes
- This directory is created automatically the first time the game runs.
