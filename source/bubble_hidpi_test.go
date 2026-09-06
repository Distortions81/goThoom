package main

import (
	"bytes"
	"fmt"
	"image"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

func setupBubbleLayoutTest(t *testing.T) {
	t.Helper()
	fontSource, err := text.NewGoTextFaceSource(bytes.NewReader(notoSansBold))
	if err != nil {
		t.Fatal(err)
	}
	oldSettings, oldFontGen := gs, fontGen
	oldBold, oldRegular := bubbleFont, bubbleFontRegular
	oldHistory, oldScratch := bubblePlacementHistory, bubbleFrameScratch
	oldContext := lastBubbleLayoutContext
	t.Cleanup(func() {
		gs, fontGen = oldSettings, oldFontGen
		bubbleFont, bubbleFontRegular = oldBold, oldRegular
		bubblePlacementHistory, bubbleFrameScratch = oldHistory, oldScratch
		lastBubbleLayoutContext = oldContext
		clearBubbleTextCaches()
	})
	gs = gsdef
	gs.SpeechBubbles, gs.BubbleNormal, gs.BubbleSelf, gs.BubbleOtherPlayers = true, true, true, true
	gs.AnimatedChatBubbles = false
	bubbleFont = &text.GoTextFace{Source: fontSource, Size: 20}
	bubbleFontRegular = bubbleFont
	bubblePlacementHistory = make(map[bubblePlacementHistoryKey]bubblePlacementHistoryEntry)
	bubbleFrameScratch = speechBubbleFrameScratch{}
}

func TestSpeechBubbleViewportCoordinatesAtDisplayScales(t *testing.T) {
	setupBubbleLayoutTest(t)
	for _, scale := range []float64{1, 1.5, 2} {
		for _, extra := range []image.Point{{}, {X: 240}, {Y: 180}} {
			t.Run(fmt.Sprintf("scale_%g_letterbox_%v", scale, extra), func(t *testing.T) {
				width := roundToInt(float64(gameAreaSizeX+extra.X) * scale)
				height := roundToInt(float64(gameAreaSizeY+extra.Y) * scale)
				view, worldScale := fittedWorldView(width, height)
				parent := ebiten.NewImage(width, height)
				t.Cleanup(parent.Dispose)
				screen := parent.SubImage(view).(*ebiten.Image)
				gs.GameScale = worldScale
				for _, position := range []image.Point{{}, {X: fieldCenterX - 12}, {Y: fieldCenterY - 12}} {
					for _, far := range []bool{false, true} {
						clear(bubblePlacementHistory)
						snap := drawSnapshot{
							mobiles: []frameMobile{{Index: 1, H: int16(position.X), V: int16(position.Y)}},
							bubbles: []bubble{{Index: 1, H: int16(position.X), V: int16(position.Y), Far: far, Type: kBubbleNormal, Text: "Hello there"}},
						}
						drawSpeechBubbles(screen, snap, 1, speechBubbleWindowScale(worldScale))
						if len(bubbleFrameScratch.drawRequests) != 1 {
							t.Fatalf("position %v, far %v: bubble disappeared", position, far)
						}
						want := image.Pt(roundToInt(float64(position.X+fieldCenterX)*worldScale), roundToInt(float64(position.Y+fieldCenterY)*worldScale))
						prepared := bubbleFrameScratch.prepared[0]
						if prepared.referenceAnchor != want {
							t.Errorf("position %v, far %v: local anchor = %v, want %v", position, far, prepared.referenceAnchor, want)
						}
						geometry, ok := prepareBubbleDraw(screen, bubbleFrameScratch.drawRequests[0])
						if !ok || geometry.noArrow {
							t.Errorf("position %v, far %v: visible anchor lost its arrow", position, far)
						}
						if got := image.Pt(geometry.tailX+geometry.offsetX, geometry.tailY+geometry.offsetY); got != want.Add(view.Min) {
							t.Errorf("position %v, far %v: drawn tail = %v, want %v", position, far, got, want.Add(view.Min))
						}
						body := image.Rect(geometry.left, geometry.top, geometry.right, geometry.bottom).Add(view.Min)
						if !body.In(view) {
							t.Errorf("drawn body %v escaped viewport %v", body, view)
						}
					}
				}
			})
		}
	}
}

func TestSpeechBubbleLayoutResetsAfterGeometryChanges(t *testing.T) {
	setupBubbleLayoutTest(t)
	for _, change := range []string{"unchanged", "display density", "viewport size", "bubble scale", "font scale", "font reload"} {
		t.Run(change, func(t *testing.T) {
			clear(bubblePlacementHistory)
			gs.GameScale, gs.BubbleScale = 2, 1
			windowScale := speechBubbleWindowScale(gs.GameScale)
			parent := ebiten.NewImage(1600, 1200)
			t.Cleanup(parent.Dispose)
			bounds := image.Rect(0, 0, 1000, 800)
			snap := drawSnapshot{
				mobiles:     []frameMobile{{Index: 1}},
				descriptors: map[uint8]frameDescriptor{1: {Name: "Speaker"}},
				bubbles:     []bubble{{Index: 1, Type: kBubbleNormal, Text: "Hello there"}},
			}
			draw := func() {
				drawSpeechBubbles(parent.SubImage(bounds).(*ebiten.Image), snap, 1, windowScale)
			}
			draw()
			key := bubbleFrameScratch.prepared[0].key
			history := bubblePlacementHistory[key]
			// Simulate a compacted, displaced bubble from a crowded layout.
			history.offset = image.Pt(8, 0)
			history.renderX += 8
			history.sizePercent = 55
			history.speakerNamed = true
			history.laidOutAt, history.renderedAt = time.Now(), time.Now()
			bubblePlacementHistory[key] = history
			switch change {
			case "display density":
				gs.GameScale *= 1.5
				windowScale *= 1.5
				bounds = image.Rect(0, 0, 1500, 1200)
			case "viewport size":
				bounds = image.Rect(0, 0, 1200, 900)
			case "bubble scale":
				gs.BubbleScale = 1.5
			case "font scale":
				windowScale *= 1.5
			case "font reload":
				fontGen++
			}
			draw()
			got := bubblePlacementHistory[key]
			if change == "unchanged" {
				if got.sizePercent != 55 || !got.speakerNamed {
					t.Fatal("unchanged geometry discarded the stable layout")
				}
				return
			}
			if got.sizePercent == 55 || got.speakerNamed || got.offset != (image.Point{}) {
				t.Errorf("retained old geometry's compacted/displaced layout: %+v", got)
			}
			prepared := bubbleFrameScratch.prepared[0]
			want := prepared.normalRect.Min.Sub(prepared.referenceAnchor)
			if got.renderX != float64(want.X) || got.renderY != float64(want.Y) {
				t.Errorf("eased from old pixel coordinates: (%g,%g), want %v", got.renderX, got.renderY, want)
			}
		})
	}
}
