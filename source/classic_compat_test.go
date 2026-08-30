package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"math"
	"strings"
	"testing"
	"time"

	"gothoom/climg"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

func TestBubbleLifeModes(t *testing.T) {
	if got := bubbleLifeFrames("one two three", BubbleLifetimeClassic, 2, 1); got != 8*(1000/framems) {
		t.Fatalf("classic lifetime = %d frames", got)
	}
	if got := bubbleLifeFrames("one two three", BubbleLifetimeModern, 2, 1); got != 5*(1000/framems) {
		t.Fatalf("modern lifetime = %d frames", got)
	}
	if got := normalizeBubbleLifetimeMode("classic"); got != BubbleLifetimeClassic {
		t.Fatalf("normalized mode = %q", got)
	}
	if got := normalizeBubbleLifetimeMode("unknown"); got != BubbleLifetimeModern {
		t.Fatalf("unknown mode = %q", got)
	}
}

func TestMeasureBubbleShrinksLongTextToBodyLimit(t *testing.T) {
	fontSource, err := text.NewGoTextFaceSource(bytes.NewReader(notoSansBold))
	if err != nil {
		t.Fatal(err)
	}
	oldBold, oldRegular := bubbleFont, bubbleFontRegular
	bubbleFont = &text.GoTextFace{Source: fontSource, Size: 20}
	bubbleFontRegular = bubbleFont
	t.Cleanup(func() {
		bubbleFont, bubbleFontRegular = oldBold, oldRegular
	})

	limit := image.Pt(160, 90)
	metrics := measureBubble(strings.Repeat("several words ", 16), kBubbleNormal, 1, 1, limit)
	if metrics.width > limit.X || metrics.height > limit.Y {
		t.Fatalf("bubble size = %dx%d, want at most %dx%d", metrics.width, metrics.height, limit.X, limit.Y)
	}
	face, ok := metrics.face.(*text.GoTextFace)
	if !ok || face.Size >= 20 {
		t.Fatalf("long bubble face = %#v, want a font smaller than 20", metrics.face)
	}
}

func TestMeasureBubbleUsesTwoToOneBody(t *testing.T) {
	fontSource, err := text.NewGoTextFaceSource(bytes.NewReader(notoSansBold))
	if err != nil {
		t.Fatal(err)
	}
	oldBold, oldRegular := bubbleFont, bubbleFontRegular
	bubbleFont = &text.GoTextFace{Source: fontSource, Size: 12}
	bubbleFontRegular = bubbleFont
	t.Cleanup(func() {
		bubbleFont, bubbleFontRegular = oldBold, oldRegular
	})

	metrics := measureBubble(strings.Repeat("several words ", 12), kBubbleNormal, 1, 1, image.Pt(500, 500))
	ratio := float64(metrics.width) / float64(metrics.height)
	// Whole words stay intact, so some text has no exact 2:1 line break.
	if math.Abs(ratio-bubbleBodyAspectRatio) > 0.4 {
		t.Fatalf("bubble size = %dx%d (%.2f:1), want wrapping near %d:1", metrics.width, metrics.height, ratio, bubbleBodyAspectRatio)
	}
	if metrics.width > 500 || metrics.height > 500 {
		t.Fatalf("bubble size = %dx%d, want it within the body limit", metrics.width, metrics.height)
	}
	if metrics.width != metrics.baseWidth+2*metrics.pad || metrics.height != metrics.lineHeight*len(metrics.lines)+2*metrics.pad {
		t.Fatalf("bubble size = %dx%d, contains aspect-ratio padding beyond the normal text inset", metrics.width, metrics.height)
	}
}

func TestBubbleAspectDistanceAllowsWiderWrap(t *testing.T) {
	// Discrete word wrapping can jump past 2:1. Compare that jump
	// proportionally so a moderately wide result wins over a square-ish one.
	narrow := bubbleAspectDistance(5, 4) // 1.25:1
	wide := bubbleAspectDistance(3, 1)   // 3:1
	if wide >= narrow {
		t.Fatalf("wide distance %.2f >= narrow distance %.2f; want 3:1 preferred over 1.25:1", wide, narrow)
	}
	if got, want := bubbleAspectDistance(1, 1), bubbleAspectDistance(4, 1); got != want {
		t.Fatalf("symmetric distances = %.2f and %.2f, want equal", got, want)
	}
}

func TestOrphanBubblePolicyAndDescriptorReuse(t *testing.T) {
	for _, typ := range []int{kBubbleYell, kBubbleThought} {
		if !bubbleVisibleWithoutOwner(typ) {
			t.Fatalf("bubble type %d was hidden without its owner", typ)
		}
	}
	for _, typ := range []int{kBubbleNormal, kBubbleWhisper, kBubbleRealAction, kBubblePlayerAction, kBubbleMonster, kBubblePonder} {
		if bubbleVisibleWithoutOwner(typ) {
			t.Fatalf("bubble type %d remained visible without its owner", typ)
		}
	}

	original := frameDescriptor{Index: 7, Type: kDescPlayer, PictID: 100, Name: "Original", Colors: []byte{1, 2}}
	same := frameDescriptor{Index: 7, Type: kDescPlayer, PictID: 100, Name: "Original", Colors: []byte{1, 2}}
	replacement := frameDescriptor{Index: 7, Type: kDescPlayer, PictID: 101, Name: "Replacement", Colors: []byte{1, 2}}
	if !sameBubbleOwnerDescriptor(original, same) || sameBubbleOwnerDescriptor(original, replacement) {
		t.Fatal("descriptor identity comparison did not detect index reuse")
	}

	bubbles := []bubble{{Index: 7, Text: "unnamed"}, {Index: 7, OwnerName: "Original", Text: "relink"}, {Index: 8, Text: "keep"}}
	bubbles = discardUnnamedBubblesForDescriptorIndex(bubbles, 7)
	if len(bubbles) != 2 || bubbles[0].OwnerName != "Original" || bubbles[1].Index != 8 {
		t.Fatalf("descriptor replacement retained wrong bubbles: %+v", bubbles)
	}

	b := bubble{Index: 7, OwnerName: "Original"}
	descriptors := map[uint8]frameDescriptor{
		7: {Index: 7, Name: "Replacement"},
		9: {Index: 9, Name: "Original"},
	}
	mobiles := map[uint8]frameMobile{7: {Index: 7}, 9: {Index: 9, H: 12, V: 34}}
	m, ok := relinkBubbleMobileByName(&b, descriptors, mobiles)
	if !ok || b.Index != 9 || m.H != 12 || m.V != 34 {
		t.Fatalf("named orphan did not relink: bubble=%+v mobile=%+v ok=%v", b, m, ok)
	}
	unnamed := bubble{Index: 6}
	if _, ok := relinkBubbleMobileByName(&unnamed, descriptors, mobiles); ok {
		t.Fatal("unnamed orphan relinked without an identity")
	}
}

func TestNormalBubbleVerbUsesTerminalPunctuation(t *testing.T) {
	tests := map[string]string{
		"Hello there":  "says",
		"Are you ok?":  "asks",
		"Look out!   ": "exclaims",
	}
	for text, want := range tests {
		data := append([]byte{0, kBubbleNormal}, encodeMacRoman(text)...)
		data = append(data, 0)
		verb, _, _, _, _, _, _ := decodeBubble(data)
		if verb != want {
			t.Errorf("decodeBubble(%q) verb = %q, want %q", text, verb, want)
		}
	}
}

func TestBEPPClassicDisplayRouting(t *testing.T) {
	for _, prefix := range []string{"dd", "dl", "cf"} {
		raw := append([]byte{0xc2, prefix[0], prefix[1]}, []byte("hidden")...)
		if got := decodeBEPP(raw); got != "" {
			t.Errorf("BEPP %s displayed %q", prefix, got)
		}
	}
	raw := append([]byte{0xc2, 't', 'l'}, []byte("log me")...)
	if got := decodeBEPP(raw); got != "log me" {
		t.Fatalf("BEPP tl = %q", got)
	}
}

func TestBEPPInfoQueuesNamedPlayer(t *testing.T) {
	infoQueueMu.Lock()
	oldQueue := infoQueue
	infoQueue = map[string]struct{}{}
	infoQueueMu.Unlock()
	oldPlayers := players
	players = map[string]*Player{}
	t.Cleanup(func() {
		infoQueueMu.Lock()
		infoQueue = oldQueue
		infoQueueMu.Unlock()
		players = oldPlayers
	})
	raw := append([]byte{0xc2, 'i', 'n'}, pnTag("Bob")...)
	decodeBEPP(raw)
	infoQueueMu.Lock()
	_, ok := infoQueue["Bob"]
	infoQueueMu.Unlock()
	if !ok {
		t.Fatal("BEPP in did not queue Bob for be-info")
	}
}

func TestMobileNameStyleUsesWireBitsAndShareeUnderline(t *testing.T) {
	for colors, want := range map[uint8]uint8{
		0: styleRegular, 1: styleBold, 2: styleItalic, 3: styleBoldItalic,
	} {
		if got := mobileNameStyle(colors, false); got != want {
			t.Errorf("colors %#x style = %#x, want %#x", colors, got, want)
		}
		if got := mobileNameStyle(colors, true); got != want|styleUnderline {
			t.Errorf("sharee colors %#x style = %#x", colors, got)
		}
	}
}

func TestExplicitShadowPictureRules(t *testing.T) {
	oldSettings := gs
	gNight.mu.Lock()
	oldLevel, oldShadows, oldFlags := gNight.Level, gNight.Shadows, gNight.Flags
	gNight.Level, gNight.Shadows, gNight.Flags = 20, 0, 0
	gNight.mu.Unlock()
	t.Cleanup(func() {
		gs = oldSettings
		gNight.mu.Lock()
		gNight.Level, gNight.Shadows, gNight.Flags = oldLevel, oldShadows, oldFlags
		gNight.mu.Unlock()
	})
	if draw, _ := explicitShadowPictureAlpha(climg.PictDefIsShadow); draw {
		t.Fatal("explicit shadow drew with shadow level zero")
	}
	gNight.mu.Lock()
	gNight.Shadows, gNight.Level = 25, 34
	gNight.mu.Unlock()
	gs.MaxNightLevel = 100
	if draw, alpha := explicitShadowPictureAlpha(climg.PictDefIsShadow); !draw || alpha != 0.25 {
		t.Fatalf("dark explicit shadow = draw %v alpha %v", draw, alpha)
	}
	gs.MaxNightLevel = 33
	if draw, alpha := explicitShadowPictureAlpha(climg.PictDefIsShadow); !draw || alpha != 1 {
		t.Fatalf("limited-night explicit shadow = draw %v alpha %v", draw, alpha)
	}
	if draw, alpha := explicitShadowPictureAlpha(0); !draw || alpha != 1 {
		t.Fatalf("ordinary picture = draw %v alpha %v", draw, alpha)
	}
}

func TestChooseBubblePlacementAvoidsOccupiedQuadrant(t *testing.T) {
	metrics := bubbleMetrics{width: 40, height: 20, tailHeight: 10}
	anchor := image.Pt(100, 100)
	upperLeft := bubbleRectForPlacement(anchor.X, anchor.Y, metrics, bubblePosUpperLeft, false)
	pos, _ := chooseBubblePlacement(anchor, anchor, metrics, image.Rect(0, 0, 200, 200), []image.Rectangle{upperLeft}, 0, 2, bubblePosNone)
	if pos != bubblePosUpperRight {
		t.Fatalf("placement = %d, want upper-right %d", pos, bubblePosUpperRight)
	}
}

func TestChooseBubblePlacementCentersClearBubbleAboveSpeaker(t *testing.T) {
	metrics := bubbleMetrics{width: 40, height: 20, tailHeight: 10}
	anchor := image.Pt(100, 100)

	pos, rect := chooseBubblePlacement(anchor, anchor, metrics, image.Rect(0, 0, 200, 200), nil, 0, 2, bubblePosUpperLeft)
	if pos != bubblePosNone {
		t.Fatalf("placement = %d, want centered %d", pos, bubblePosNone)
	}
	want := bubbleRectForPlacement(anchor.X, anchor.Y, metrics, bubblePosNone, false)
	if rect != want {
		t.Fatalf("centered bubble = %v, want %v", rect, want)
	}
}

func TestChooseBubblePlacementUsesLowerAnchorNearTopEdge(t *testing.T) {
	metrics := bubbleMetrics{width: 240, height: 60, tailHeight: 10}
	upperAnchor := image.Pt(100, 20)
	lowerAnchor := image.Pt(100, 90)
	bounds := image.Rect(0, 0, 200, 200)

	pos, rect := chooseBubblePlacement(upperAnchor, lowerAnchor, metrics, bounds, nil, 0, 2, bubblePosUpperLeft)
	if pos != bubblePosLowerLeft && pos != bubblePosLowerRight {
		t.Fatalf("placement = %d, want a lower placement", pos)
	}
	if rect.Min.Y < lowerAnchor.Y+metrics.tailHeight {
		t.Fatalf("lower bubble %v was not placed below target anchor %v", rect, lowerAnchor)
	}
}

func TestChooseBubblePlacementUsesUpperAnchorNearBottomEdge(t *testing.T) {
	metrics := bubbleMetrics{width: 240, height: 60, tailHeight: 10}
	upperAnchor := image.Pt(100, 110)
	lowerAnchor := image.Pt(100, 180)
	bounds := image.Rect(0, 0, 200, 200)

	pos, rect := chooseBubblePlacement(upperAnchor, lowerAnchor, metrics, bounds, nil, 0, 6, bubblePosLowerLeft)
	if pos != bubblePosUpperLeft && pos != bubblePosUpperRight {
		t.Fatalf("placement = %d, want an upper placement", pos)
	}
	if rect.Max.Y > upperAnchor.Y-metrics.tailHeight {
		t.Fatalf("upper bubble %v was not placed above target anchor %v", rect, upperAnchor)
	}
}

func TestChooseBubblePlacementStaysAboveAwayFromTopEdge(t *testing.T) {
	metrics := bubbleMetrics{width: 40, height: 20, tailHeight: 10}
	upperAnchor := image.Pt(100, 90)
	lowerAnchor := image.Pt(100, 130)
	bounds := image.Rect(0, 0, 200, 200)
	upperLeft := bubbleRectForPlacement(upperAnchor.X, upperAnchor.Y, metrics, bubblePosUpperLeft, false)
	upperRight := bubbleRectForPlacement(upperAnchor.X, upperAnchor.Y, metrics, bubblePosUpperRight, false)

	pos, _ := chooseBubblePlacement(
		upperAnchor,
		lowerAnchor,
		metrics,
		bounds,
		[]image.Rectangle{upperLeft, upperRight},
		0,
		6,
		bubblePosLowerLeft,
	)
	if pos != bubblePosUpperLeft && pos != bubblePosUpperRight {
		t.Fatalf("placement = %d, want an upper placement away from the top edge", pos)
	}
}

func TestBubbleLayoutPeriodicReflow(t *testing.T) {
	now := time.Unix(100, 0)
	if !bubbleLayoutNeedsReflow(time.Time{}, now) {
		t.Fatal("new bubble did not request an initial layout")
	}
	if bubbleLayoutNeedsReflow(now.Add(-bubbleLayoutReflowInterval+time.Millisecond), now) {
		t.Fatal("bubble requested a reflow before the interval elapsed")
	}
	if !bubbleLayoutNeedsReflow(now.Add(-bubbleLayoutReflowInterval), now) {
		t.Fatal("bubble did not request a reflow when the interval elapsed")
	}
	if !bubbleLayoutNeedsReflow(now.Add(time.Millisecond), now) {
		t.Fatal("future layout timestamp did not recover with a reflow")
	}
}

func TestBubbleLayoutSolveTriggersImmediatelyOnLayoutEvents(t *testing.T) {
	now := time.Unix(100, 0)
	recent := []time.Time{now.Add(-100 * time.Millisecond)}
	clear := []jointBubbleLayoutItem{{rect: image.Rect(20, 20, 80, 50)}}
	if bubbleLayoutNeedsSolve(1, 1, recent, clear, now, true) {
		t.Fatal("unchanged clear layout solved again before the periodic check")
	}
	if !bubbleLayoutNeedsSolve(2, 1, recent, clear, now, true) {
		t.Fatal("new bubble did not trigger an immediate solve")
	}
	if !bubbleLayoutNeedsSolve(0, 1, nil, nil, now, true) {
		t.Fatal("expired bubble did not trigger an immediate solve")
	}
	if !bubbleLayoutNeedsSolve(1, 1, []time.Time{{}}, clear, now, true) {
		t.Fatal("same-count bubble replacement did not trigger an immediate solve")
	}
	overlapping := []jointBubbleLayoutItem{
		{rect: image.Rect(20, 20, 80, 50)},
		{rect: image.Rect(60, 20, 120, 50)},
	}
	if !bubbleLayoutNeedsSolve(2, 2, []time.Time{recent[0], recent[0]}, overlapping, now, true) {
		t.Fatal("new collision did not trigger an immediate solve")
	}
}

func TestSmoothBubbleLayoutCoordinate(t *testing.T) {
	if got := smoothBubbleLayoutCoordinate(0, 100, bubbleLayoutMotionHalfLife); math.Abs(got-50) > 0.001 {
		t.Fatalf("one half-life moved to %v, want 50", got)
	}
	if got := smoothBubbleLayoutCoordinate(25, 100, 0); got != 25 {
		t.Fatalf("zero-time movement = %v, want current position 25", got)
	}
	if got := smoothBubbleLayoutCoordinate(0, 100, bubbleLayoutMotionSnapAfter); got != 100 {
		t.Fatalf("completed movement = %v, want target 100", got)
	}
}

func TestBubbleDrawRectMatchesResolvedRequest(t *testing.T) {
	metrics := bubbleMetrics{width: 60, height: 30, tailHeight: 10, tailHalf: 2}
	request := bubbleDrawRequest{
		txt: "hello", x: 100, y: 100, placement: bubblePosUpperRight,
		bodyOffset: image.Pt(18, -7), metrics: metrics,
	}
	got, _, ok := bubbleDrawRect(image.Rect(0, 0, 300, 200), request)
	if !ok {
		t.Fatal("resolved draw request was rejected")
	}
	want := bubbleRectForPlacement(request.x, request.y, metrics, request.placement, false).Add(request.bodyOffset)
	if got != want {
		t.Fatalf("draw rectangle = %v, want solver rectangle %v", got, want)
	}
	if !bubbleOverlapsOccupied(got, []image.Rectangle{got}, 0) {
		t.Fatal("final overlap gate missed the exact rendered rectangle")
	}
}

func TestJointBubbleLayoutLeavesClearBubblesAtNormalPositions(t *testing.T) {
	bounds := image.Rect(0, 0, 400, 240)
	items := []jointBubbleLayoutItem{
		{rect: image.Rect(30, 30, 90, 60)},
		{rect: image.Rect(250, 150, 310, 180)},
	}
	wantA, wantB := items[0].rect, items[1].rect
	separateBubbleLayout(items, bounds)
	if items[0].rect != wantA || items[1].rect != wantB {
		t.Fatalf("clear bubbles moved from normal positions: %v %v", items[0].rect, items[1].rect)
	}
}

func TestJointBubbleLayoutSharesConflictDisplacement(t *testing.T) {
	bounds := image.Rect(0, 0, 400, 240)
	startA := image.Rect(100, 80, 180, 120)
	startB := image.Rect(150, 80, 230, 120)
	items := []jointBubbleLayoutItem{
		{rect: startA},
		{rect: startB},
	}
	separateBubbleLayout(items, bounds)
	if items[0].rect == startA || items[1].rect == startB {
		t.Fatalf("conflict displacement was not shared: %v %v", items[0].rect, items[1].rect)
	}
	if flags := bubbleLayoutConflictFlags(items); flags[0] || flags[1] {
		t.Fatalf("joint result still overlaps: %v %v", items[0].rect, items[1].rect)
	}
	moveA := startA.Min.Sub(items[0].rect.Min)
	moveB := items[1].rect.Min.Sub(startB.Min)
	difference := moveA.X - moveB.X
	if difference < 0 {
		difference = -difference
	}
	if difference > 1 || moveA.Y != 0 || moveB.Y != 0 {
		t.Fatalf("unequal shared displacement: a=%v b=%v", moveA, moveB)
	}
}

func TestJointBubbleLayoutTransfersEdgeConstrainedMovement(t *testing.T) {
	bounds := image.Rect(0, 0, 300, 180)
	startA := image.Rect(0, 60, 80, 100)
	startB := image.Rect(60, 60, 140, 100)
	items := []jointBubbleLayoutItem{{rect: startA}, {rect: startB}}
	separateBubbleLayout(items, bounds)
	if items[0].rect != startA {
		t.Fatalf("edge-constrained bubble moved outside its available half: %v", items[0].rect)
	}
	if items[1].rect.Min.X != startA.Max.X+bubbleCollisionGap {
		t.Fatalf("other bubble moved to %v, want the full separation transferred to x=%d", items[1].rect, startA.Max.X+bubbleCollisionGap)
	}
	if flags := bubbleLayoutConflictFlags(items); flags[0] || flags[1] {
		t.Fatalf("edge-constrained pair still conflicts: %v", items)
	}
}

func TestJointBubbleLayoutSeparatesDenseGroup(t *testing.T) {
	bounds := image.Rect(0, 0, 640, 480)
	items := make([]jointBubbleLayoutItem, 6)
	for i := range items {
		items[i].rect = image.Rect(270, 210, 370, 260)
	}
	separateBubbleLayout(items, bounds)
	for i, conflict := range bubbleLayoutConflictFlags(items) {
		if conflict {
			t.Fatalf("dense bubble %d still conflicts after joint solve: %v", i, items)
		}
	}
}

func TestBubbleLayoutEpochKeepsOnlyComparableClearLayout(t *testing.T) {
	anchors := []image.Point{image.Pt(100, 100), image.Pt(300, 100)}
	candidate := []jointBubbleLayoutItem{
		{rect: image.Rect(110, 70, 170, 100)},
		{rect: image.Rect(240, 70, 300, 100)},
	}
	comparable := []jointBubbleLayoutItem{
		{rect: candidate[0].rect.Add(image.Pt(8, 0))},
		{rect: candidate[1].rect.Add(image.Pt(-8, 0))},
	}
	if !keepPriorBubbleLayout(comparable, candidate, anchors, 12) {
		t.Fatal("comparable clear prior layout was needlessly replaced")
	}
	worse := []jointBubbleLayoutItem{
		{rect: candidate[0].rect.Add(image.Pt(40, 0))},
		{rect: candidate[1].rect.Add(image.Pt(-40, 0))},
	}
	if keepPriorBubbleLayout(worse, candidate, anchors, 12) {
		t.Fatal("meaningfully worse prior layout was retained")
	}
	overlapping := []jointBubbleLayoutItem{
		{rect: image.Rect(100, 70, 180, 110)},
		{rect: image.Rect(150, 70, 230, 110)},
	}
	if keepPriorBubbleLayout(overlapping, candidate, anchors, 12) {
		t.Fatal("overlapping prior layout was retained")
	}
}

func TestDistantBubbleSpeakerNameUsesHysteresis(t *testing.T) {
	anchor := image.Pt(100, 100)
	near := image.Rect(110, 80, 170, 120)
	middle := image.Rect(160, 80, 220, 120)
	far := image.Rect(180, 80, 240, 120)
	if bubbleNeedsSpeakerName(false, near, anchor, 1) {
		t.Fatal("near bubble requested a speaker name")
	}
	if bubbleNeedsSpeakerName(false, middle, anchor, 1) {
		t.Fatal("middle-distance bubble enabled its speaker name too early")
	}
	if !bubbleNeedsSpeakerName(true, middle, anchor, 1) {
		t.Fatal("middle-distance bubble dropped its existing speaker name without hysteresis")
	}
	if !bubbleNeedsSpeakerName(false, far, anchor, 1) {
		t.Fatal("far bubble did not request a speaker name")
	}
	if got := bubbleTextWithSpeakerName("Walker", "Line one"); got != "Walker: Line one" {
		t.Fatalf("speaker-labelled text = %q", got)
	}
	if got := bubbleTextWithSpeakerName("Walker", "Walker: Line one"); got != "Walker: Line one" {
		t.Fatalf("speaker name was duplicated: %q", got)
	}
}

func TestMovieMobileTableRestoresBubbleAndPicturelessFallback(t *testing.T) {
	resetDrawState()
	t.Cleanup(resetDrawState)
	layout, ok := layoutForMobileTable(142)
	if !ok {
		t.Fatal("current movie layout unavailable")
	}
	data := make([]byte, 4+16+layout.descSize+2+5+4)
	pos := 0
	binary.BigEndian.PutUint32(data[pos:], 1)
	pos += 4
	binary.BigEndian.PutUint32(data[pos:], 0)
	binary.BigEndian.PutUint32(data[pos+4:], 10)
	binary.BigEndian.PutUint32(data[pos+8:], 20)
	pos += 16
	buf := data[pos : pos+layout.descSize]
	binary.BigEndian.PutUint32(buf[0:], 0xffffffff)
	binary.BigEndian.PutUint32(buf[layout.typeOffset:], kDescPlayer)
	binary.BigEndian.PutUint32(buf[layout.bubbleTypeOffset:], kBubbleNormal)
	binary.BigEndian.PutUint32(buf[layout.bubbleCounterOffset:], 40)
	copy(buf[layout.nameOffset:], "Bob")
	pos += layout.descSize
	binary.BigEndian.PutUint16(data[pos:], 5)
	copy(data[pos+2:], "Hello")
	pos += 7
	binary.BigEndian.PutUint32(data[pos:], 0xffffffff)
	parseMobileTable(data, 0, 142, 0)
	stateMu.Lock()
	desc := state.descriptors[1]
	bubbles := append([]bubble(nil), state.bubbles...)
	stateMu.Unlock()
	if desc.PictID != picturelessDescriptorFallbackPictID {
		t.Fatalf("movie fallback picture = %d", desc.PictID)
	}
	if len(bubbles) != 1 || bubbles[0].Text != "Hello" || bubbles[0].LifeFrames != 40 {
		t.Fatalf("restored bubbles = %#v", bubbles)
	}
}

func TestLivePicturelessDescriptorUsesClassicFallback(t *testing.T) {
	resetTestState()
	data := buildDrawData("Bob", kBubbleNormal, "hello")
	// descriptor count starts at byte 9; the first descriptor picture is 12:14.
	binary.BigEndian.PutUint16(data[12:14], 0xffff)
	if _, _, err := parseDrawState(data, false); err != nil {
		t.Fatal(err)
	}
	stateMu.Lock()
	pict := state.descriptors[1].PictID
	stateMu.Unlock()
	if pict != picturelessDescriptorFallbackPictID {
		t.Fatalf("live fallback picture = %d", pict)
	}
}

func liveDrawPacketForTest(ack, resent uint32, text string) []byte {
	body := buildDrawData("Bob", kBubbleNormal, text)
	binary.BigEndian.PutUint32(body[1:5], ack)
	binary.BigEndian.PutUint32(body[5:9], resent)
	packet := make([]byte, len(body)+2)
	copy(packet[2:], body)
	return packet
}

func classicStateRecordForTest(text string) []byte {
	payload := []byte{0, 1, 1, byte(kBubbleNormal)}
	payload = append(payload, encodeMacRoman(text)...)
	payload = append(payload, 0, 0, 0)
	record := []byte{byte(len(payload) >> 8), byte(len(payload))}
	return append(record, payload...)
}

func liveDrawPacketWithStateFragmentForTest(ack, resent uint32, text string, fragment []byte) []byte {
	body := buildDrawData("Bob", kBubbleNormal, text)
	fullRecord := classicStateRecordForTest(text)
	body = append(body[:len(body)-len(fullRecord)], fragment...)
	binary.BigEndian.PutUint32(body[1:5], ack)
	binary.BigEndian.PutUint32(body[5:9], resent)
	packet := make([]byte, len(body)+2)
	binary.BigEndian.PutUint16(packet[:2], 2)
	copy(packet[2:], body)
	return packet
}

func prepareLiveStateFragmentTest(t *testing.T) {
	t.Helper()
	oldAck, oldResend, oldLastAck := ackFrame, resendFrame, lastAckFrame
	oldFrameCounter, oldMovieMode, oldEncrypted := frameCounter, movieMode, drawStateEncrypted
	oldSettings, oldPlayers := gs, players
	t.Cleanup(func() {
		ackFrame, resendFrame, lastAckFrame = oldAck, oldResend, oldLastAck
		frameCounter, movieMode, drawStateEncrypted = oldFrameCounter, oldMovieMode, oldEncrypted
		gs, players = oldSettings, oldPlayers
		resetDrawState()
	})
	resetDrawState()
	players = map[string]*Player{}
	gs.SpeechBubbles = true
	gs.BubbleNormal = true
	gs.BubbleOtherPlayers = true
	movieMode, drawStateEncrypted = false, false
	ackFrame, resendFrame, lastAckFrame, frameCounter = 0, 0, 0, 0
}

func TestLiveStateRecordCanSpanSizeAndPayloadAcrossFrames(t *testing.T) {
	prepareLiveStateFragmentTest(t)
	record := classicStateRecordForTest("fragmented")

	handleDrawState(liveDrawPacketWithStateFragmentForTest(1, 0, "fragmented", record[:1]), false)
	if ackFrame != 1 || resendFrame != 0 {
		t.Fatalf("size-high fragment ack=%d resend=%d, want 1/0", ackFrame, resendFrame)
	}
	handleDrawState(liveDrawPacketWithStateFragmentForTest(2, 0, "fragmented", record[1:6]), false)
	stateMu.Lock()
	bubblesBeforeCompletion := len(state.bubbles)
	pendingBeforeCompletion := len(state.stateDataStream)
	stateMu.Unlock()
	if bubblesBeforeCompletion != 0 || pendingBeforeCompletion != 6 {
		t.Fatalf("partial state = bubbles %d pending %d, want 0/6", bubblesBeforeCompletion, pendingBeforeCompletion)
	}

	handleDrawState(liveDrawPacketWithStateFragmentForTest(3, 0, "fragmented", record[6:]), false)
	stateMu.Lock()
	bubbles := append([]bubble(nil), state.bubbles...)
	pending := len(state.stateDataStream)
	stateMu.Unlock()
	if ackFrame != 3 || resendFrame != 0 || pending != 0 {
		t.Fatalf("completed state ack=%d resend=%d pending=%d, want 3/0/0", ackFrame, resendFrame, pending)
	}
	if len(bubbles) != 1 || bubbles[0].Text != "fragmented" {
		t.Fatalf("completed fragmented bubbles = %#v", bubbles)
	}
}

func TestLiveStateFragmentProcessesMultipleCompleteRecords(t *testing.T) {
	prepareLiveStateFragmentTest(t)
	first := classicStateRecordForTest("first")
	second := classicStateRecordForTest("second")
	fragment := append(append([]byte(nil), first...), second...)

	handleDrawState(liveDrawPacketWithStateFragmentForTest(1, 0, "first", fragment), false)
	stateMu.Lock()
	bubbles := append([]bubble(nil), state.bubbles...)
	pending := len(state.stateDataStream)
	stateMu.Unlock()
	if pending != 0 || len(bubbles) != 2 || bubbles[0].Text != "first" || bubbles[1].Text != "second" {
		t.Fatalf("multiple state records = pending %d bubbles %#v", pending, bubbles)
	}
}

func TestBadLogicalStateRecordDoesNotBlockFollowingRecord(t *testing.T) {
	prepareLiveStateFragmentTest(t)
	bad := []byte{0, 1, 'x'} // one-byte payload without the required C string terminator
	good := classicStateRecordForTest("after bad record")
	fragment := append(append([]byte(nil), bad...), good...)

	if !handleDrawState(liveDrawPacketWithStateFragmentForTest(1, 0, "after bad record", fragment), false) {
		t.Fatal("bad logical record rejected its structurally valid frame")
	}
	stateMu.Lock()
	bubbles := append([]bubble(nil), state.bubbles...)
	stateMu.Unlock()
	if len(bubbles) != 1 || bubbles[0].Text != "after bad record" {
		t.Fatalf("following logical record was not decoded: %#v", bubbles)
	}
}

func TestPendingStateFragmentSurvivesDrawStateClone(t *testing.T) {
	original := drawState{stateDataStream: []byte{0, 12, 1, 2, 3}}
	cloned := cloneDrawState(original)
	if got := cloned.stateDataStream; len(got) != 5 || got[4] != 3 {
		t.Fatalf("cloned pending state = %v", got)
	}
	cloned.stateDataStream[2] = 99
	if original.stateDataStream[2] != 1 {
		t.Fatal("cloned pending state aliases the original")
	}
}

func TestLiveStateFragmentRecoveryMatchesClassicResendFlow(t *testing.T) {
	prepareLiveStateFragmentTest(t)
	record := classicStateRecordForTest("recovered split")
	split := 7

	handleDrawState(liveDrawPacketWithStateFragmentForTest(1, 0, "recovered split", record[:split]), false)
	ignored := classicStateRecordForTest("must be ignored")
	handleDrawState(liveDrawPacketWithStateFragmentForTest(3, 0, "must be ignored", ignored), false)
	if ackFrame != 3 || resendFrame != 2 {
		t.Fatalf("gap state ack=%d resend=%d, want 3/2", ackFrame, resendFrame)
	}

	handleDrawState(liveDrawPacketWithStateFragmentForTest(4, 2, "recovered split", record[split:]), false)
	stateMu.Lock()
	bubbles := append([]bubble(nil), state.bubbles...)
	pending := len(state.stateDataStream)
	stateMu.Unlock()
	if ackFrame != 4 || resendFrame != 0 || pending != 0 {
		t.Fatalf("recovered split ack=%d resend=%d pending=%d, want 4/0/0", ackFrame, resendFrame, pending)
	}
	if len(bubbles) != 1 || bubbles[0].Text != "recovered split" {
		t.Fatalf("recovered split bubbles = %#v", bubbles)
	}
}

func TestLiveFrameOrderingAndStateRecovery(t *testing.T) {
	oldAck, oldResend, oldLastAck := ackFrame, resendFrame, lastAckFrame
	oldFrameCounter, oldMovieMode, oldEncrypted := frameCounter, movieMode, drawStateEncrypted
	oldSettings, oldPlayers := gs, players
	t.Cleanup(func() {
		ackFrame, resendFrame, lastAckFrame = oldAck, oldResend, oldLastAck
		frameCounter, movieMode, drawStateEncrypted = oldFrameCounter, oldMovieMode, oldEncrypted
		gs, players = oldSettings, oldPlayers
		resetDrawState()
	})
	resetDrawState()
	players = map[string]*Player{}
	gs.SpeechBubbles = true
	gs.BubbleNormal = true
	gs.BubbleOtherPlayers = true
	movieMode, drawStateEncrypted = false, false
	ackFrame, resendFrame, lastAckFrame, frameCounter = 0, 0, 0, 0

	handleDrawState(liveDrawPacketForTest(1, 0, "first"), false)
	if ackFrame != 1 || resendFrame != 0 {
		t.Fatalf("first frame ack=%d resend=%d", ackFrame, resendFrame)
	}

	handleDrawState(liveDrawPacketForTest(3, 0, "skipped state"), false)
	if ackFrame != 3 || resendFrame != 2 {
		t.Fatalf("gap frame ack=%d resend=%d, want 3/2", ackFrame, resendFrame)
	}
	if frameCounter != 3 {
		t.Fatalf("live frame clock = %d, want server acknowledgement 3", frameCounter)
	}
	stateMu.Lock()
	if len(state.bubbles) != 1 || state.bubbles[0].Text != "first" {
		got := append([]bubble(nil), state.bubbles...)
		stateMu.Unlock()
		t.Fatalf("gap applied state data: %#v", got)
	}
	stateMu.Unlock()

	handleDrawState(liveDrawPacketForTest(4, 2, "recovered"), false)
	if ackFrame != 4 || resendFrame != 0 {
		t.Fatalf("recovery frame ack=%d resend=%d, want 4/0", ackFrame, resendFrame)
	}
	if frameCounter != 4 {
		t.Fatalf("recovered live frame clock = %d, want 4", frameCounter)
	}
	stateMu.Lock()
	lastBubble := state.bubbles[len(state.bubbles)-1]
	stateMu.Unlock()
	if lastBubble.Text != "recovered" {
		t.Fatalf("recovered state bubble = %q", lastBubble.Text)
	}

	beforeFrame := frameCounter
	handleDrawState(liveDrawPacketForTest(4, 0, "duplicate"), false)
	if ackFrame != 4 || frameCounter != beforeFrame {
		t.Fatalf("duplicate changed ack/frame counter to %d/%d", ackFrame, frameCounter)
	}
}
