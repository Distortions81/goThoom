package eui

import "testing"

func TestExternalTextEditingRetainsPointerEditingWithoutKeyboardMutation(t *testing.T) {
	item, _ := NewText()
	item.EditableText = true
	item.ExternalTextEditing = true
	if !itemAcceptsTextEditing(item) {
		t.Fatal("externally edited text lost pointer cursor and selection behavior")
	}
	if itemHandlesTextEditing(item) {
		t.Fatal("EUI would also mutate text owned by the embedding application")
	}
	item.ExternalTextEditing = false
	if !itemHandlesTextEditing(item) {
		t.Fatal("ordinary editable text no longer handles keyboard input")
	}
}
