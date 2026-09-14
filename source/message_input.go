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
	if !inputActive && !gs.InputBarAlwaysOpen {
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

func messageInputSelectionRange() (int, int, bool) {
	item := currentMessageInputItem()
	if item == nil || item.SelectStart == item.SelectEnd {
		return 0, 0, false
	}
	start := plainCursorPos(item.Text, item.SelectStart)
	end := plainCursorPos(item.Text, item.SelectEnd)
	if start > end {
		start, end = end, start
	}
	start = max(0, min(start, len(inputText)))
	end = max(start, min(end, len(inputText)))
	return start, end, start != end
}

func selectedMessageInputText() string {
	start, end, ok := messageInputSelectionRange()
	if !ok {
		return ""
	}
	return string(inputText[start:end])
}

func setMessageInputSelection(start, end int) {
	start = max(0, min(start, len(inputText)))
	end = max(0, min(end, len(inputText)))
	for _, flow := range []*eui.ItemData{inputFlow, chatInputFlow} {
		item := messageInputItem(flow)
		if item == nil {
			continue
		}
		item.SelectStart = wrappedCursorPos(item.Text, start)
		item.SelectEnd = wrappedCursorPos(item.Text, end)
		item.CursorPos = item.SelectEnd
		item.Dirty = true
		if item.ParentWindow != nil {
			item.ParentWindow.Refresh()
		}
	}
}

func selectAllMessageInput() bool {
	if len(inputText) == 0 || currentMessageInputItem() == nil {
		return false
	}
	inputPos = len(inputText)
	setMessageInputSelection(0, len(inputText))
	return true
}

func replaceSelectedMessageInput(replacement []rune) bool {
	start, end, ok := messageInputSelectionRange()
	if !ok {
		return false
	}
	updated := make([]rune, 0, len(inputText)-(end-start)+len(replacement))
	updated = append(updated, inputText[:start]...)
	updated = append(updated, replacement...)
	updated = append(updated, inputText[end:]...)
	inputText = updated
	inputPos = start + len(replacement)
	setMessageInputSelection(inputPos, inputPos)
	return true
}

// clearMessageInputForOtherUI releases the chat input when an unrelated text
// field takes focus. The emoji picker is part of composing the current message:
// its search field must receive keyboard input without discarding or hiding the
// draft that the chosen emoji will be appended to.
func clearMessageInputForOtherUI(typingElsewhere, paletteKeyboardActive bool) bool {
	if !typingElsewhere || !inputActive || paletteKeyboardActive {
		return false
	}
	if gs.InputBarAlwaysOpen {
		return false
	}
	if activeEmojiPicker != nil && activeEmojiPicker.win != nil && activeEmojiPicker.win.IsOpen() {
		return false
	}
	inputActive = false
	inputText = inputText[:0]
	inputPos = 0
	historyPos = len(inputHistory)
	return true
}
