package eui

import (
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"math"
	"testing"
)

func TestTextWindowComposedFaceLayoutAndInvalidation(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	win, list, input := NewTextWindow("Composed", HZoneLeft, VZoneTop, true)
	defer win.RemoveWindow()
	var cache TextWindowWrapCache
	for _, size := range []float64{14, 34} {
		face, err := text.NewMultiFace(&text.GoTextFace{Source: FontSource(), Size: size})
		if err != nil {
			t.Fatal(err)
		}
		options := TextWindowOptions{FontSize: 12, Face: face, FirstChanged: 1, InputText: "draft"}
		UpdateTextWindow(win, list, input, []string{"message"}, options, &cache)
		if list.Contents[0].Face != face || input.Contents[0].Face != face {
			t.Fatal("composed face did not reach both rows and input")
		}
		metrics := face.Metrics()
		want := float32(math.Ceil(metrics.HAscent+metrics.HDescent+2)) / UIScale()
		if list.Contents[0].Size.Y != want {
			t.Fatalf("row height = %v, want %v", list.Contents[0].Size.Y, want)
		}
	}
}

func findCompositeItem(items []*ItemData, match func(*ItemData) bool) *ItemData {
	for _, item := range items {
		if match(item) {
			return item
		}
		if found := findCompositeItem(item.Contents, match); found != nil {
			return found
		}
	}
	return nil
}

func TestIndependentColorPickersAndTemporaryWindowCleanup(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	before := len(Windows())
	initial := NewColor(22, 77, 153, 200)
	var applied []Color
	first := ShowColorPicker("First", initial, func(c Color) { applied = append(applied, c) })
	second := ShowColorPicker("Second", initial, nil)
	t.Cleanup(first.RemoveWindow)
	t.Cleanup(second.RemoveWindow)
	if !first.IsOpen() || !second.IsOpen() {
		t.Fatal("opening a second picker closed the first")
	}
	wheel := findCompositeItem(second.Contents, func(i *ItemData) bool { return i.ItemType == ITEM_COLORWHEEL })
	wheel.OnColorChange(NewColor(200, 10, 20, 255))
	cancel := findCompositeItem(second.Contents, func(i *ItemData) bool { return i.Text == "Cancel" })
	cancel.Handler.Emit(UIEvent{Type: EventClick})
	if len(applied) != 0 || len(Windows()) != before+1 {
		t.Fatal("cancel applied a color or retained the window")
	}
	apply := findCompositeItem(first.Contents, func(i *ItemData) bool { return i.Text == "Apply" })
	apply.Handler.Emit(UIEvent{Type: EventClick})
	if len(applied) != 1 || applied[0] != initial || len(Windows()) != before {
		t.Fatal("apply changed the exact initial color or retained the window")
	}
}

func TestPopupActionCanOpenAnotherPopup(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	before := len(Windows())
	var next *WindowData
	win := ShowPopup("First", "Message", []PopupButton{{Text: "Next", Action: func() { next = ShowPopup("Next", "", nil) }}})
	t.Cleanup(win.RemoveWindow)
	button := findCompositeItem(win.Contents, func(i *ItemData) bool { return i.Text == "Next" })
	button.Handler.Emit(UIEvent{Type: EventClick})
	if next == nil {
		t.Fatal("action was not called")
	}
	t.Cleanup(next.RemoveWindow)
	if win.IsOpen() || !next.IsOpen() || len(Windows()) != before+1 {
		t.Fatal("popup lifecycle lost replacement or retained original")
	}
}

func TestTextWindowApplicationHooksAndIncrementalRows(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	win, list, input := NewTextWindow("Log", HZoneLeft, VZoneTop, true)
	t.Cleanup(win.RemoveWindow)
	var cache TextWindowWrapCache
	var opened, annotated string
	options := TextWindowOptions{
		FontSize: 12, InputText: "hello", InputEditable: true,
		OnURLClick:      func(url string) { opened = url },
		InputUnderlines: func(wrapped string) []TextSpan { annotated = wrapped; return []TextSpan{{Start: 0, End: 2}} },
	}
	UpdateTextWindow(win, list, input, []string{"first"}, options, &cache)
	first := list.Contents[0]
	first.OnURLClick("https://example.com")
	if opened != "https://example.com" || annotated != "hello" || !input.Contents[0].EditableText || len(input.Contents[0].Underlines) != 1 {
		t.Fatal("application callbacks or input options not applied")
	}
	options.FirstChanged = 1
	UpdateTextWindow(win, list, input, []string{"first", "second"}, options, &cache)
	if len(list.Contents) != 2 || list.Contents[0] != first {
		t.Fatal("append replaced unchanged row")
	}
	options.FirstChanged = 0
	options.InputUnderlines = nil
	options.OnURLClick = nil
	options.InputEditable = false
	UpdateTextWindow(win, list, input, nil, options, nil)
	if len(list.Contents) != 0 || len(input.Contents[0].Underlines) != 0 || input.Contents[0].EditableText {
		t.Fatal("removed content or options retained old state")
	}
}

func TestConvenienceControlsFitTheirLabels(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	short := NewLabel("Ready")
	long := NewLabel("A UI with no external assets")
	if long.GetSize().X <= short.GetSize().X || long.GetSize().Y > 32*UIScale() {
		t.Fatal("labels did not measure their content with compact height")
	}
	button := NewActionButton("A button with a substantially longer caption", nil)
	label := NewLabel(button.Text)
	if button.GetSize().X < label.GetSize().X {
		t.Fatal("button clips its caption")
	}
}
