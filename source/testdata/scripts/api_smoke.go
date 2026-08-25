package main

import "gt2"

func Init() {
	gt2.Print("api: init")
	gt2.AddShortcut("yy", "/yell ")
	gt2.Command("apit_cmd", func(args string) {
		gt2.Store("last_args", args)
		gt2.Print("cmd:" + args)
	})
	removed := gt2.Command("removed_cmd", func(args string) { gt2.Store("removed_ran", args) })
	removed.Remove()
	gt2.Bind("Ctrl-Alt-T", func(event gt2.InputEvent) {
		gt2.Store("hotkey", "triggered")
		gt2.Store("hotkey_combo", event.Chord)
		event.Consume()
	})
	gt2.OnChat(gt2.ChatFilter{Contains: "ping"}, onPing)
	gt2.OnChat(gt2.ChatFilter{Contains: "structured"}, onStructuredChat)
	gt2.OnServerMessage(gt2.ServerMessageFilter{Contains: "structured server"}, onStructuredServer)
	gt2.OnLogin(onLogin)
	gt2.OnLogout(onLogout)
	gt2.OnCharacterChange(onCharacterChange)
	gt2.OnStop(onStop)
	gt2.OnChange(gt2.ChangeVitals, onVitals)
	gt2.OnServerMessage(gt2.ServerMessageFilter{Contains: "ready"}, onReady)
	gt2.SetInputText("test-in")
	gt2.Store("started", "yes")
}

func onPing(event gt2.ChatEvent) {
	gt2.Store("chat", "ping")
	gt2.Print("chat:" + event.Message)
}

func onStructuredChat(event gt2.ChatEvent) {
	gt2.Store("structured_speaker", event.Speaker)
	gt2.Store("structured_message", event.Message)
}

func onStructuredServer(event gt2.ServerMessage) {
	gt2.Store("server_message", event.Message)
	gt2.Store("server_type", event.Type)
}

func onLogin(event gt2.LifecycleEvent)  { gt2.Store("login_character", event.Character) }
func onLogout(event gt2.LifecycleEvent) { gt2.Store("logout_character", event.Character) }
func onCharacterChange(event gt2.LifecycleEvent) {
	gt2.Store("previous_character", event.PreviousCharacter)
}
func onStop(event gt2.LifecycleEvent) { gt2.Store("stop_reason", event.Reason) }
func onVitals(event gt2.ChangeEvent)  { gt2.Store("health", event.Health) }
func onReady(event gt2.ServerMessage) { gt2.Store("console", "ready") }
