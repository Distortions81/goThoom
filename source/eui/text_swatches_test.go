package eui

import "testing"

func TestTextSwatchesKeepSourceAndRevealForEditing(t *testing.T) {
	item := editingFixture(t, "é\t\"#11223344\" tail", true)
	span := TextColorSpan{Start: 3, End: 12, Color: NewColor(17, 34, 51, 68)}
	clicks, scans := 0, 0
	item.SetTextColorSwatches(func(value string) []TextColorSpan {
		scans++
		return []TextColorSpan{span}
	}, func(got TextColorSpan) {
		clicks++
		if got != span {
			t.Fatal("wrong swatch clicked")
		}
	})
	layout := item.editLayout()
	_, origin := item.editGeometry()
	r := textSwatchRect(span, layout.lines[0], layout, origin, origin.Y)
	pos := point{X: (r.X0 + r.X1) / 2, Y: (r.Y0 + r.Y1) / 2}
	before := item.editSnapshot()
	if _, ok := item.textSwatchAt(pos); !ok {
		t.Fatal("swatch hit test missed after Unicode and a tab")
	}
	item.clickItem(pos, true)
	if clicks != 1 || item.editSnapshot() != before || item.CanUndo() {
		t.Fatal("opening a swatch changed source or history")
	}
	item.editSelect(span.Start, span.End)
	if item.SelectedText() != "#11223344" || item.textSwatchVisible(span) {
		t.Fatal("selection did not reveal/copy the original literal")
	}
	item.editSelect(span.Start+1, span.Start+1)
	if _, ok := item.textSwatchAt(pos); ok {
		t.Fatal("swatch obscured a value being edited")
	}
	if scans != 1 {
		t.Fatalf("unchanged swatches rescanned %d times", scans)
	}
	item.ReplaceText(item.Text + "!")
	item.textSwatches()
	if scans != 2 {
		t.Fatal("changed source did not refresh swatches")
	}
	item.Disabled = true
	if len(item.textSwatches()) != 0 {
		t.Fatal("disabled input exposed interactive swatches")
	}
	item.Disabled, item.HideText = false, true
	if len(item.textSwatches()) != 0 || scans != 2 {
		t.Fatal("password content was passed to swatches")
	}
}

func TestTextSwatchesIgnoreInvalidRanges(t *testing.T) {
	item := editingFixture(t, "first\nsecond", true)
	item.SetTextColorSwatches(func(string) []TextColorSpan {
		return []TextColorSpan{{Start: -1, End: 2}, {Start: 3, End: 9}, {Start: 20, End: 24}, {Start: 6, End: 12}}
	}, nil)
	got := item.textSwatches()
	if len(got) != 1 || got[0].Start != 6 || got[0].End != 12 {
		t.Fatalf("invalid ranges were retained: %+v", got)
	}
}
