package main

import (
	"image"
	"log"
	"runtime"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2/exp/textinput"
)

type nativeChatField interface {
	Focus()
	Blur()
	HandleInputWithBounds(image.Rectangle) (bool, error)
	Text() string
	Selection() (int, int)
	SetTextAndSelection(string, int, int)
	UncommittedTextLengthInBytes() int
	TextForRendering() string
}

var chatTextField nativeChatField = &textinput.Field{}
var nativeChatPlatform = runtime.GOOS == "darwin"
var chatComposition string
var chatComposing bool
var nativeChatFailed bool

func nativeChatEnabled() bool { return nativeChatPlatform && !nativeChatFailed }

type nativeChatEdit struct {
	text         string
	cursor       int
	handled      bool
	changed      bool
	composing    bool
	wasComposing bool
}

// pollNativeChatInput uses Cocoa's text input client for dictation, accents and
// IME commits. AppendInputChars alone does not establish a native text field.
// Keep the committed text separate from the composition displayed in the bar.
func pollNativeChatInput(active bool) nativeChatEdit {
	result := nativeChatEdit{text: string(inputText), cursor: inputPos, wasComposing: chatComposing}
	if !nativeChatEnabled() {
		return result
	}
	if !active {
		chatTextField.Blur()
		chatComposition, chatComposing = "", false
		return result
	}
	syncNativeChatInput()
	chatTextField.Focus()
	bounds := image.Rect(0, 0, 1, 20)
	if item := currentMessageInputItem(); item != nil {
		r := item.DrawRect
		bounds = image.Rect(int(r.X0), int(r.Y0), int(r.X0)+1, max(int(r.Y0)+1, int(r.Y1)))
	}
	handled, err := chatTextField.HandleInputWithBounds(bounds)
	if err != nil {
		log.Printf("native chat input: %v", err)
		chatTextField.Blur()
		nativeChatFailed = true
		chatComposition, chatComposing = "", false
		return result
	}
	result.changed = chatTextField.Text() != result.text
	result.text = chatTextField.Text()
	_, end := chatTextField.Selection()
	result.cursor = utf8.RuneCountInString(result.text[:end])
	result.handled = handled
	chatComposing = chatTextField.UncommittedTextLengthInBytes() > 0
	chatComposition = chatTextField.TextForRendering()
	result.composing = chatComposing
	return result
}

// Do not reset a live composition every frame. Only synchronize actual edits
// from clipboard actions, history, macros or cursor movement.
func syncNativeChatInput() {
	if !nativeChatEnabled() {
		return
	}
	text := string(inputText)
	pos := len(string(inputText[:max(0, min(inputPos, len(inputText)))]))
	start, end := chatTextField.Selection()
	if chatTextField.Text() != text || start != pos || end != pos {
		chatTextField.SetTextAndSelection(text, pos, pos)
	}
}

// Native replacements (including dictation corrections) can replace existing
// text. Plain insertions still pass through the usual macro boundary handling.
func nativeChatInsertion(before []rune, cursor int, after string) ([]rune, bool) {
	cursor = max(0, min(cursor, len(before)))
	runes := []rune(after)
	added := len(runes) - len(before)
	if added < 0 || string(runes[:cursor]) != string(before[:cursor]) ||
		string(runes[cursor+added:]) != string(before[cursor:]) {
		return nil, false
	}
	return runes[cursor : cursor+added], true
}
