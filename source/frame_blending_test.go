package main

import (
	"testing"
	"time"
)

func TestFrameBlendingRequiresMotionSmoothing(t *testing.T) {
	originalMotion := gs.MotionSmoothing
	originalMobiles := gs.BlendMobiles
	originalPictures := gs.BlendPicts
	originalSuppress := suppressInterpOnce
	t.Cleanup(func() {
		gs.MotionSmoothing = originalMotion
		gs.BlendMobiles = originalMobiles
		gs.BlendPicts = originalPictures
		suppressInterpOnce = originalSuppress
	})

	gs.MotionSmoothing = false
	gs.BlendMobiles = true
	gs.BlendPicts = true
	suppressInterpOnce = false

	if mobileFrameBlendingEnabled() || pictureFrameBlendingEnabled() {
		t.Fatal("frame blending enabled without motion smoothing")
	}
	prev := time.Unix(0, 0)
	cur := prev.Add(time.Second)
	_, mobileFade, pictureFade := computeInterpolation(prev.Add(time.Second/2), prev, cur, 1, 1)
	if mobileFade != 1 || pictureFade != 1 {
		t.Fatalf("frame blending applied without motion smoothing: mobile=%v picture=%v", mobileFade, pictureFade)
	}

	gs.MotionSmoothing = true
	if !mobileFrameBlendingEnabled() || !pictureFrameBlendingEnabled() {
		t.Fatal("frame blending not enabled with motion smoothing")
	}
}

func TestFrameBlendFadeUsesElapsedTime(t *testing.T) {
	originalSettings := gs
	originalSuppress := suppressInterpOnce
	t.Cleanup(func() {
		gs = originalSettings
		suppressInterpOnce = originalSuppress
	})

	gs.MotionSmoothing = true
	gs.BlendMobiles = true
	gs.BlendPicts = true
	suppressInterpOnce = false
	previous := time.Unix(0, 0)
	current := previous.Add(200 * time.Millisecond)

	_, mobileFade, pictureFade := computeInterpolation(previous.Add(50*time.Millisecond), previous, current, 0.5, 1)
	if mobileFade != 0.5 {
		t.Errorf("mobile fade = %v, want 0.5", mobileFade)
	}
	if pictureFade != 0.25 {
		t.Errorf("picture fade = %v, want 0.25", pictureFade)
	}
}

func TestObscuringPictureOpacityPremultipliesColor(t *testing.T) {
	red, green, blue, alpha := premultipliedDrawColor(1, 0.5, 0.25, 0.4)
	if red != 0.4 || green != 0.2 || blue != 0.1 || alpha != 0.4 {
		t.Fatalf("premultiplied color = (%v, %v, %v, %v), want (0.4, 0.2, 0.1, 0.4)", red, green, blue, alpha)
	}
}
