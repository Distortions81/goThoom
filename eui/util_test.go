//go:build test

package eui

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func TestWithinRange(t *testing.T) {
	if !withinRange(1, 1.1, 0.2) {
		t.Errorf("expected true")
	}
	if withinRange(1, 1.3, 0.2) {
		t.Errorf("expected false")
	}
}

func TestPointOperations(t *testing.T) {
	a := point{X: 1, Y: 2}
	b := point{X: 3, Y: 4}
	if r := pointAdd(a, b); r.X != 4 || r.Y != 6 {
		t.Errorf("pointAdd result %+v", r)
	}
	if r := pointSub(b, a); r.X != 2 || r.Y != 2 {
		t.Errorf("pointSub result %+v", r)
	}
	if r := pointMul(a, b); r.X != 3 || r.Y != 8 {
		t.Errorf("pointMul result %+v", r)
	}
	if r := pointDiv(b, a); r.X != 3 || r.Y != 2 {
		t.Errorf("pointDiv result %+v", r)
	}
	uiScale = 2
	if r := pointScaleMul(a); r.X != 2 || r.Y != 4 {
		t.Errorf("pointScaleMul result %+v", r)
	}
	if r := pointScaleDiv(point{X: 4, Y: 6}); r.X != 2 || r.Y != 3 {
		t.Errorf("pointScaleDiv result %+v", r)
	}
	uiScale = 1
}

func TestUnionRect(t *testing.T) {
	a := rect{X0: 0, Y0: 0, X1: 10, Y1: 10}
	b := rect{X0: 5, Y0: 5, X1: 15, Y1: 20}
	exp := rect{X0: 0, Y0: 0, X1: 15, Y1: 20}
	if r := unionRect(a, b); r != exp {
		t.Errorf("unionRect got %+v want %+v", r, exp)
	}
}

func TestMergeData(t *testing.T) {
	orig := &windowData{Title: "orig", Size: point{X: 10, Y: 10}, TitleHeight: 5}
	upd := &windowData{Title: "new", Size: point{X: 20, Y: 30}}
	res := mergeData(orig, upd).(*windowData)
	if res.Title != "new" {
		t.Errorf("Title=%v", res.Title)
	}
	if res.Size != (point{X: 20, Y: 30}) {
		t.Errorf("Size=%v", res.Size)
	}
	if res.TitleHeight != 5 {
		t.Errorf("TitleHeight=%v", res.TitleHeight)
	}
}

func TestMergeDataBoolHandling(t *testing.T) {
	type origStruct struct {
		A bool
		B bool
		C bool
	}
	type updStruct struct {
		A    bool
		ASet bool
		B    *bool
		C    bool
	}

	orig := &origStruct{A: true, B: true, C: true}
	bFalse := false
	upd := &updStruct{A: false, ASet: true, B: &bFalse, C: false}
	res := mergeData(orig, upd).(*origStruct)

	if res.A {
		t.Errorf("A=%v want false", res.A)
	}
	if res.B {
		t.Errorf("B=%v want false", res.B)
	}
	if !res.C {
		t.Errorf("C=%v want true", res.C)
	}
}

func TestMergeDataMismatchedStructs(t *testing.T) {
	type origStruct struct {
		A int
		B int
	}
	type updStruct struct {
		B int
		C int
	}

	orig := &origStruct{A: 1, B: 2}
	upd := &updStruct{B: 3, C: 4}
	res := mergeData(orig, upd).(*origStruct)
	if res.A != 1 {
		t.Errorf("A=%v", res.A)
	}
	if res.B != 3 {
		t.Errorf("B=%v", res.B)
	}
}

func TestPinPositions(t *testing.T) {
	screenWidth = 800
	screenHeight = 600
	win := &windowData{Position: point{X: 10, Y: 10}, Size: point{X: 100, Y: 80}, TitleHeight: 10}
	var pin pinType = PIN_TOP_RIGHT
	pos := pin.getWinPosition(win)
	exp := point{X: 800 - win.GetSize().X - win.GetPos().X, Y: win.GetPos().Y}
	if pos != exp {
		t.Errorf("top right got %+v want %+v", pos, exp)
	}
	item := &itemData{Position: point{X: 0, Y: 0}, Size: point{X: 20, Y: 20}, PinTo: PIN_BOTTOM_CENTER}
	res := item.PinTo.getItemPosition(win, item)
	expItem := point{X: win.GetSize().X/2 - item.GetSize().X/2 + item.GetPos().X,
		Y: win.GetSize().Y - win.GetTitleSize() - item.GetSize().Y - item.GetPos().Y}
	if res != expItem {
		t.Errorf("item position got %+v want %+v", res, expItem)
	}
}

func TestCenterPinOddScreenDimensions(t *testing.T) {
	oldW, oldH := screenWidth, screenHeight
	defer func() {
		screenWidth = oldW
		screenHeight = oldH
	}()

	screenWidth = 801
	screenHeight = 601
	win := &windowData{Position: point{X: 0, Y: 0}, Size: point{X: 100, Y: 80}}
	var pin pinType = PIN_MID_CENTER
	pos := pin.getWinPosition(win)
	exp := point{X: float32(screenWidth)/2 - win.GetSize().X/2 + win.GetPos().X,
		Y: float32(screenHeight)/2 - win.GetSize().Y/2 + win.GetPos().Y}
	if pos != exp {
		t.Errorf("mid center got %+v want %+v", pos, exp)
	}
}

func TestItemOverlap(t *testing.T) {
	win := &windowData{Size: point{X: 100, Y: 100}, Position: point{X: 0, Y: 0}}
	a := &itemData{Position: point{X: 0, Y: 0}, Size: point{X: 60, Y: 60}}
	b := &itemData{Position: point{X: 50, Y: 50}, Size: point{X: 60, Y: 60}}
	win.Contents = []*itemData{a, b}
	xc, yc := win.itemOverlap(win.Size)
	if !xc || !yc {
		t.Errorf("expected overlap got %v %v", xc, yc)
	}
	b.Position = point{X: 70, Y: 70}
	xc, yc = win.itemOverlap(win.Size)
	if xc || yc {
		t.Errorf("expected no overlap got %v %v", xc, yc)
	}
}

func TestSetSliderValue(t *testing.T) {
	if mplusFaceSource == nil {
		s, err := text.NewGoTextFaceSource(bytes.NewReader(notoTTF))
		if err != nil {
			t.Fatalf("font init: %v", err)
		}
		mplusFaceSource = s
	}
	item := &itemData{MinValue: 0, MaxValue: 10, AuxSize: point{X: 8}, AuxSpace: 4}
	item.DrawRect = rect{X0: 0, Y0: 0, X1: 100, Y1: 20}
	item.setSliderValue(point{X: 42})
	maxLabel := sliderMaxLabel
	textSize := (item.FontSize * uiScale) + 2
	face := textFace(textSize)
	maxW, _ := text.Measure(maxLabel, face, 0)
	knobW := item.AuxSize.X * uiScale
	width := item.DrawRect.X1 - item.DrawRect.X0 - knobW - currentStyle.SliderValueGap - float32(maxW)
	start := item.DrawRect.X0 + knobW/2
	val := float32(42) - start
	if val < 0 {
		val = 0
	}
	if val > width {
		val = width
	}
	want := item.MinValue + (val/width)*(item.MaxValue-item.MinValue)
	if math.Abs(float64(item.Value-want)) > 0.001 {
		t.Errorf("slider value got %v want %v", item.Value, want)
	}
	item.IntOnly = true
	item.setSliderValue(point{X: 42})
	if item.Value != float32(int(want+0.5)) {
		t.Errorf("int slider got %v want %v", item.Value, float32(int(want+0.5)))
	}
}

func TestSetSliderValueLog(t *testing.T) {
	if mplusFaceSource == nil {
		s, err := text.NewGoTextFaceSource(bytes.NewReader(notoTTF))
		if err != nil {
			t.Fatalf("font init: %v", err)
		}
		mplusFaceSource = s
	}
	item := &itemData{MinValue: 1, MaxValue: 1000, AuxSize: point{X: 8}, AuxSpace: 4, Log: true, LogValue: 10}
	item.DrawRect = rect{X0: 0, Y0: 0, X1: 100, Y1: 20}
	item.setSliderValue(point{X: 42})
	maxLabel := sliderMaxLabel
	textSize := (item.FontSize * uiScale) + 2
	face := textFace(textSize)
	maxW, _ := text.Measure(maxLabel, face, 0)
	knobW := item.AuxSize.X * uiScale
	width := item.DrawRect.X1 - item.DrawRect.X0 - knobW - currentStyle.SliderValueGap - float32(maxW)
	start := item.DrawRect.X0 + knobW/2
	val := float32(42) - start
	if val < 0 {
		val = 0
	}
	if val > width {
		val = width
	}
	ratio := val / width
	minLog := math.Log(float64(item.MinValue)) / math.Log(float64(item.LogValue))
	maxLog := math.Log(float64(item.MaxValue)) / math.Log(float64(item.LogValue))
	want := float32(math.Pow(float64(item.LogValue), minLog+float64(ratio)*(maxLog-minLog)))
	if math.Abs(float64(item.Value-want)) > 0.001 {
		t.Errorf("log slider value got %v want %v", item.Value, want)
	}
}

func sliderTrackLength(item *itemData) float32 {
	maxSize := item.GetSize()
	// Use a fixed label size when measuring so sliders with
	// different ranges have equal track lengths.
	maxLabel := sliderMaxLabel
	textSize := (item.FontSize * uiScale) + 2
	face := textFace(textSize)
	maxW, maxH := text.Measure(maxLabel, face, 0)
	knobW := item.AuxSize.X * uiScale
	knobH := item.AuxSize.Y * uiScale
	if item.Vertical {
		height := maxSize.Y - knobH - currentStyle.SliderValueGap - float32(maxH)
		if height < 0 {
			height = 0
		}
		return height
	}
	width := maxSize.X - knobW - currentStyle.SliderValueGap - float32(maxW)
	if width < 0 {
		width = 0
	}
	return width
}

func TestSliderTrackLengthMatch(t *testing.T) {
	if mplusFaceSource == nil {
		s, err := text.NewGoTextFaceSource(bytes.NewReader(notoTTF))
		if err != nil {
			t.Fatalf("font init: %v", err)
		}
		mplusFaceSource = s
	}

	base := &itemData{Size: point{X: 180, Y: 24}, AuxSize: point{X: 8, Y: 16}, FontSize: 12, MaxValue: 100}
	floatTrack := sliderTrackLength(base)

	intSlider := *base
	intSlider.MaxValue = 10
	intSlider.IntOnly = true
	intTrack := sliderTrackLength(&intSlider)

	if math.Abs(float64(floatTrack-intTrack)) > 0.001 {
		t.Errorf("track length mismatch: float %v int %v", floatTrack, intTrack)
	}
}

func TestSetSliderValueVertical(t *testing.T) {
	if mplusFaceSource == nil {
		s, err := text.NewGoTextFaceSource(bytes.NewReader(notoTTF))
		if err != nil {
			t.Fatalf("font init: %v", err)
		}
		mplusFaceSource = s
	}
	item := &itemData{MinValue: 0, MaxValue: 10, AuxSize: point{X: 16, Y: 8}, AuxSpace: 4, Vertical: true}
	item.DrawRect = rect{X0: 0, Y0: 0, X1: 20, Y1: 100}
	item.setSliderValue(point{Y: 58})
	maxLabel := sliderMaxLabel
	textSize := (item.FontSize * uiScale) + 2
	face := textFace(textSize)
	_, maxH := text.Measure(maxLabel, face, 0)
	knobH := item.AuxSize.Y * uiScale
	height := item.DrawRect.Y1 - item.DrawRect.Y0 - knobH - currentStyle.SliderValueGap - float32(maxH)
	start := item.DrawRect.Y0 + knobH/2
	val := float32(58) - start
	if val < 0 {
		val = 0
	}
	if val > height {
		val = height
	}
	want := item.MaxValue - (val/height)*(item.MaxValue-item.MinValue)
	if math.Abs(float64(item.Value-want)) > 0.001 {
		t.Errorf("vertical slider value got %v want %v", item.Value, want)
	}
	item.IntOnly = true
	item.setSliderValue(point{Y: 58})
	if item.Value != float32(int(want+0.5)) {
		t.Errorf("vertical int slider got %v want %v", item.Value, float32(int(want+0.5)))
	}
}

func TestVerticalSliderTrackLengthMatch(t *testing.T) {
	if mplusFaceSource == nil {
		s, err := text.NewGoTextFaceSource(bytes.NewReader(notoTTF))
		if err != nil {
			t.Fatalf("font init: %v", err)
		}
		mplusFaceSource = s
	}

	base := &itemData{Size: point{X: 24, Y: 180}, AuxSize: point{X: 16, Y: 8}, FontSize: 12, MaxValue: 100, Vertical: true}
	floatTrack := sliderTrackLength(base)

	intSlider := *base
	intSlider.MaxValue = 10
	intSlider.IntOnly = true
	intTrack := sliderTrackLength(&intSlider)

	if math.Abs(float64(floatTrack-intTrack)) > 0.001 {
		t.Errorf("vertical track length mismatch: float %v int %v", floatTrack, intTrack)
	}
}

func TestMarkOpen(t *testing.T) {
	oldW, oldH := screenWidth, screenHeight
	defer func() { screenWidth, screenHeight = oldW, oldH }()
	screenWidth, screenHeight = 200, 150

	win1 := &windowData{Title: "win1", open: true, Size: point{X: 100, Y: 100}}
	win2 := &windowData{Title: "win2", open: false, Size: point{X: 100, Y: 100}, Position: point{X: -50, Y: -50}}
	windows = []*windowData{win2, win1}
	activeWindow = win1
	win2.MarkOpen()
	if !win2.IsOpen() {
		t.Errorf("expected window to be open")
	}
	if activeWindow != win2 {
		t.Errorf("expected active window to be win2")
	}
	if len(windows) != 2 || windows[1] != win2 {
		t.Errorf("window order incorrect: %v", windows)
	}
	pos := win2.getPosition()
	if pos.X < 0 || pos.Y < 0 {
		t.Errorf("window not clamped: %+v", pos)
	}
}

func TestAddWindowReorders(t *testing.T) {
	win1 := &windowData{Title: "win1", open: true, Size: point{X: 100, Y: 100}}
	win2 := &windowData{Title: "win2", open: true, Size: point{X: 100, Y: 100}}
	windows = nil

	win1.AddWindow(false)
	win2.AddWindow(false)
	if len(windows) != 2 || windows[1] != win2 {
		t.Fatalf("expected win2 at front: %v", windows)
	}

	win1.AddWindow(false)
	if windows[1] != win1 {
		t.Errorf("expected win1 brought forward: %v", windows)
	}

	win1.AddWindow(true)
	if windows[0] != win1 {
		t.Errorf("expected win1 moved to back: %v", windows)
	}
}

func TestAddWindowRejectsInvalidSize(t *testing.T) {
	windows = nil

	win := &windowData{Title: "bad", Size: point{X: 0, Y: 50}}
	win.AddWindow(false)
	if len(windows) != 0 {
		t.Fatalf("expected window with zero width rejected, got %d", len(windows))
	}

	win.Size = point{X: 50, Y: -10}
	win.AddWindow(false)
	if len(windows) != 0 {
		t.Fatalf("expected window with negative height rejected, got %d", len(windows))
	}

	var nilWin *windowData
	// Ensure nil windows are ignored without panic
	nilWin.AddWindow(false)
	if len(windows) != 0 {
		t.Fatalf("expected nil window ignored, got %d", len(windows))
	}
}

func TestAddWindowNoTitle(t *testing.T) {
	windows = nil
	prevTheme := currentTheme
	currentTheme = &Theme{Window: windowData{TitleHeight: 24}}
	win := &windowData{Title: "win", Size: point{X: 100, Y: 100}, NoTitle: true}
	win.AddWindow(false)
	if win.TitleHeight != 0 {
		t.Fatalf("expected TitleHeight 0, got %v", win.TitleHeight)
	}
	currentTheme = prevTheme
}

func TestSetTitleSizeOverridesTheme(t *testing.T) {
	windows = nil
	prevTheme := currentTheme
	win := &windowData{Title: "win", Size: point{X: 100, Y: 100}}
	win.AddWindow(false)
	win.SetTitleSize(30)
	currentTheme = &Theme{Window: windowData{TitleHeight: 24}}
	applyThemeToAll()
	if win.TitleHeight != 30 {
		t.Fatalf("expected TitleHeight 30, got %v", win.TitleHeight)
	}
	currentTheme = prevTheme
}

func TestNoTitlebar(t *testing.T) {
	windows = nil
	prevTheme := currentTheme
	currentTheme = &Theme{Window: windowData{TitleHeight: 24}}
	win := &windowData{Title: "win", Size: point{X: 100, Y: 100}}
	win.AddWindow(false)
	win.NoTitlebar()
	if !win.NoTitle || win.TitleHeight != 0 {
		t.Fatalf("expected no titlebar, got NoTitle=%v TitleHeight=%v", win.NoTitle, win.TitleHeight)
	}
	applyThemeToAll()
	if !win.NoTitle || win.TitleHeight != 0 {
		t.Fatalf("expected NoTitlebar override to persist, got NoTitle=%v TitleHeight=%v", win.NoTitle, win.TitleHeight)
	}
	currentTheme = prevTheme
}
func TestSetSizeClampAndScroll(t *testing.T) {
	win := &windowData{
		Size:        point{X: 100, Y: 100},
		Scroll:      point{X: 50, Y: 50},
		Padding:     0,
		BorderPad:   0,
		TitleHeight: 0,
	}
	// content smaller than window
	win.Contents = []*itemData{{Size: point{X: 50, Y: 50}}}
	win.setSize(point{-10, -10})
	if win.Size.X < MinWinSizeX || win.Size.Y < MinWinSizeY {
		t.Errorf("size not clamped: %+v", win.Size)
	}
	// enlarge window so scroll should reset
	win.setSize(point{X: 200, Y: 200})
	if win.Scroll.X != 0 || win.Scroll.Y != 0 {
		t.Errorf("scroll not reset: %+v", win.Scroll)
	}
}

func TestResizeMarksDirty(t *testing.T) {
	win := &windowData{Size: point{X: 100, Y: 100}}
	win.setSize(point{X: 150, Y: 150})
	if !win.Dirty {
		t.Fatalf("expected window marked dirty after resize")
	}
}

func TestFixedAspectRatio(t *testing.T) {
	win := &windowData{Size: point{X: 100, Y: 50}, TitleHeight: 10, AspectA: 16, AspectB: 9, FixedRatio: true}

	win.setSize(point{X: 160, Y: 100})
	want := point{X: 160, Y: 100}
	if win.Size != want {
		t.Errorf("resize by width got %+v want %+v", win.Size, want)
	}

	win.Size = point{X: 100, Y: 50}
	win.setSize(point{X: 120, Y: 120})
	want = point{X: 195.55556, Y: 120}
	if math.Abs(float64(win.Size.X-want.X)) > 0.01 || math.Abs(float64(win.Size.Y-want.Y)) > 0.01 {
		t.Errorf("resize by height got %+v want %+v", win.Size, want)
	}
}

func TestFlowContentBounds(t *testing.T) {
	uiScale = 1

	vflow := &itemData{ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL}
	vflow.addItemTo(&itemData{ItemType: ITEM_BUTTON, Size: point{X: 10, Y: 20}})
	vflow.addItemTo(&itemData{ItemType: ITEM_BUTTON, Size: point{X: 15, Y: 30}})
	wantV := point{X: 15, Y: 50}
	if got := vflow.contentBounds(); got != wantV {
		t.Errorf("vertical bounds got %+v want %+v", got, wantV)
	}

	hflow := &itemData{ItemType: ITEM_FLOW, FlowType: FLOW_HORIZONTAL}
	hflow.addItemTo(&itemData{ItemType: ITEM_BUTTON, Size: point{X: 10, Y: 20}})
	hflow.addItemTo(&itemData{ItemType: ITEM_BUTTON, Size: point{X: 15, Y: 30}})
	wantH := point{X: 25, Y: 30}
	if got := hflow.contentBounds(); got != wantH {
		t.Errorf("horizontal bounds got %+v want %+v", got, wantH)
	}
}

func TestMarginAffectsBounds(t *testing.T) {
	uiScale = 1

	vflow := &itemData{ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL}
	vflow.addItemTo(&itemData{ItemType: ITEM_BUTTON, Size: point{X: 10, Y: 20}, Margin: 5})
	vflow.addItemTo(&itemData{ItemType: ITEM_BUTTON, Size: point{X: 15, Y: 30}, Margin: 5})
	wantV := point{X: 25, Y: 65}
	if got := vflow.contentBounds(); got != wantV {
		t.Errorf("vertical bounds with margin got %+v want %+v", got, wantV)
	}

	hflow := &itemData{ItemType: ITEM_FLOW, FlowType: FLOW_HORIZONTAL}
	hflow.addItemTo(&itemData{ItemType: ITEM_BUTTON, Size: point{X: 10, Y: 20}, Margin: 5})
	hflow.addItemTo(&itemData{ItemType: ITEM_BUTTON, Size: point{X: 15, Y: 30}, Margin: 5})
	wantH := point{X: 40, Y: 40}
	if got := hflow.contentBounds(); got != wantH {
		t.Errorf("horizontal bounds with margin got %+v want %+v", got, wantH)
	}

	win := &windowData{AutoSize: true, TitleHeight: 0}
	win.addItemTo(&itemData{ItemType: ITEM_BUTTON, Size: point{X: 10, Y: 20}, Margin: 5})
	win.Refresh()
	wantWin := point{X: 20, Y: 30}
	if got := win.GetSize(); got != wantWin {
		t.Errorf("window size with margin got %+v want %+v", got, wantWin)
	}
}

func TestWindowRefreshRecalculatesFlow(t *testing.T) {
	uiScale = 1

	win := &windowData{AutoSize: true, TitleHeight: 0}
	flow := &itemData{ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL}
	win.addItemTo(flow)
	flow.addItemTo(&itemData{ItemType: ITEM_BUTTON, Size: point{X: 10, Y: 20}})
	flow.addItemTo(&itemData{ItemType: ITEM_BUTTON, Size: point{X: 10, Y: 20}})

	win.Refresh()
	if got := win.GetSize().Y; got != 40 {
		t.Fatalf("initial window height got %v want %v", got, 40)
	}

	flow.Contents = flow.Contents[1:]
	win.Refresh()
	if got := win.GetSize().Y; got != 20 {
		t.Fatalf("refreshed window height got %v want %v", got, 20)
	}
}

func TestWindowRefreshKeepsFixedSize(t *testing.T) {
	uiScale = 1

	win := &windowData{Size: point{X: 100, Y: 50}, TitleHeight: 0}
	flow := &itemData{ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL}
	win.addItemTo(flow)
	flow.addItemTo(&itemData{ItemType: ITEM_BUTTON, Size: point{X: 10, Y: 20}})

	win.Refresh()
	if got := win.GetSize().Y; got != 50 {
		t.Fatalf("initial window height got %v want %v", got, 50)
	}

	flow.addItemTo(&itemData{ItemType: ITEM_BUTTON, Size: point{X: 10, Y: 40}})
	win.Refresh()
	if got := win.GetSize().Y; got != 50 {
		t.Fatalf("refreshed window height got %v want %v", got, 50)
	}
}

func TestPixelOffset(t *testing.T) {
	if off := pixelOffset(1); off != 0.5 {
		t.Errorf("odd width offset got %v", off)
	}
	if off := pixelOffset(2); off != 0 {
		t.Errorf("even width offset got %v", off)
	}
	if off := pixelOffset(3); off != 0.5 {
		t.Errorf("odd width offset got %v", off)
	}
}

func roundRectKeyPoints(rrect *roundRect) []point {
	width := float32(math.Round(float64(rrect.Border)))
	off := float32(0)
	if !rrect.Filled {
		off = pixelOffset(width)
	}
	x := float32(math.Round(float64(rrect.Position.X))) + off
	y := float32(math.Round(float64(rrect.Position.Y))) + off
	x1 := float32(math.Round(float64(rrect.Position.X+rrect.Size.X))) + off
	y1 := float32(math.Round(float64(rrect.Position.Y+rrect.Size.Y))) + off
	w := x1 - x
	h := y1 - y
	fillet := rrect.Fillet
	if !rrect.Filled && width > 0 {
		inset := width / 2
		x += inset
		y += inset
		w -= width
		h -= width
		if w < 0 {
			w = 0
		}
		if h < 0 {
			h = 0
		}
		if fillet > inset {
			fillet -= inset
		} else {
			fillet = 0
		}
	}
	if fillet*2 > w {
		fillet = w / 2
	}
	if fillet*2 > h {
		fillet = h / 2
	}
	fillet = float32(math.Round(float64(fillet)))
	return []point{
		{X: x + fillet, Y: y},
		{X: x + w - fillet, Y: y},
		{X: x + w, Y: y + fillet},
		{X: x + w, Y: y + h - fillet},
		{X: x + w - fillet, Y: y + h},
		{X: x + fillet, Y: y + h},
		{X: x, Y: y + h - fillet},
		{X: x, Y: y + fillet},
	}
}

func TestRoundRectSymmetry(t *testing.T) {
	r := &roundRect{
		Size:     point{X: 23.7, Y: 15.2},
		Position: point{X: 4.3, Y: 7.6},
		Fillet:   5,
		Filled:   true,
	}
	pts := roundRectKeyPoints(r)
	x := float32(math.Round(float64(r.Position.X)))
	x1 := float32(math.Round(float64(r.Position.X + r.Size.X)))
	mid := x + (x1-x)/2
	checkMirror := func(a, b point) bool {
		ax := a.X - mid
		bx := b.X - mid
		return math.Abs(float64(ax+bx)) < 0.001 && math.Abs(float64(a.Y-b.Y)) < 0.001
	}
	pairs := [][2]int{{0, 1}, {2, 7}, {3, 6}, {4, 5}}
	for _, p := range pairs {
		if !checkMirror(pts[p[0]], pts[p[1]]) {
			t.Errorf("points %d and %d not symmetrical: %+v vs %+v", p[0], p[1], pts[p[0]], pts[p[1]])
		}
	}
}

func TestRoundRectFilletClamp(t *testing.T) {
	r := &roundRect{
		Size:     point{X: 4, Y: 10},
		Position: point{X: 1, Y: 1},
		Fillet:   8,
		Filled:   true,
	}
	pts := roundRectKeyPoints(r)
	if pts[0].X != 3 || pts[1].X != 3 {
		t.Errorf("fillet clamp failed: %+v", pts)
	}
}

func TestStrokeLineParams(t *testing.T) {
	var x0, y0, w float32
	strokeLineFn = func(dst *ebiten.Image, ax0, ay0, ax1, ay1, width float32, col color.Color, aa bool) {
		x0, y0, w = ax0, ay0, width
	}
	defer func() { strokeLineFn = vector.StrokeLine }()

	img := ebiten.NewImage(10, 10)
	strokeLine(img, 1.3, 2.1, 5.8, 2.1, 1, color.White, false)
	if x0 != 1.5 || y0 != 2.5 || w != 1 {
		t.Errorf("odd width params %v %v %v", x0, y0, w)
	}

	strokeLine(img, 3.7, 4.2, 9.1, 4.2, 2, color.White, false)
	if x0 != 4 || y0 != 4 || w != 2 {
		t.Errorf("even width params %v %v %v", x0, y0, w)
	}
}

func TestStrokeRectParams(t *testing.T) {
	var x, y, bw float32
	strokeRectFn = func(dst *ebiten.Image, ax, ay, aw, ah, width float32, col color.Color, aa bool) {
		x, y, bw = ax, ay, width
	}
	defer func() { strokeRectFn = vector.StrokeRect }()

	img := ebiten.NewImage(10, 10)
	strokeRect(img, 1.3, 2.2, 5.6, 4.4, 3, color.White, false)
	if x != 1.5 || y != 2.5 || bw != 3 {
		t.Errorf("rect odd width params %v %v %v", x, y, bw)
	}

	strokeRect(img, 6.7, 1.4, 3.3, 2.8, 2, color.White, false)
	if x != 7 || y != 1 || bw != 2 {
		t.Errorf("rect even width params %v %v %v", x, y, bw)
	}
}

func TestClampToScreen(t *testing.T) {
	oldW, oldH := screenWidth, screenHeight
	defer func() { screenWidth, screenHeight = oldW, oldH }()
	screenWidth = 200
	screenHeight = 150

	pins := []pinType{
		PIN_TOP_LEFT, PIN_TOP_CENTER, PIN_TOP_RIGHT,
		PIN_MID_LEFT, PIN_MID_CENTER, PIN_MID_RIGHT,
		PIN_BOTTOM_LEFT, PIN_BOTTOM_CENTER, PIN_BOTTOM_RIGHT,
	}

	for _, pin := range pins {
		win := &windowData{
			Size:     point{X: 100, Y: 50},
			Position: point{X: -200, Y: -200},
			PinTo:    pin,
		}
		win.clampToScreen()
		pos := win.getPosition()
		if pos.X < 0 || pos.Y < 0 || pos.X+win.GetSize().X > float32(screenWidth) || pos.Y+win.GetSize().Y > float32(screenHeight) {
			t.Errorf("pin %v not clamped: pos=%+v size=%+v", pin, pos, win.GetSize())
		}
		switch pin {
		case PIN_TOP_RIGHT, PIN_MID_RIGHT, PIN_BOTTOM_RIGHT:
			if win.Position.X != 0 {
				t.Errorf("pin %v X offset %v not clamped to 0", pin, win.Position.X)
			}
		}
		switch pin {
		case PIN_BOTTOM_LEFT, PIN_BOTTOM_CENTER, PIN_BOTTOM_RIGHT:
			if win.Position.Y != 0 {
				t.Errorf("pin %v Y offset %v not clamped to 0", pin, win.Position.Y)
			}
		}
	}
}

func TestSetScreenSizeClamps(t *testing.T) {
	win := &windowData{Size: point{X: 100, Y: 50}, Position: point{X: 80, Y: 60}}
	win2 := &windowData{Size: point{X: 50, Y: 50}, Position: point{X: 80, Y: 60}}
	windows = []*windowData{win, win2}
	oldW, oldH := screenWidth, screenHeight
	defer func() {
		screenWidth, screenHeight = oldW, oldH
		windows = nil
	}()
	SetScreenSize(90, 70)
	pos := win.getPosition()
	if pos.X+win.GetSize().X > float32(screenWidth) || pos.Y+win.GetSize().Y > float32(screenHeight) {
		t.Errorf("window not clamped after SetScreenSize: %+v", pos)
	}
	pos2 := win2.getPosition()
	if pos2.X+win2.GetSize().X > float32(screenWidth) || pos2.Y+win2.GetSize().Y > float32(screenHeight) {
		t.Errorf("window not clamped after SetScreenSize: %+v", pos2)
	}
}

func TestAutoSizeWindowResizesWithScale(t *testing.T) {
	uiScale = 1
	oldW, oldH := screenWidth, screenHeight
	windows = nil
	overlays = nil
	screenWidth, screenHeight = 210, 200
	defer func() {
		screenWidth, screenHeight = oldW, oldH
		uiScale = 1
		windows = nil
	}()

	win := &windowData{AutoSize: true, TitleHeight: 0, Padding: 10}
	win.addItemTo(&itemData{ItemType: ITEM_BUTTON, Size: point{X: 100, Y: 50}})
	win.AddWindow(false)

	SetUIScale(2)
	if got := win.GetSize().X; got != float32(screenWidth) {
		t.Fatalf("scaled window width got %v want %v", got, screenWidth)
	}
}

func TestSetUIScaleClampsWindows(t *testing.T) {
	uiScale = 1
	oldW, oldH := screenWidth, screenHeight
	windows = nil
	screenWidth, screenHeight = 200, 150
	defer func() {
		screenWidth, screenHeight = oldW, oldH
		uiScale = 1
		windows = nil
	}()

	win := &windowData{Title: "win", open: true, Size: point{X: 100, Y: 50}, Position: point{X: 80, Y: 80}}
	windows = []*windowData{win}

	SetUIScale(2)

	pos := win.getPosition()
	size := win.GetSize()
	if pos.X < 0 || pos.Y < 0 || pos.X+size.X > float32(screenWidth) || pos.Y+size.Y > float32(screenHeight) {
		t.Fatalf("window not clamped: pos=%+v size=%+v", pos, size)
	}
}

func TestStrokeRectBorderRendering(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	strokeRectFn = func(_ *ebiten.Image, x, y, w, h, width float32, col color.Color, aa bool) {
		c := color.NRGBAModel.Convert(col).(color.NRGBA)
		ix, iy := int(x), int(y)
		iw, ih := int(w), int(h)

		for dx := 0; dx < iw; dx++ {
			img.SetNRGBA(ix+dx, iy, c)
			img.SetNRGBA(ix+dx, iy+ih-1, c)
		}
		for dy := 0; dy < ih; dy++ {
			img.SetNRGBA(ix, iy+dy, c)
			img.SetNRGBA(ix+iw-1, iy+dy, c)
		}
	}
	defer func() { strokeRectFn = vector.StrokeRect }()

	strokeRect(nil, 0, 0, 10, 10, 1, color.White, false)

	for x := 0; x < 10; x++ {
		if img.NRGBAAt(x, 0).A == 0 {
			t.Fatalf("top edge pixel (%d,0) is transparent", x)
		}
		if img.NRGBAAt(x, 9).A == 0 {
			t.Fatalf("bottom edge pixel (%d,9) is transparent", x)
		}
	}

	for y := 0; y < 10; y++ {
		if img.NRGBAAt(0, y).A == 0 {
			t.Fatalf("left edge pixel (0,%d) is transparent", y)
		}
		if img.NRGBAAt(9, y).A == 0 {
			t.Fatalf("right edge pixel (9,%d) is transparent", y)
		}
	}
}

func TestClampToScreenCenterMove(t *testing.T) {
	uiScale = 1
	oldW, oldH := screenWidth, screenHeight
	screenWidth, screenHeight = 200, 200
	defer func() {
		screenWidth, screenHeight = oldW, oldH
		uiScale = 1
	}()

	m := float32(10)
	win := &windowData{Size: point{X: 50, Y: 50}, PinTo: PIN_MID_CENTER, Margin: m}

	win.Position = point{X: -1000, Y: 0}
	win.clampToScreen()
	if pos := win.getPosition(); pos.X != m {
		t.Fatalf("left edge X=%v want %v", pos.X, m)
	}

	win.Position = point{X: 1000, Y: 0}
	win.clampToScreen()
	if pos := win.getPosition(); pos.X != float32(screenWidth)-win.GetSize().X-m {
		t.Fatalf("right edge X=%v", pos.X)
	}

	win.Position = point{X: 0, Y: -1000}
	win.clampToScreen()
	if pos := win.getPosition(); pos.Y != m {
		t.Fatalf("top edge Y=%v want %v", pos.Y, m)
	}

	win.Position = point{X: 0, Y: 1000}
	win.clampToScreen()
	if pos := win.getPosition(); pos.Y != float32(screenHeight)-win.GetSize().Y-m {
		t.Fatalf("bottom edge Y=%v", pos.Y)
	}
}

func TestClampToScreenCenterResize(t *testing.T) {
	uiScale = 1
	oldW, oldH := screenWidth, screenHeight
	screenWidth, screenHeight = 200, 200
	defer func() {
		screenWidth, screenHeight = oldW, oldH
		uiScale = 1
	}()

	m := float32(10)

	win := &windowData{Size: point{X: 50, Y: 50}, PinTo: PIN_MID_CENTER, Margin: m, Position: point{X: 1000, Y: 0}}
	win.clampToScreen()
	win.Size = point{X: 80, Y: 50}
	win.clampToScreen()
	if pos := win.getPosition(); pos.X != float32(screenWidth)-win.GetSize().X-m {
		t.Fatalf("right resize X=%v", pos.X)
	}

	win = &windowData{Size: point{X: 50, Y: 50}, PinTo: PIN_MID_CENTER, Margin: m, Position: point{X: 0, Y: 1000}}
	win.clampToScreen()
	win.Size = point{X: 50, Y: 80}
	win.clampToScreen()
	if pos := win.getPosition(); pos.Y != float32(screenHeight)-win.GetSize().Y-m {
		t.Fatalf("bottom resize Y=%v", pos.Y)
	}
}
