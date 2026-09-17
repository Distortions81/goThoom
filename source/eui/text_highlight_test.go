package eui

import (
	"reflect"
	"testing"
)

func TestTextHighlightCacheTracksEditsAndPalette(t *testing.T) {
	item := editingFixture(t, "café\tvalue", true)
	layout := item.editLayout()
	var seen []string
	color := NewColor(120, 200, 255, 255)
	item.SetTextHighlighter(func(value string) []TextColorSpan {
		seen = append(seen, value)
		return []TextColorSpan{{0, 4, color}}
	})
	item.textHighlights()
	item.textHighlights()
	if len(seen) != 1 || item.editLayout() != layout {
		t.Fatal("coloring was not cached independently of text layout")
	}
	item.editSelect(0, 4)
	if item.SelectedText() != "café" {
		t.Fatal("coloring changed selection offsets")
	}
	item.editInsert("new", "typing")
	item.textHighlights()
	item.editUndo(false)
	item.textHighlights()
	item.editUndo(true)
	item.textHighlights()
	if want := []string{"café\tvalue", "new\tvalue", "café\tvalue", "new\tvalue"}; !reflect.DeepEqual(seen, want) {
		t.Fatalf("highlighted documents = %q, want %q", seen, want)
	}
	before := item.editSnapshot()
	color = NewColor(255, 200, 100, 255)
	item.RefreshTextHighlighting()
	if got := item.textHighlights()[0].Color; got != color || item.editSnapshot() != before || !item.Dirty {
		t.Fatal("palette refresh failed or changed editing state")
	}
	item.Text = "external"
	item.textHighlights()
	if seen[len(seen)-1] != "external" {
		t.Fatal("external replacement retained stale colors")
	}
	item.SetTextHighlighter(nil)
	if item.textHighlights() != nil {
		t.Fatal("disabling highlighting retained colors")
	}
}

func TestTextHighlightRangesAreBoundedAndCopied(t *testing.T) {
	item := editingFixture(t, "éabcdef", true)
	red, blue, normal := ColorRed, ColorBlue, ColorWhite
	spans := []TextColorSpan{{5, 99, blue}, {-2, 2, red}, {1, 4, blue}, {4, 4, red}, {6, 3, red}}
	original := append([]TextColorSpan(nil), spans...)
	item.SetTextHighlighter(func(string) []TextColorSpan { return spans })
	got := item.textHighlights()
	want := []TextColorSpan{{0, 2, red}, {2, 4, blue}, {5, 7, blue}}
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(spans, original) {
		t.Fatalf("normalized spans = %v, input = %v", got, spans)
	}
	spans[0].Color = red
	for pos, want := range []Color{red, red, blue, blue, normal, blue, blue, normal} {
		if got := textHighlightColor(item.textHighlights(), pos, normal); got != want {
			t.Fatalf("color at rune %d = %v, want %v", pos, got, want)
		}
	}
}

func TestTextHighlightSourceOffsetsIncludeUnicodeTabsAndNewlines(t *testing.T) {
	item := editingFixture(t, "é\tβ\n\tcat", true)
	layout := item.editLayout()
	// Display bytes: é(0..2), three spaces(2..5), β(5..7).
	for byteOffset, want := range map[int]int{0: 0, 2: 1, 3: 1, 4: 1, 5: 2, 7: 3} {
		if got := layout.lines[0].sourceRuneAtDisplayByte(byteOffset); got != want {
			t.Fatalf("first line byte %d = source rune %d, want %d", byteOffset, got, want)
		}
	}
	if got := layout.lines[1].sourceRuneAtDisplayByte(4); got != 5 {
		t.Fatalf("second line after tab = %d, want 5", got)
	}
}

func TestTextHighlightSkipsPasswordsAndDisabledControls(t *testing.T) {
	item := editingFixture(t, "secret", false)
	calls := 0
	item.SetTextHighlighter(func(string) []TextColorSpan {
		calls++
		return []TextColorSpan{{0, 6, ColorRed}}
	})
	item.HideText, item.SecretText = true, item.Text
	if len(item.textHighlights()) != 0 || calls != 0 {
		t.Fatal("password reached highlighter")
	}
	item.HideText, item.Disabled = false, true
	if len(item.textHighlights()) != 0 || calls != 0 {
		t.Fatal("disabled text lost its normal foreground")
	}
	item.Disabled = false
	if len(item.textHighlights()) != 1 || calls != 1 {
		t.Fatal("re-enabled control did not highlight")
	}
}
