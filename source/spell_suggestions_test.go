package main

import (
	"math"
	"reflect"
	"testing"

	"github.com/f1monkey/spellchecker"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"gothoom/eui"
)

func TestRightClickSpellSuggestions(t *testing.T) {
	initFont()
	oldSC, oldCache, oldDirty := sc, spellCache, spellDirty
	oldEnabled, oldMenu := gs.InputSpellcheck, showContextMenu
	oldInput, oldPos, oldActive := inputText, inputPos, inputActive
	oldSelected := selectedMessageInput
	oldConsole, oldChat := consoleWin, chatWin
	consoleWin, chatWin = nil, nil
	t.Cleanup(func() {
		sc, spellCache, spellDirty = oldSC, oldCache, oldDirty
		gs.InputSpellcheck, showContextMenu = oldEnabled, oldMenu
		inputText, inputPos, inputActive = oldInput, oldPos, oldActive
		selectedMessageInput = oldSelected
		consoleWin, chatWin = oldConsole, oldChat
		eui.CloseContextMenus()
	})
	var err error
	sc, err = spellchecker.New("abcdefghijklmnopqrstuvwxyz'", spellchecker.WithMaxErrors(1))
	if err != nil {
		t.Fatal(err)
	}
	sc.Add("hello", "world")
	spellCache = map[string]bool{}
	gs.InputSpellcheck = true
	inputText = []rune("helo world")

	win := eui.NewWindow()
	flow := eui.NewColumn()
	txt := eui.NewLabel(string(inputText))
	txt.Face = &text.GoTextFace{Source: eui.FontSource(), Size: 14}
	txt.Underlines = findMisspellings(txt.Text)
	flow.AddItem(txt)
	win.AddItem(flow)
	win.MarkOpen()
	t.Cleanup(win.RemoveWindow)
	metrics := txt.Face.Metrics()
	lineHeight := float32(math.Ceil(metrics.HAscent + metrics.HDescent + 2))
	w, _ := text.Measure(txt.Text, txt.Face, 0)
	txt.DrawRect = eui.Rect{X0: 20, Y0: 20, X1: 20 + float32(w), Y1: 20 + lineHeight}
	var menu *eui.ItemData
	showContextMenu = func(opts []string, x, y float32, selectOption func(int)) *eui.ItemData {
		menu = eui.ShowContextMenu(opts, x, y, selectOption)
		return menu
	}
	if !handleMessageInputContext(win, flow, 24, 24) || menu == nil {
		t.Fatal("right-click did not open spelling suggestions")
	}
	if !reflect.DeepEqual(menu.Options, suggestCorrections("helo", 5)) {
		t.Fatalf("unexpected suggestions: %v", menu.Options)
	}
	if string(inputText) != "helo world" {
		t.Fatal("opening suggestions changed the draft")
	}
	choice := -1
	for i, option := range menu.Options {
		if option == "hello" {
			choice = i
		}
	}
	if choice < 0 {
		t.Fatal("missing hello suggestion")
	}
	menu.OnSelect(choice)
	if string(inputText) != "hello world" || txt.Text != "hello world" || eui.ContextMenusOpen() {
		t.Fatal("selecting a correction did not replace the word and close the menu")
	}

	txt.Text, inputText = "helo world", []rune("helo world")
	txt.Underlines = findMisspellings(txt.Text)
	handleMessageInputContext(win, flow, 24, 24)
	inputText = []rune("new draft")
	menu.OnSelect(choice)
	if string(inputText) != "new draft" {
		t.Fatal("old suggestions overwrote a newer draft")
	}
	gs.InputSpellcheck = false
	if showSpellSuggestions(txt, 24, 24) {
		t.Fatal("disabled spellcheck showed suggestions")
	}
}
