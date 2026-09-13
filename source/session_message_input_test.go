package main

import "testing"

func TestSessionMessageDraftsAndHistoryAreIndependent(t *testing.T) {
	first := mustNewSession(1)
	second := mustNewSession(2)
	first.input.storeMessage(sessionMessageInput{
		draft: []rune("first draft"), draftPos: 5,
		history: []string{"first command"}, historyPos: 1, active: true,
	})
	second.input.storeMessage(sessionMessageInput{
		draft: []rune("second draft"), draftPos: 12,
		history: []string{"second command"}, historyPos: 0,
	})

	firstState := first.input.messageSnapshot()
	secondState := second.input.messageSnapshot()
	if got := string(firstState.draft); got != "first draft" {
		t.Fatalf("first draft = %q", got)
	}
	if got := string(secondState.draft); got != "second draft" {
		t.Fatalf("second draft = %q", got)
	}
	if len(firstState.history) != 1 || firstState.history[0] != "first command" ||
		len(secondState.history) != 1 || secondState.history[0] != "second command" {
		t.Fatalf("histories crossed: first=%v second=%v", firstState.history, secondState.history)
	}
	firstState.draft[0] = 'X'
	firstState.history[0] = "changed"
	if got := first.input.messageSnapshot(); string(got.draft) != "first draft" || got.history[0] != "first command" {
		t.Fatal("message snapshot aliases session-owned data")
	}
}

func TestQueueSessionInputTargetsOnlyOwningSession(t *testing.T) {
	first := mustNewSession(2)
	second := mustNewSession(3)
	queueSessionInput(first, inputState{mouseX: 11, mouseY: 12, mouseDown: true})

	if got := first.input.next(); got.mouseX != 11 || got.mouseY != 12 || !got.mouseDown {
		t.Fatalf("first input = %+v", got)
	}
	if got := second.input.next(); got != (inputState{}) {
		t.Fatalf("second input changed: %+v", got)
	}
}

func TestScriptInputTextTargetsOwningSession(t *testing.T) {
	first := mustNewSession(2)
	second := mustNewSession(3)
	scriptSetInputTextForSession(first, "first")
	scriptSetInputTextForSession(second, "second")

	if got := scriptInputTextForSession(first); got != "first" {
		t.Fatalf("first input text = %q", got)
	}
	if got := scriptInputTextForSession(second); got != "second" {
		t.Fatalf("second input text = %q", got)
	}
}
