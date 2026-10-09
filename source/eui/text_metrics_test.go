package eui

import (
	"bytes"
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

func TestTextSizeTracksTextFontAndScale(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	oldScale, oldSource := UIScale(), FontSource()
	t.Cleanup(func() { SetFontSource(oldSource); SetUIScale(oldScale) })
	SetUIScale(1)
	item := NewLabel("Ready")
	check := func() {
		t.Helper()
		face := textFace(item.FontSize*UIScale() + 2)
		width, _ := text.Measure(item.Text, face, 0)
		metrics := face.Metrics()
		want := Point{X: float32(math.Ceil(width)), Y: float32(math.Ceil(metrics.HAscent + metrics.HDescent + 2))}
		if got := item.GetSize(); got != want {
			t.Fatalf("text size = %+v, want %+v", got, want)
		}
		if got := item.GetSize(); got != want {
			t.Fatalf("repeated text size = %+v, want %+v", got, want)
		}
	}
	check()
	item.Text = "A longer status message"
	check()
	item.FontSize = 18
	check()
	SetUIScale(1.5)
	check()
	source, err := text.NewGoTextFaceSource(bytes.NewReader(gobold.TTF))
	if err != nil {
		t.Fatal(err)
	}
	SetFontSource(source)
	check()
}

func TestTextSizeKeepsMultilineMinimumAndConstraintBehavior(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	oldScale := UIScale()
	t.Cleanup(func() { SetUIScale(oldScale) })
	SetUIScale(1)
	item := NewLabel("First\nA wider second line\n")
	face := textFace(14)
	width, _ := text.Measure("A wider second line", face, 0)
	metrics := face.Metrics()
	height := float32(math.Ceil(metrics.HAscent+metrics.HDescent+2)) * 3
	if got := item.GetSize(); got != (Point{X: float32(math.Ceil(width)), Y: height}) {
		t.Fatalf("multiline text size = %+v", got)
	}
	item.ConstrainToSize = true
	if got := item.GetSize(); got != (Point{X: float32(width), Y: height}) {
		t.Fatalf("constrained auto text size = %+v", got)
	}
	item.Size = Point{X: 20, Y: 25}
	if got := item.GetSize(); got != item.Size {
		t.Fatalf("constrained explicit text size = %+v, want %+v", got, item.Size)
	}
	item.ConstrainToSize = false
	if got := item.GetSize(); got != (Point{X: float32(math.Ceil(width)), Y: 25}) {
		t.Fatalf("unconstrained explicit text size = %+v", got)
	}
	item.Size = Point{}
	item.Text = ""
	if got := item.GetSize(); got != (Point{X: 14, Y: height / 3}) {
		t.Fatalf("empty text size = %+v", got)
	}
}

func TestTextSizeMeasuresMutableCustomFaces(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	oldScale := UIScale()
	t.Cleanup(func() { SetUIScale(oldScale) })
	SetUIScale(1)
	item := NewLabel("Office AVATAR")
	// With an explicit height/width, only the content-growth measurement uses
	// the custom face. Autosizing retains its ordinary-font minimum dimensions.
	item.Size = Point{X: 1, Y: 25}
	face := &text.GoTextFace{Size: 14}
	item.Face = face
	for _, font := range [][]byte{goregular.TTF, gobold.TTF} {
		source, err := text.NewGoTextFaceSource(bytes.NewReader(font))
		if err != nil {
			t.Fatal(err)
		}
		face.Source = source
		width, _ := text.Measure(item.Text, face, 0)
		want := Point{X: float32(math.Ceil(width)), Y: 25}
		if got := item.GetSize(); got != want {
			t.Fatalf("custom face size = %+v, want %+v", got, want)
		}
	}
	face.Size = 30 // A mismatched size falls back to the ordinary face.
	width, _ := text.Measure(item.Text, textFace(14), 0)
	if got := item.GetSize().X; got != float32(math.Ceil(width)) {
		t.Fatalf("mismatched face width = %g, want %g", got, math.Ceil(width))
	}
}

func TestNewTextFaceMutationDoesNotChangeSharedMeasurements(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	oldScale := UIScale()
	t.Cleanup(func() { SetUIScale(oldScale) })
	SetUIScale(1)
	first, _ := NewText()
	second, _ := NewText()
	face := first.Face.(*text.GoTextFace)
	if face == second.Face || face == textFace(first.FontSize) {
		t.Fatal("text constructor exposed a shared mutable face")
	}
	label := NewLabel("Office AVATAR")
	label.FontSize = first.FontSize - 2 // Measure at the exposed face's size.
	want := label.GetSize()
	source, err := text.NewGoTextFaceSource(bytes.NewReader(gobold.TTF))
	if err != nil {
		t.Fatal(err)
	}
	face.Source = source
	face.Size *= 2
	face.SetFeature(text.Tag('k'<<24|'e'<<16|'r'<<8|'n'), 0)
	if got := label.GetSize(); got != want {
		t.Fatalf("mutating another item's face changed cached size from %+v to %+v", want, got)
	}
	// A fresh measurement must agree too, rather than just serving stale data.
	label.textMeasure = nil
	if got := label.GetSize(); got != want {
		t.Fatalf("mutating another item's face changed measured size from %+v to %+v", want, got)
	}
}
