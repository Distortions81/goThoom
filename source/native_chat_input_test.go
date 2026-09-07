package main

import (
	"image"
	"testing"
)

type fakeNativeChatField struct {
	text, composition string
	start, end        int
	sets              int
	focused           bool
	input             func()
}

func (f *fakeNativeChatField) Focus() { f.focused = true }
func (f *fakeNativeChatField) Blur()  { f.focused = false; f.composition = "" }
func (f *fakeNativeChatField) HandleInputWithBounds(image.Rectangle) (bool, error) {
	if f.input == nil {
		return false, nil
	}
	f.input()
	f.input = nil
	return true, nil
}
func (f *fakeNativeChatField) Text() string          { return f.text }
func (f *fakeNativeChatField) Selection() (int, int) { return f.start, f.end }
func (f *fakeNativeChatField) SetTextAndSelection(s string, start, end int) {
	f.text, f.start, f.end = s, start, end
	f.composition = ""
	f.sets++
}
func (f *fakeNativeChatField) UncommittedTextLengthInBytes() int { return len(f.composition) }
func (f *fakeNativeChatField) TextForRendering() string {
	return f.text[:f.start] + f.composition + f.text[f.end:]
}

func TestNativeChatCompositionAndReplacement(t *testing.T) {
	oldField, oldPlatform, oldFailed := chatTextField, nativeChatPlatform, nativeChatFailed
	oldText, oldPos := inputText, inputPos
	oldComposition, oldComposing := chatComposition, chatComposing
	t.Cleanup(func() {
		chatTextField, nativeChatPlatform, nativeChatFailed = oldField, oldPlatform, oldFailed
		inputText, inputPos = oldText, oldPos
		chatComposition, chatComposing = oldComposition, oldComposing
	})
	f := &fakeNativeChatField{}
	chatTextField, nativeChatPlatform, nativeChatFailed = f, true, false
	chatComposition, chatComposing = "", false
	inputText, inputPos = []rune("café here"), 4
	f.input = func() { f.composition = "世界" }
	edit := pollNativeChatInput(true)
	if edit.changed || edit.text != "café here" || !edit.composing || chatComposition != "café世界 here" || f.start != len("café") {
		t.Fatalf("composition changed committed text or used rune offsets as bytes: %+v, field=%+v", edit, f)
	}
	sets := f.sets
	syncNativeChatInput()
	pollNativeChatInput(true)
	if f.sets != sets || f.composition != "世界" {
		t.Fatal("idle frames reset the native composition")
	}
	f.input = func() {
		f.text, f.composition = "café世界 here", ""
		f.start, f.end = len("café世界"), len("café世界")
	}
	edit = pollNativeChatInput(true)
	if edit.cursor != 6 || edit.composing || !edit.wasComposing || !edit.handled || !edit.changed {
		t.Fatalf("commit must preserve Unicode cursor and reserve confirmation key: %+v", edit)
	}
	inputText, inputPos = []rune(edit.text), edit.cursor
	syncNativeChatInput()
	if f.sets != sets {
		t.Fatal("accepted commit was unnecessarily reset")
	}
	if idle := pollNativeChatInput(true); idle.changed {
		t.Fatal("an idle native field must not overwrite later script edits")
	}
	// A dictation service may correct text before the cursor.
	f.input = func() { f.text = "hello here"; f.start, f.end = 5, 5 }
	edit = pollNativeChatInput(true)
	if edit.text != "hello here" || edit.cursor != 5 {
		t.Fatalf("replacement lost: %+v", edit)
	}
	pollNativeChatInput(false)
	if f.focused || chatComposing || chatComposition != "" {
		t.Fatal("inactive chat kept native focus/composition")
	}
}

func TestNativeChatInsertion(t *testing.T) {
	for _, tc := range []struct {
		before           string
		cursor           int
		after, insertion string
		ok               bool
	}{
		{"hello world", 6, "hello brave world", "brave ", true},
		{"café!", 4, "café世界!", "世界", true},
		{"", 0, "dictated phrase", "dictated phrase", true},
		{"hello", 5, "hello", "", true},
		{"hello", 5, "help", "", false},
		{"abc", 1, "xyzabc", "", false},
	} {
		got, ok := nativeChatInsertion([]rune(tc.before), tc.cursor, tc.after)
		if ok != tc.ok || string(got) != tc.insertion {
			t.Errorf("%q -> %q: got %q, %v", tc.before, tc.after, got, ok)
		}
	}
}
