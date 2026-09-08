package main

import "gothoom/eui"

// Both message windows edit the same outgoing line. Remember which bar owns
// its cursor/selection so a click in Chat cannot be overwritten by Console.
var selectedMessageInput *eui.ItemData

func messageInputItem(flow *eui.ItemData) *eui.ItemData {
	if flow == nil || len(flow.Contents) == 0 {
		return nil
	}
	return flow.Contents[0]
}

func currentMessageInputItem() *eui.ItemData {
	if chatWin != nil && chatWin.IsOpen() && (selectedMessageInput == chatInputFlow || consoleWin == nil || !consoleWin.IsOpen()) {
		if item := messageInputItem(chatInputFlow); item != nil {
			return item
		}
	}
	return messageInputItem(inputFlow)
}

func captureMessageInputFocus() bool {
	for _, flow := range []*eui.ItemData{inputFlow, chatInputFlow} {
		if item := messageInputItem(flow); item != nil && item.Focused {
			if item.ParentWindow != nil && !item.ParentWindow.IsOpen() {
				continue
			}
			changed := selectedMessageInput != flow || !inputActive
			selectedMessageInput = flow
			if !inputActive {
				item.Text = string(inputText)
				item.CursorPos = wrappedCursorPos(item.Text, inputPos)
			}
			inputActive = true
			return changed
		}
	}
	return false
}

func messageInputText() string {
	if !inputActive {
		return "[Press Enter To Type]"
	}
	if chatComposing {
		return chatComposition
	}
	return string(inputText)
}

func updateMessageInputPresentation(flow *eui.ItemData) {
	if item := messageInputItem(flow); item != nil {
		item.Focused = inputActive && item == currentMessageInputItem()
		item.CursorPos = wrappedCursorPos(item.Text, inputPos)
		item.Prediction = ""
		if gs.InputAutocomplete && inputActive && !chatComposing {
			item.Prediction = currentInputCompletionSuffix(string(inputText), inputPos)
		}
	}
}

func updateMessageInputWindows() {
	// Each window wraps and spellchecks independently at its own width.
	checkSpelling := spellDirty
	updateConsoleWindow()
	spellDirty = checkSpelling
	updateChatWindow()
}
