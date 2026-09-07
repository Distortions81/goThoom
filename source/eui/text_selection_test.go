package eui

import "testing"

func TestHasTextSelection(t *testing.T) {
	old := selectedTextItem
	t.Cleanup(func() { selectedTextItem = old })
	selectedTextItem = &itemData{Text: "café", SelectStart: 0, SelectEnd: 4}
	if !HasTextSelection() {
		t.Fatal("selected text must take priority over copying the whole input")
	}
	selectedTextItem.SelectEnd = 0
	if HasTextSelection() {
		t.Fatal("an empty selection must allow copying the whole input")
	}
	selectedTextItem = nil
	if HasTextSelection() {
		t.Fatal("nil selection must allow copying the whole input")
	}
}

func TestSelectedText(t *testing.T) {
	item := &itemData{Text: "one two", SelectStart: 7, SelectEnd: 3}
	if got, want := item.SelectedText(), " two"; got != want {
		t.Fatalf("SelectedText() = %q, want %q", got, want)
	}
}

func TestSelectedTextClampsToText(t *testing.T) {
	item := &itemData{Text: "café", SelectStart: -2, SelectEnd: 99}
	if got, want := item.SelectedText(), "café"; got != want {
		t.Fatalf("SelectedText() = %q, want %q", got, want)
	}
}

func TestFilledSelectableTextRemainsReadOnly(t *testing.T) {
	item := &itemData{ItemType: ITEM_TEXT, Filled: true, SelectableText: true}
	if itemAcceptsTextEditing(item) {
		t.Fatal("filled read-only text was treated as editable")
	}
	item.EditableText = true
	if !itemAcceptsTextEditing(item) {
		t.Fatal("explicitly editable text was treated as read-only")
	}
}
