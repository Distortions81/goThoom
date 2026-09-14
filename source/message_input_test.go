package main

import (
	"testing"

	"gothoom/eui"
)

func TestAlwaysOpenMessageInputNeverUsesInactivePrompt(t *testing.T) {
	oldAlwaysOpen := gs.InputBarAlwaysOpen
	oldText, oldPos, oldActive := inputText, inputPos, inputActive
	oldHistory, oldHistoryPos := inputHistory, historyPos
	t.Cleanup(func() {
		gs.InputBarAlwaysOpen = oldAlwaysOpen
		inputText, inputPos, inputActive = oldText, oldPos, oldActive
		inputHistory, historyPos = oldHistory, oldHistoryPos
	})

	gs.InputBarAlwaysOpen = true
	inputText = []rune("unfinished draft")
	inputPos = len(inputText)
	inputActive = false
	if got := messageInputText(); got != "unfinished draft" {
		t.Fatalf("always-open input displayed %q, want the draft", got)
	}

	inputActive = true
	if clearMessageInputForOtherUI(true, false) {
		t.Fatal("another UI field deactivated an always-open input bar")
	}
	if got := string(inputText); got != "unfinished draft" || !inputActive {
		t.Fatalf("another UI field changed always-open draft %q, active=%v", got, inputActive)
	}
}

func TestMessageInputSelectAllAndReplaceSelection(t *testing.T) {
	oldConsole, oldChat := consoleWin, chatWin
	oldInputFlow, oldChatInputFlow := inputFlow, chatInputFlow
	oldSelected := selectedMessageInput
	oldText, oldPos, oldActive := inputText, inputPos, inputActive
	t.Cleanup(func() {
		consoleWin, chatWin = oldConsole, oldChat
		inputFlow, chatInputFlow = oldInputFlow, oldChatInputFlow
		selectedMessageInput = oldSelected
		inputText, inputPos, inputActive = oldText, oldPos, oldActive
	})

	consoleWin, chatWin = nil, nil
	inputFlow = eui.NewColumn()
	input, _ := eui.NewText()
	input.Text = "Select this\nwrapped draft"
	inputFlow.AddItem(input)
	chatInputFlow = nil
	selectedMessageInput = inputFlow
	inputText = []rune("Select thiswrapped draft")
	inputPos = 6
	inputActive = true

	if !selectAllMessageInput() {
		t.Fatal("select all did not select a non-empty draft")
	}
	if got := selectedMessageInputText(); got != "Select thiswrapped draft" {
		t.Fatalf("selected text = %q", got)
	}
	if !replaceSelectedMessageInput([]rune("replacement")) {
		t.Fatal("selection replacement was not applied")
	}
	if got := string(inputText); got != "replacement" || inputPos != len([]rune("replacement")) {
		t.Fatalf("replacement result = %q at %d", got, inputPos)
	}
	if input.SelectStart != input.SelectEnd {
		t.Fatal("selection did not collapse after replacement")
	}
}
