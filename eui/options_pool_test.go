package eui

import (
	"image"
	"reflect"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

func mutateDrawImageOptions(op *ebiten.DrawImageOptions) {
	op.GeoM.Translate(1, 1)
	op.ColorScale.Scale(1, 2, 3, 4)
	op.Filter = ebiten.FilterLinear
	op.CompositeMode = ebiten.CompositeModeSourceOver

	v := reflect.ValueOf(op).Elem()
	if f := v.FieldByName("Address"); f.IsValid() && f.CanSet() {
		f.Set(reflect.ValueOf(ebiten.AddressClampToZero))
	}
	if f := v.FieldByName("SourceRect"); f.IsValid() && f.CanSet() {
		r := image.Rect(1, 2, 3, 4)
		f.Set(reflect.ValueOf(&r))
	}
}

func TestAcquireDrawImageOptions(t *testing.T) {
	op := acquireDrawImageOptions()
	mutateDrawImageOptions(op)
	releaseDrawImageOptions(op)

	op = acquireDrawImageOptions()
	var expected ebiten.DrawImageOptions
	if !reflect.DeepEqual(*op, expected) {
		t.Errorf("acquired options not reset: %+v", op)
	}
}

func TestAcquireTextDrawOptions(t *testing.T) {
	op := acquireTextDrawOptions()
	mutateDrawImageOptions(&op.DrawImageOptions)
	op.LayoutOptions.LineSpacing = 1
	op.LayoutOptions.PrimaryAlign = text.AlignCenter
	releaseTextDrawOptions(op)

	op = acquireTextDrawOptions()
	var expected text.DrawOptions
	if !reflect.DeepEqual(*op, expected) {
		t.Errorf("acquired text options not reset: %+v", op)
	}
}

func BenchmarkAcquireDrawImageOptions(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		op := acquireDrawImageOptions()
		op.GeoM.Translate(1, 1)
		releaseDrawImageOptions(op)
	}
}

func BenchmarkAcquireTextDrawOptions(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		op := acquireTextDrawOptions()
		op.DrawImageOptions.GeoM.Translate(1, 1)
		releaseTextDrawOptions(op)
	}
}

func TestDrawImageOptionsPoolResets(t *testing.T) {
	op := acquireDrawImageOptions()
	op.GeoM.Translate(1, 1)
	op.ColorScale.Scale(0.5, 0.25, 0.75, 0.5)
	releaseDrawImageOptions(op)

	op = acquireDrawImageOptions()
	if tx, ty := op.GeoM.Apply(0, 0); tx != 0 || ty != 0 {
		t.Fatalf("GeoM not reset: translation (%f,%f)", tx, ty)
	}
	if op.ColorScale.R() != 1 || op.ColorScale.G() != 1 || op.ColorScale.B() != 1 || op.ColorScale.A() != 1 {
		t.Fatalf("ColorScale not reset: %v", op.ColorScale)
	}
	releaseDrawImageOptions(op)
}

func TestTextDrawOptionsPoolResets(t *testing.T) {
	op := acquireTextDrawOptions()
	op.DrawImageOptions.GeoM.Translate(1, 1)
	op.DrawImageOptions.ColorScale.Scale(0.5, 0.25, 0.75, 0.5)
	op.LayoutOptions = text.LayoutOptions{
		LineSpacing:    1,
		PrimaryAlign:   text.AlignCenter,
		SecondaryAlign: text.AlignEnd,
	}
	op.ColorScale.Scale(0.5, 0.5, 0.5, 0.5)
	releaseTextDrawOptions(op)

	op = acquireTextDrawOptions()
	if tx, ty := op.GeoM.Apply(0, 0); tx != 0 || ty != 0 {
		t.Fatalf("GeoM not reset: translation (%f,%f)", tx, ty)
	}
	if op.ColorScale.R() != 1 || op.ColorScale.G() != 1 || op.ColorScale.B() != 1 || op.ColorScale.A() != 1 {
		t.Fatalf("ColorScale not reset: %v", op.ColorScale)
	}
	if op.LayoutOptions != (text.LayoutOptions{}) {
		t.Fatalf("LayoutOptions not reset: %#v", op.LayoutOptions)
	}
	releaseTextDrawOptions(op)
}
