//go:build script

package main

import (
	"gt"
	"time"
)

const scriptName = "APIFull"
const scriptAuthor = "Test"
const scriptCategory = "Tests"
const scriptAPIVersion = 1

func Init() {
	// Console and notifications
	gt.Print("apifull:init")
	gt.ShowNotification("notif1")
	gt.Notify("notif2")

	// Shortcuts and input
	gt.AddShortcut("yy", "/yell ")
	gt.AddShortcuts(map[string]string{"gg": "/give "})
	gt.RegisterInputHandler(func(s string) string {
		if gt.StartsWith(gt.Lower(s), "foo ") {
			return "bar " + s[4:]
		}
		return s
	})
	gt.SetInputText("in_text")

	// Commands and hotkeys
	gt.RegisterCommand("apit_cmd", func(args string) { gt.Save("last_args", args) })
	gt.AddHotkey("Ctrl-U", "/wave")
	gt.RemoveHotkey("Ctrl-U")
	gt.Key("Ctrl-Alt-F", func() { gt.Save("hkf", "ok") })
	gt.AddConfig("optA", "string")

	// World overlay and geometry
	gt.OverlayClear()
	gt.OverlayRect(1, 2, 3, 4, 5, 6, 7, 8)
	gt.OverlayText(2, 3, "txt", 10, 11, 12, 13)
	gt.OverlayImage(1, 4, 5)
	w, h := gt.WorldSize()
	gt.StorageSet("world_w", w)
	gt.StorageSet("world_h", h)
	iw, ih := gt.ImageSize(1)
	gt.StorageSet("img_w", iw)
	gt.StorageSet("img_h", ih)

	// Player/world info
	gt.StorageSet("me", gt.Me())
	gt.StorageSet("players_len", len(gt.Players()))
	inv := gt.Inventory()
	gt.StorageSet("inv_len", len(inv))
	gt.StorageSet("has_shield", gt.HasItem("Shield"))
	gt.StorageSet("is_equipped", gt.IsEquipped("Shield"))

	// Input state
	gt.StorageSet("key_a", gt.KeyJustPressed("A"))
	gt.StorageSet("mouse_right", gt.MouseJustPressed("right"))
	dx, dy := gt.MouseWheel()
	gt.StorageSet("wheel_dx", dx)
	gt.StorageSet("wheel_dy", dy)
	lc := gt.LastClick()
	gt.StorageSet("click_x", int(lc.X))
	gt.StorageSet("click_y", int(lc.Y))
	gt.StorageSet("click_btn", lc.Button)
	gt.StorageSet("click_onmobile", lc.OnMobile)

	// String helpers
	gt.StorageSet("eq_ic", gt.IgnoreCase("AbC", "aBc"))
	gt.StorageSet("starts", gt.StartsWith("hello", "he"))
	gt.StorageSet("ends", gt.EndsWith("hello", "lo"))
	gt.StorageSet("incl", gt.Includes("hello", "ell"))
	gt.StorageSet("lower", gt.Lower("HeLLo"))
	gt.StorageSet("upper", gt.Upper("HeLLo"))
	gt.StorageSet("trim", gt.Trim("  hi  "))
	gt.StorageSet("trim_s", gt.TrimStart("--hi", "--"))
	gt.StorageSet("trim_e", gt.TrimEnd("hi--", "--"))
	gt.StorageSet("words", gt.Words("a b  c"))
	gt.StorageSet("join", gt.Join([]string{"a", "b", "c"}, ","))
	gt.StorageSet("repl", gt.Replace("piper", "pi", "ha"))
	gt.StorageSet("split", gt.Split("x|y|z", "|"))

	// Timers
	gt.After(10, func() { gt.Save("after", "yes") })
	gt.AfterDur(15*time.Millisecond, func() { gt.Save("afterdur", "yes") })
	gt.Every(10, func() {
		n := gt.Load("every")
		if n == "" {
			gt.Save("every", "1")
			return
		}
		if n == "1" {
			gt.Save("every", "2")
		} else {
			gt.Save("every", "3")
		}
	})
	gt.EveryDur(15*time.Millisecond, func() {
		n := gt.Load("everydur")
		if n == "" {
			gt.Save("everydur", "1")
			return
		}
		if n == "1" {
			gt.Save("everydur", "2")
		} else {
			gt.Save("everydur", "3")
		}
	})
	go func() {
		gt.SleepTicks(2)
		gt.Save("slept", "yes")
	}()

	// Triggers
	gt.Chat("ping", func(msg string) { gt.Save("chat_any", "1") })
	gt.PlayerChat("ping", func(msg string) { gt.Save("chat_player", "1") })
	gt.NPCChat("ping", func(msg string) { gt.Save("chat_npc", "1") })
	gt.CreatureChat("ping", func(msg string) { gt.Save("chat_creature", "1") })
	gt.SelfChat("ping", func(msg string) { gt.Save("chat_self", "1") })
	gt.OtherChat("Other", "ping", func(msg string) { gt.Save("chat_other", "1") })
	gt.ChatFrom("Hero", "ping", func(msg string) { gt.Save("chat_from", "1") })
	gt.PlayerChatFrom("Hero", "ping", func(msg string) { gt.Save("chat_pfrom", "1") })
	gt.OtherChatFrom("Other", "ping", func(msg string) { gt.Save("chat_ofrom", "1") })
	gt.Console("ready", func(msg string) { gt.Save("cons_new", "1") })
	// Use ConsoleMsg for console trigger with message parameter
	gt.ConsoleMsg("legacy", func(msg string) { gt.Save("cons_old", "1") })
	gt.RegisterTriggers("any", []string{"bb"}, func(msg string) { gt.Save("legacy_trig", "1") })
	// Single chat trigger: name "unit", phrase "ping"
	gt.RegisterTrigger("unit", "ping", func() { gt.Save("sing_trig", "1") })
	gt.RegisterChatHandler(func(msg string) { gt.Save("allchat", msg) })
	gt.RegisterPlayerHandler(func(p gt.Player) { gt.Save("player_seen", p.Name) })

	// Commands
	gt.Run("/think hi")
	gt.Cmd("/say hi")
	gt.RunCommand("/shout ok")
	gt.EnqueueCommand("/pose ok")

	// Sound (no assert)
	gt.PlaySound([]uint16{1})

	// Mark ready at the end
	gt.Save("started", "yes")
}
