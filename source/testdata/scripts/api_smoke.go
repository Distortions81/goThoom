package main

import "gt"

func Init() {
	gt.Print("api: init")
	gt.AddShortcut("yy", "/yell ")
	gt.Command("apit_cmd", func(args string) {
		gt.Store("last_args", args)
		gt.Print("cmd:" + args)
	})
	removed := gt.Command("removed_cmd", func(args string) { gt.Store("removed_ran", args) })
	removed.Remove()
	gt.Bind("Ctrl-Alt-T", func(event gt.InputEvent) {
		gt.Store("hotkey", "triggered")
		gt.Store("hotkey_combo", event.Chord)
		event.Consume()
	})
	gt.Chat("ping", func(msg string) {
		gt.Store("chat", "ping")
		gt.Print("chat:" + msg)
	})
	gt.OnChat(gt.ChatFilter{Contains: "structured"}, func(event gt.ChatEvent) {
		gt.Store("structured_speaker", event.Speaker)
		gt.Store("structured_message", event.Message)
	})
	gt.OnServerMessage(gt.ServerMessageFilter{Contains: "structured server"}, func(event gt.ServerMessage) {
		gt.Store("server_message", event.Message)
		gt.Store("server_type", event.Type)
	})
	gt.OnLogin(func(event gt.LifecycleEvent) { gt.Store("login_character", event.Character) })
	gt.OnLogout(func(event gt.LifecycleEvent) { gt.Store("logout_character", event.Character) })
	gt.OnCharacterChange(func(event gt.LifecycleEvent) { gt.Store("previous_character", event.PreviousCharacter) })
	gt.OnStop(func(event gt.LifecycleEvent) { gt.Store("stop_reason", event.Reason) })
	gt.OnChange(gt.ChangeVitals, func(event gt.ChangeEvent) { gt.Store("health", event.Health) })
	gt.Console("ready", func(msg string) {
		gt.Store("console", "ready")
	})
	gt.SetInputText("test-in")
	gt.Store("started", "yes")
}
