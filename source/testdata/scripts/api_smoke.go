package main

import "gt"

func Init() {
	gt.Print("api: init")
	gt.AddShortcut("yy", "/yell ")
	gt.Command("apit_cmd", func(args string) {
		gt.Save("last_args", args)
		gt.Print("cmd:" + args)
	})
	removed := gt.Command("removed_cmd", func(args string) { gt.Save("removed_ran", args) })
	removed.Remove()
	gt.Bind("Ctrl-Alt-T", func(event gt.InputEvent) {
		gt.Save("hotkey", "triggered")
		gt.Save("hotkey_combo", event.Chord)
		event.Consume()
	})
	gt.Chat("ping", func(msg string) {
		gt.Save("chat", "ping")
		gt.Print("chat:" + msg)
	})
	gt.OnChat(gt.ChatFilter{Contains: "structured"}, func(event gt.ChatEvent) {
		gt.Save("structured_speaker", event.Speaker)
		gt.Save("structured_message", event.Message)
	})
	gt.OnServerMessage(gt.ServerMessageFilter{Contains: "structured server"}, func(event gt.ServerMessageEvent) {
		gt.Save("server_message", event.Message)
		gt.Save("server_type", event.Type)
	})
	gt.OnLogin(func(event gt.LifecycleEvent) { gt.Save("login_character", event.Character) })
	gt.OnLogout(func(event gt.LifecycleEvent) { gt.Save("logout_character", event.Character) })
	gt.OnCharacterChange(func(event gt.LifecycleEvent) { gt.Save("previous_character", event.PreviousCharacter) })
	gt.OnStop(func(event gt.LifecycleEvent) { gt.Save("stop_reason", event.Reason) })
	gt.OnChange(gt.ChangeVitals, func(event gt.ChangeEvent) { gt.StorageSet("health", event.Health) })
	gt.Console("ready", func(msg string) {
		gt.Save("console", "ready")
	})
	gt.SetInputText("test-in")
	gt.Save("started", "yes")
}
