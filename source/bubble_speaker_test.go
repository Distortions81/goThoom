package main

import (
	"image"
	"testing"
)

func TestBubbleSpeakerAnchorsKeepBodyAboveSprite(t *testing.T) {
	center := image.Pt(100, 100)
	upper, lower, mouth, sprite := bubbleSpeakerAnchors(center, 40, 0, 12, 4)
	metrics := bubbleMetrics{width: 80, height: 30, tailHeight: 12}
	body := bubbleRectForPlacement(upper.X, upper.Y, metrics, bubblePosNone, false)
	if body.Max.Y > sprite.Min.Y-4 {
		t.Fatalf("body %v covers sprite %v", body, sprite)
	}
	if !mouth.In(sprite) || mouth.Y >= center.Y || mouth.Y <= sprite.Min.Y {
		t.Fatalf("mouth estimate %v outside upper sprite", mouth)
	}
	if mouth == upper || mouth == lower {
		t.Fatal("mouth and body-placement anchors must be separate")
	}
	_, _, leftMouth, _ := bubbleSpeakerAnchors(center, 40, 4, 12, 4)
	if leftMouth.X >= mouth.X {
		t.Fatal("mouth estimate did not follow facing")
	}
}

func TestBubbleSpeakerClearanceAfterLayoutMovement(t *testing.T) {
	sprite := image.Rect(80, 80, 120, 120)
	for _, body := range []image.Rectangle{image.Rect(60, 60, 140, 100), image.Rect(60, 90, 140, 130), image.Rect(70, 70, 100, 150)} {
		clear, ok := clearBubbleSpeaker(body, sprite, 5, 200, 200)
		if !ok || !bubbleOverlapRect(clear, 5).Intersect(sprite).Empty() || !clear.In(image.Rect(0, 0, 200, 200)) {
			t.Fatalf("body %v was not moved clear: %v", body, clear)
		}
	}
	if _, ok := clearBubbleSpeaker(image.Rect(0, 0, 200, 200), sprite, 5, 200, 200); ok {
		t.Fatal("impossible clearance was accepted")
	}
}

func TestCaptionsAndSunstoneMessagesHaveNoSpeechTail(t *testing.T) {
	for _, kind := range []int{kBubbleThought, kBubbleRealAction, kBubblePlayerAction, kBubbleNarrate} {
		if bubbleHasSpeakerTail(kind) || bubbleTailHeight(kind, 2) != 0 {
			t.Fatalf("type %d has a speech tail", kind)
		}
	}
	for _, kind := range []int{kBubbleNormal, kBubbleWhisper, kBubbleYell, kBubbleMonster, kBubblePonder} {
		if !bubbleHasSpeakerTail(kind) || bubbleTailHeight(kind, 2) <= 0 {
			t.Fatalf("type %d lost its pointer/trail", kind)
		}
	}
}
