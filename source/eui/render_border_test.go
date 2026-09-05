package eui

import "testing"

func TestInsideRectBorderRectsKeepAllEdgesInsideBounds(t *testing.T) {
	got := insideRectBorderRects(0, 0, 10, 10, 1)
	want := [4]rect{
		{X0: 0, Y0: 0, X1: 10, Y1: 1},
		{X0: 0, Y0: 9, X1: 10, Y1: 10},
		{X0: 0, Y0: 0, X1: 1, Y1: 10},
		{X0: 9, Y0: 0, X1: 10, Y1: 10},
	}
	if got != want {
		t.Fatalf("inside border rects got %+v, want %+v", got, want)
	}
}

func TestRenderImageSizeMatchesRoundedDrawSize(t *testing.T) {
	w, h := renderImageSize(point{X: 100.6, Y: 40.6})
	if w != 101 || h != 41 {
		t.Fatalf("rounded image size got %dx%d, want 101x41", w, h)
	}

	w, h = renderImageSize(point{X: 100.4, Y: 40.4})
	if w != 100 || h != 40 {
		t.Fatalf("rounded image size got %dx%d, want 100x40", w, h)
	}
}

func TestReusableRenderTargetKeepsSameSizedTexture(t *testing.T) {
	win := NewWindow()
	first := win.reusableRenderTarget(120, 80)
	if first == nil {
		t.Fatal("initial render target is nil")
	}
	if second := win.reusableRenderTarget(120, 80); second != first {
		t.Fatal("same-sized render target was replaced")
	}
	if resized := win.reusableRenderTarget(160, 80); resized == first {
		t.Fatal("resized render target reused the old texture")
	}
	win.deallocate()
}

func TestWindowCloseAndResizeRecycleRenderTargets(t *testing.T) {
	uiRenderTargets.Clear()
	defer uiRenderTargets.Clear()
	win := NewWindow()
	initial := win.reusableRenderTarget(120, 80)
	win.Close()
	if win.Render != nil {
		t.Fatal("closed window retained its leased render view")
	}
	if reopened := win.reusableRenderTarget(120, 80); reopened != initial {
		t.Fatal("reopening the same window replaced its allocation")
	}
	win.reusableRenderTarget(240, 160)
	if restored := win.reusableRenderTarget(120, 80); restored != initial {
		t.Fatal("returning to a previous window size missed its recycled target")
	}
	win.deallocate()
}
