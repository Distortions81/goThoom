package main

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"gothoom/eui"
)

func TestMessageInputCaretSurvivesFramesWithoutKeys(t *testing.T) {
	if err := eui.EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	oldConsole, oldChat := consoleWin, chatWin
	oldInput, oldChatInput, oldSelected := inputFlow, chatInputFlow, selectedMessageInput
	oldText, oldPos, oldActive := inputText, inputPos, inputActive
	oldAutocomplete := gs.InputAutocomplete
	oldFocus := eui.FocusedTextInput()
	t.Cleanup(func() {
		if focused := eui.FocusedTextInput(); focused != nil {
			eui.ClearFocus(focused)
		}
		if oldFocus != nil {
			eui.Focus(oldFocus)
		}
		consoleWin, chatWin = oldConsole, oldChat
		inputFlow, chatInputFlow, selectedMessageInput = oldInput, oldChatInput, oldSelected
		inputText, inputPos, inputActive = oldText, oldPos, oldActive
		gs.InputAutocomplete = oldAutocomplete
	})
	gs.InputAutocomplete = false
	consoleWin, chatWin = eui.NewWindow(), eui.NewWindow()
	consoleWin.Open, chatWin.Open = true, true
	inputFlow, chatInputFlow = eui.NewColumn(), eui.NewColumn()
	for _, pair := range []struct {
		win  *eui.WindowData
		flow *eui.ItemData
	}{{consoleWin, inputFlow}, {chatWin, chatInputFlow}} {
		item, _ := eui.NewText()
		item.Text = "hello world"
		item.EditableText, item.ExternalTextEditing = true, true
		pair.flow.AddItem(item)
		pair.win.AddItem(pair.flow)
	}
	inputText, inputActive = []rune("hello world"), true
	for _, clicked := range []*eui.ItemData{inputFlow, chatInputFlow} {
		// The click selects a bar and places its caret without changing text.
		item := messageInputItem(clicked)
		eui.Focus(item)
		item.CursorPos = 3
		captureMessageInputFocus()
		inputPos = plainCursorPos(item.Text, item.CursorPos)
		for range 3 {
			captureMessageInputFocus()
			updateMessageInputPresentation(inputFlow)
			updateMessageInputPresentation(chatInputFlow)
			if !item.Focused || item.CursorPos != 3 || currentMessageInputItem() != item {
				t.Fatal("clicked caret disappeared or moved to the other bar")
			}
		}
		item.CursorPos = 0
		item.Dirty, item.ParentWindow.Dirty = false, false
		updateMessageInputPresentation(clicked)
		if !item.Dirty || !item.ParentWindow.Dirty {
			t.Fatal("caret change did not invalidate cached rendering")
		}
		item.Dirty, item.ParentWindow.Dirty = false, false
		updateMessageInputPresentation(clicked)
		if item.Dirty || item.ParentWindow.Dirty {
			t.Fatal("unchanged presentation requested another repaint")
		}
	}
}

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
