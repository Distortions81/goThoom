package main

import (
	"math"
	"testing"

	"gothoom/climg"
)

func TestFloorReflectionProfilesAndGroundOrder(t *testing.T) {
	oldEnabled := gs.ReplacementEffects
	gs.ReplacementEffects = true
	t.Cleanup(func() { gs.ReplacementEffects = oldEnabled })
	for _, id := range []uint16{160, 178, 249, 8006} {
		profile, ok := floorReflectionProfileForPict(id)
		if !ok || profile.alpha <= 0 || profile.heightScale <= 0 || !groundPictureDrawsBelowMobiles(id) {
			t.Errorf("floor %d has no usable reflection profile or ground draw order", id)
		}
	}
	for _, id := range []uint16{20, 185, 1260, 2632, 2994, 2995, 2996, 3529, 3558, 3575, 3721, 3946, 4716, 4808, 4966, 5334, 5975, 8007} {
		if pictureUsesFloorReflection(id) {
			t.Errorf("unregistered floor %d unexpectedly reflects mobiles", id)
		}
	}
	gs.ReplacementEffects = false
	if pictureUsesFloorReflection(8006) || groundPictureDrawsBelowMobiles(8006) {
		t.Fatal("floor reflection remained active with replacement effects disabled")
	}
}

func TestMobileReflectionFeetMeetGroundPoint(t *testing.T) {
	for _, test := range []struct {
		footY, height, footRow float64
	}{
		{100, 48, 0.9}, {35, 72, 0.65}, {-10, 13, 1},
	} {
		top := mobileReflectionTop(test.footY, test.height, test.footRow)
		if got := top - test.height*test.footRow; math.Abs(got-test.footY) > 1e-9 {
			t.Errorf("reflected foot row = %v, want %v", got, test.footY)
		}
	}
}

func TestMobilePoseCanReflect(t *testing.T) {
	for _, test := range []struct {
		state uint8
		want  bool
	}{
		{state: 0, want: true},
		{state: 31, want: true},
		{state: poseDead, want: false},
		{state: poseLie, want: false},
		{state: 40, want: true},
	} {
		if got := mobilePoseCanReflect(test.state); got != test.want {
			t.Errorf("pose %d reflection eligibility = %v, want %v", test.state, got, test.want)
		}
	}
}

func TestMobileReflectionRequiresUprightShadowArtwork(t *testing.T) {
	const upright = climg.PictDefFlagUprightShadow
	for _, test := range []struct {
		state uint8
		flags uint32
		want  bool
	}{
		{state: 0, flags: upright, want: true},
		{state: 4, flags: upright | climg.PictDefFlagEmitsLight, want: true},
		{state: 0, flags: 0, want: false},
		{state: 0, flags: climg.PictDefFlagEmitsLight, want: false},
		{state: poseDead, flags: upright, want: false},
		{state: poseLie, flags: upright, want: false},
	} {
		if got := mobileReflectionEligibleForFlags(test.state, test.flags); got != test.want {
			t.Errorf("pose %d, flags %#x reflection eligibility = %v, want %v", test.state, test.flags, got, test.want)
		}
	}
}

func TestMobileReflectionPoseSkewFollowsFacingAndDepth(t *testing.T) {
	const width, floorHeight = 40.0, 28.8
	east := mobileReflectionPoseSkew(0, width, floorHeight)
	if east <= 0 || east > 2.5 || mobileReflectionPoseSkew(3, width, floorHeight) != east {
		t.Fatalf("east-facing walk pose skew = %v", east)
	}
	if got := mobileReflectionPoseSkew(4, width, floorHeight); got <= 0 || got >= east {
		t.Errorf("southeast skew = %v, want smaller positive lean", got)
	}
	if got := mobileReflectionPoseSkew(16, width, floorHeight); math.Abs(got+east) > 1e-9 {
		t.Errorf("west skew = %v, want %v", got, -east)
	}
	for _, state := range []uint8{8, 24, poseDead, poseLie, 40} {
		if got := mobileReflectionPoseSkew(state, width, floorHeight); got != 0 {
			t.Errorf("pose %d skew = %v, want 0", state, got)
		}
	}
	if got := mobileReflectionPoseSkew(0, width, floorHeight/2); got <= 0 || got >= east {
		t.Errorf("shallow reflection skew = %v, want smaller than %v", got, east)
	}
}

func TestMobileReflectionOverlapScalesWithDepth(t *testing.T) {
	oldScale := gs.GameScale
	gs.GameScale = 1
	t.Cleanup(func() { gs.GameScale = oldScale })
	for _, drawSize := range []float64{20, 40, 80} {
		floorHeight := drawSize * 0.72
		floorOverlap := mobileReflectionFootOverlap(drawSize, floorHeight)
		puddleOverlap := mobileReflectionFootOverlap(drawSize, floorHeight/2)
		if floorOverlap <= 0 || floorOverlap > 2 || puddleOverlap <= 0 || puddleOverlap >= floorOverlap {
			t.Errorf("draw size %v has incorrect foot overlaps: floor %v, shallow puddle %v", drawSize, floorOverlap, puddleOverlap)
		}
	}
}

func TestFloorReflectionTileBoundsShareFractionalEdges(t *testing.T) {
	for _, scale := range []float64{0.75, 1, 1.3, 2} {
		_, right := tiledPictureSpan(100.35, 200, scale)
		left, _ := tiledPictureSpan(100.35+200*scale, 200, scale)
		if right != left {
			t.Errorf("tile clips split at scale %v: right %v, next left %v", scale, right, left)
		}
	}
}
