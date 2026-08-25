package main

import (
	"gt2"
	"time"
)

type apiStoredState struct {
	Name  string
	Count int
}

func Init() {
	gt2.Print("apifull:init")
	gt2.ShowNotification("notification")
	gt2.AddShortcut("yy", "/yell ")
	gt2.AddShortcut("gg", "/give ")
	gt2.SetInputText("in_text")

	gt2.Command("apit_cmd", func(args string) { gt2.Store("last_args", args) })
	gt2.Bind("Ctrl-Alt-F", func(event gt2.InputEvent) {
		event.Consume()
		gt2.Store("hkf", "ok")
	})
	configValue := gt2.Bool(gt2.BoolOption{Key: "enabled", Label: "Enabled", Default: false, OnChange: func(enabled bool) {
		if enabled {
			gt2.Store("config_callback", "yes")
		}
	}})
	gt2.Store("config_default", configValue)

	gt2.Store("typed_string", "hello")
	gt2.Store("typed_bool", true)
	gt2.Store("typed_integer", 7)
	gt2.Store("typed_decimal", 2.5)
	gt2.Store("typed_strings", []string{"a", "b"})
	gt2.Store("typed_json", apiStoredState{Name: "state", Count: 3})
	gt2.Store("loaded_string", gt2.LoadString("typed_string", "missing"))
	gt2.Store("loaded_bool", gt2.LoadBool("typed_bool", false))
	gt2.Store("loaded_integer", gt2.LoadInteger("typed_integer", 0))
	gt2.Store("loaded_decimal", gt2.LoadDecimal("typed_decimal", 0))
	gt2.Store("loaded_strings", gt2.LoadStrings("typed_strings", nil))
	var loadedJSON apiStoredState
	gt2.Store("loaded_json_ok", gt2.LoadJSON("typed_json", &loadedJSON))
	gt2.Store("loaded_json_name", loadedJSON.Name)
	gt2.Store("loaded_json_count", loadedJSON.Count)

	gt2.OverlayClear()
	gt2.OverlayRect(1, 2, 3, 4, 5, 6, 7, 8)
	gt2.OverlayText(2, 3, "txt", 10, 11, 12, 13)
	gt2.OverlayImage(1, 4, 5)
	w, h := gt2.WorldSize()
	gt2.Store("world_w", w)
	gt2.Store("world_h", h)
	iw, ih := gt2.ImageSize(1)
	gt2.Store("img_w", iw)
	gt2.Store("img_h", ih)

	gt2.Store("cl_version", gt2.CLVersion)
	gt2.Store("player_field", gt2.Player{}.Offline)
	gt2.Store("me", gt2.Self().Name)
	gt2.Store("players_len", len(gt2.Players()))
	gt2.Store("inv_len", len(gt2.Inventory()))
	gt2.Store("has_shield", gt2.HasItem("Shield"))
	gt2.Store("is_equipped", gt2.IsEquipped("Shield"))
	lc := gt2.LastClick()
	gt2.Store("click_x", int(lc.X))
	gt2.Store("click_y", int(lc.Y))
	gt2.Store("click_btn", lc.Button)
	gt2.Store("click_onmobile", lc.OnMobile)

	repeat := gt2.Repeat(time.Hour, func() { gt2.Store("repeat_should_not_run", "yes") })
	repeat.Stop()
	go func() {
		gt2.WaitTicks(2)
		gt2.Wait(time.Millisecond)
		gt2.Store("slept", "yes")
	}()

	gt2.OnChat(gt2.ChatFilter{Contains: "ping"}, fullChat)
	gt2.OnChat(gt2.ChatFilter{Contains: "ping", Kinds: gt2.ChatNPC}, fullNPCChat)
	gt2.OnServerMessage(gt2.ServerMessageFilter{Contains: "ready"}, fullServerMessage)
	gt2.OnChange(gt2.ChangeInventory, fullInventoryChange)

	gt2.Send("/ponder canonical")
	gt2.PlaySound([]uint16{1})
	gt2.Store("started", "yes")
}

func fullChat(event gt2.ChatEvent)                { gt2.Store("chat_message", event.Message) }
func fullNPCChat(event gt2.ChatEvent)             { gt2.Store("chat_npc", true) }
func fullServerMessage(message gt2.ServerMessage) { gt2.Store("server_type", message.Type) }
func fullInventoryChange(event gt2.ChangeEvent)   { gt2.Store("inventory_changed", true) }
