package main

import (
	"gt"
	"time"
)

type apiStoredState struct {
	Name  string
	Count int
}

func Init() {
	gt.Print("apifull:init")
	gt.ShowNotification("notif1")
	gt.Notify("notif2")

	gt.AddShortcut("yy", "/yell ")
	gt.AddShortcuts(map[string]string{"gg": "/give "})
	gt.RegisterInputHandler(func(s string) string {
		if gt.StartsWith(gt.Lower(s), "foo ") {
			return "bar " + s[4:]
		}
		return s
	})
	gt.SetInputText("in_text")

	gt.RegisterCommand("apit_cmd", func(args string) { gt.Store("last_args", args) })
	gt.AddHotkey("Ctrl-U", "/wave")
	gt.RemoveHotkey("Ctrl-U")
	gt.Key("Ctrl-Alt-F", func() { gt.Store("hkf", "ok") })
	configValue := gt.Bool(gt.BoolOption{Key: "enabled", Label: "Enabled", Default: false, OnChange: func(enabled bool) {
		if enabled {
			gt.Store("config_callback", "yes")
		}
	}})
	gt.Store("config_default", configValue)
	gt.Store("typed_string", "hello")
	gt.Store("typed_bool", true)
	gt.Store("typed_integer", 7)
	gt.Store("typed_decimal", 2.5)
	gt.Store("typed_strings", []string{"a", "b"})
	gt.Store("typed_json", apiStoredState{Name: "state", Count: 3})
	gt.Store("loaded_string", gt.LoadString("typed_string", "missing"))
	gt.Store("loaded_bool", gt.LoadBool("typed_bool", false))
	gt.Store("loaded_integer", gt.LoadInteger("typed_integer", 0))
	gt.Store("loaded_decimal", gt.LoadDecimal("typed_decimal", 0))
	gt.Store("loaded_strings", gt.LoadStrings("typed_strings", nil))
	var loadedJSON apiStoredState
	gt.Store("loaded_json_ok", gt.LoadJSON("typed_json", &loadedJSON))
	gt.Store("loaded_json_name", loadedJSON.Name)
	gt.Store("loaded_json_count", loadedJSON.Count)

	gt.OverlayClear()
	gt.OverlayRect(1, 2, 3, 4, 5, 6, 7, 8)
	gt.OverlayText(2, 3, "txt", 10, 11, 12, 13)
	gt.OverlayImage(1, 4, 5)
	w, h := gt.WorldSize()
	gt.Store("world_w", w)
	gt.Store("world_h", h)
	iw, ih := gt.ImageSize(1)
	gt.Store("img_w", iw)
	gt.Store("img_h", ih)

	gt.Store("cl_version", gt.CLVersion)
	gt.Store("player_field", gt.Player{}.Offline)
	gt.Store("me", gt.Self().Name)
	gt.Store("players_len", len(gt.Players()))
	inv := gt.Inventory()
	gt.Store("inv_len", len(inv))
	gt.Store("has_shield", gt.HasItem("Shield"))
	gt.Store("is_equipped", gt.IsEquipped("Shield"))

	gt.Store("key_a", gt.KeyJustPressed("A"))
	gt.Store("mouse_right", gt.MouseJustPressed("right"))
	dx, dy := gt.MouseWheel()
	gt.Store("wheel_dx", dx)
	gt.Store("wheel_dy", dy)
	lc := gt.LastClick()
	gt.Store("click_x", int(lc.X))
	gt.Store("click_y", int(lc.Y))
	gt.Store("click_btn", lc.Button)
	gt.Store("click_onmobile", lc.OnMobile)

	gt.Store("eq_ic", gt.IgnoreCase("AbC", "aBc"))
	gt.Store("starts", gt.StartsWith("hello", "he"))
	gt.Store("ends", gt.EndsWith("hello", "lo"))
	gt.Store("incl", gt.Includes("hello", "ell"))
	gt.Store("lower", gt.Lower("HeLLo"))
	gt.Store("upper", gt.Upper("HeLLo"))
	gt.Store("trim", gt.Trim("  hi  "))
	gt.Store("trim_s", gt.TrimStart("--hi", "--"))
	gt.Store("trim_e", gt.TrimEnd("hi--", "--"))
	gt.Store("words", gt.Words("a b  c"))
	gt.Store("join", gt.Join([]string{"a", "b", "c"}, ","))
	gt.Store("repl", gt.Replace("piper", "pi", "ha"))
	gt.Store("split", gt.Split("x|y|z", "|"))

	gt.After(10, func() { gt.Store("after", "yes") })
	gt.AfterDur(15*time.Millisecond, func() { gt.Store("afterdur", "yes") })
	gt.Every(10, func() {
		n := gt.LoadString("every", "")
		if n == "" {
			gt.Store("every", "1")
			return
		}
		if n == "1" {
			gt.Store("every", "2")
		} else {
			gt.Store("every", "3")
		}
	})
	gt.EveryDur(15*time.Millisecond, func() {
		n := gt.LoadString("everydur", "")
		if n == "" {
			gt.Store("everydur", "1")
			return
		}
		if n == "1" {
			gt.Store("everydur", "2")
		} else {
			gt.Store("everydur", "3")
		}
	})
	repeat := gt.Repeat(time.Hour, func() { gt.Store("repeat_should_not_run", "yes") })
	repeat.Stop()
	go func() {
		gt.WaitTicks(2)
		gt.Wait(time.Millisecond)
		gt.Store("slept", "yes")
	}()

	gt.Chat("ping", func(msg string) { gt.Store("chat_any", "1") })
	gt.PlayerChat("ping", func(msg string) { gt.Store("chat_player", "1") })
	gt.NPCChat("ping", func(msg string) { gt.Store("chat_npc", "1") })
	gt.CreatureChat("ping", func(msg string) { gt.Store("chat_creature", "1") })
	gt.SelfChat("ping", func(msg string) { gt.Store("chat_self", "1") })
	gt.OtherChat("Other", "ping", func(msg string) { gt.Store("chat_other", "1") })
	gt.ChatFrom("Hero", "ping", func(msg string) { gt.Store("chat_from", "1") })
	gt.PlayerChatFrom("Hero", "ping", func(msg string) { gt.Store("chat_pfrom", "1") })
	gt.OtherChatFrom("Other", "ping", func(msg string) { gt.Store("chat_ofrom", "1") })
	gt.Console("ready", func(msg string) { gt.Store("cons_new", "1") })
	gt.RegisterConsoleTriggers([]string{"legacy"}, func() { gt.Store("cons_old", "1") })
	gt.RegisterTriggers("", []string{"bb"}, func(msg string) { gt.Store("legacy_trig", "1") })
	gt.RegisterTrigger("unit", "test", func() { gt.Store("sing_trig", "1") })
	gt.RegisterChatHandler(func(msg string) { gt.Store("allchat", msg) })
	gt.RegisterPlayerHandler(func(p gt.Player) { gt.Store("player_seen", p.Name) })

	gt.Run("/think hi")
	gt.Cmd("/say hi")
	gt.RunCommand("/shout ok")
	gt.EnqueueCommand("/pose ok")
	gt.Send("/ponder preferred")
	gt.PlaySound([]uint16{1})
	gt.Store("started", "yes")
}
