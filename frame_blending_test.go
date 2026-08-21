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
