package main

import (
	"testing"
	"time"

	scriptapi "gt2"
)

func TestSessionScriptCommandsUseOnlyOwningRuntime(t *testing.T) {
	first := mustNewSession(2)
	second := mustNewSession(3)
	owner := "session-command-test"
	firstQueue := startSessionScriptEventQueue(first, owner)
	secondQueue := startSessionScriptEventQueue(second, owner)
	t.Cleanup(func() {
		stopSessionScriptEventQueue(first, owner)
		stopSessionScriptEventQueue(second, owner)
	})

	firstCalls := make(chan string, 1)
	secondCalls := make(chan string, 1)
	firstRegistration := first.registerSessionScriptCommand(owner, "hello", func(args string) { firstCalls <- args }, firstQueue)
	secondRegistration := second.registerSessionScriptCommand(owner, "hello", func(args string) { secondCalls <- args }, secondQueue)
	if !firstRegistration.valid() || !secondRegistration.valid() {
		t.Fatal("same command was not registered independently in both sessions")
	}

	if !dispatchSessionLocalCommand(first, "/hello one") {
		t.Fatal("first session command was not handled")
	}
	select {
	case got := <-firstCalls:
		if got != "one" {
			t.Fatalf("first args = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("first session handler did not run")
	}
	select {
	case got := <-secondCalls:
		t.Fatalf("first command reached second session: %q", got)
	default:
	}

	if !dispatchSessionLocalCommand(second, "/hello two") {
		t.Fatal("second session command was not handled")
	}
	select {
	case got := <-secondCalls:
		if got != "two" {
			t.Fatalf("second args = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("second session handler did not run")
	}
}

func TestSessionScriptHotkeysUseOnlyOwningRuntime(t *testing.T) {
	first := mustNewSession(2)
	second := mustNewSession(3)
	owner := "session-hotkey-test"
	firstQueue := startSessionScriptEventQueue(first, owner)
	secondQueue := startSessionScriptEventQueue(second, owner)
	t.Cleanup(func() {
		stopSessionScriptEventQueue(first, owner)
		stopSessionScriptEventQueue(second, owner)
	})

	firstCalls := make(chan struct{}, 1)
	secondCalls := make(chan struct{}, 1)
	firstRegistration := first.registerSessionScriptHotkey(owner, "F8", func(event InputEvent) {
		firstCalls <- struct{}{}
		event.Consume()
	}, firstQueue)
	secondRegistration := second.registerSessionScriptHotkey(owner, "F8", func(InputEvent) {
		secondCalls <- struct{}{}
	}, secondQueue)
	if !firstRegistration.valid() || !secondRegistration.valid() {
		t.Fatal("same hotkey was not registered independently in both sessions")
	}

	firstHotkey, matched, enabled := first.sessionScriptHotkey("F8")
	if !matched || !enabled || firstHotkey.handler == nil {
		t.Fatal("first session hotkey was not found")
	}
	event := InputEvent{decision: &inputEventDecision{continueInput: true}}
	if firstHotkey.handler(event) {
		t.Fatal("consumed first-session hotkey continued")
	}
	select {
	case <-firstCalls:
	case <-time.After(time.Second):
		t.Fatal("first session hotkey did not run")
	}
	select {
	case <-secondCalls:
		t.Fatal("first session hotkey reached second runtime")
	default:
	}
}

func TestSessionScriptToolbarUsesOwningRuntime(t *testing.T) {
	session := mustNewSession(2)
	owner := "session-toolbar-test"
	queue := startSessionScriptEventQueue(session, owner)
	t.Cleanup(func() { stopSessionScriptEventQueue(session, owner) })

	clicked := make(chan struct{}, 1)
	registration := session.registerSessionScriptToolbar(owner, scriptapi.ToolbarOptions{
		Label: "Second tools",
		Buttons: []scriptapi.ToolbarButton{{
			Label: "Do it", OnClick: func() { clicked <- struct{}{} },
		}},
	}, nil, queue)
	if !registration.valid() {
		t.Fatal("secondary toolbar registration is inactive")
	}
	session.automation.scriptMu.RLock()
	toolbars := append([]*scriptToolbarRegistration(nil), session.automation.localToolbars[owner]...)
	session.automation.scriptMu.RUnlock()
	if len(toolbars) != 1 || len(toolbars[0].buttons) != 1 {
		t.Fatalf("secondary toolbars = %#v", toolbars)
	}
	toolbars[0].buttons[0].onClick()
	select {
	case <-clicked:
	case <-time.After(time.Second):
		t.Fatal("secondary toolbar callback did not reach its runtime")
	}
}
