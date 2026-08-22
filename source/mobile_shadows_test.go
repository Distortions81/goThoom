package main

import (
	"math"
	"testing"
)

func TestChooseUprightShadowPose(t *testing.T) {
	tests := []struct {
		name    string
		state   uint8
		azimuth int
		want    uint8
		casts   bool
	}{
		{name: "east facing east sun", state: 0, azimuth: 0, want: 24, casts: true},
		{name: "preserves subpose", state: 3, azimuth: 0, want: 27, casts: true},
		{name: "rounds sun direction", state: 0, azimuth: 21, want: 24, casts: true},
		{name: "next sun direction", state: 0, azimuth: 22, want: 28, casts: true},
		{name: "wraps negative azimuth", state: 0, azimuth: -1, want: 24, casts: true},
		{name: "dead", state: poseDead, azimuth: 90, casts: false},
		{name: "lying", state: poseLie, azimuth: 90, casts: false},
		{name: "special pose unchanged", state: 40, azimuth: 90, want: 40, casts: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, casts := chooseUprightShadowPose(tt.state, tt.azimuth)
			if got != tt.want || casts != tt.casts {
				t.Fatalf("chooseUprightShadowPose(%d, %d) = (%d, %v), want (%d, %v)", tt.state, tt.azimuth, got, casts, tt.want, tt.casts)
			}
		})
	}
}

func TestUprightShadowLengthCardinalAngles(t *testing.T) {
	long := uprightShadowLength(0)
	short := uprightShadowLength(90)
	if long <= short {
		t.Fatalf("morning shadow length %v should exceed midday length %v", long, short)
	}
	if math.Abs(long-uprightShadowLength(180)) > 1e-9 {
		t.Error("opposite horizon angles should produce equal shadow lengths")
	}
	if math.Abs(short-uprightShadowLength(270)) > 1e-9 {
		t.Error("opposite midday angles should produce equal shadow lengths")
	}
	if long > 5 {
		t.Errorf("low-sun shadow length %v should remain visually bounded", long)
	}
}

func TestLowSunShadowsHaveSofterContrast(t *testing.T) {
	originalScale := gs.GameScale
	gs.GameScale = 1
	t.Cleanup(func() { gs.GameScale = originalScale })

	low := newCharacterShadowProjection(0)
	noon := newCharacterShadowProjection(90)
	if low.contrast >= noon.contrast {
		t.Fatalf("low-sun contrast %v should be softer than noon contrast %v", low.contrast, noon.contrast)
	}
	if low.contrast < minimumShadowContrast || noon.contrast > 1 {
		t.Fatalf("shadow contrasts out of range: low=%v noon=%v", low.contrast, noon.contrast)
	}
}

func TestDetailedShadowPenumbraGrowsAtLowSun(t *testing.T) {
	originalScale := gs.GameScale
	gs.GameScale = 1
	t.Cleanup(func() { gs.GameScale = originalScale })

	low := detailedCharacterShadowRadius(newCharacterShadowProjection(0))
	noon := detailedCharacterShadowRadius(newCharacterShadowProjection(90))
	if low <= noon {
		t.Fatalf("low-sun penumbra radius %v should exceed noon radius %v", low, noon)
	}
	if noon < 1 || low > detailedEdgeMaxRadius {
		t.Fatalf("penumbra radii out of bounds: low=%v noon=%v", low, noon)
	}
}

func TestUprightShadowProjectionDirection(t *testing.T) {
	originalScale := gs.GameScale
	gs.GameScale = 1
	t.Cleanup(func() { gs.GameScale = originalScale })

	tests := []struct {
		azimuth int
		dxSign  int
		dySign  int
	}{
		{azimuth: 0, dxSign: -1},
		{azimuth: 90, dySign: 1},
		{azimuth: 180, dxSign: 1},
		{azimuth: 270, dySign: -1},
	}
	for _, tt := range tests {
		projection := newCharacterShadowProjection(tt.azimuth)
		geo := uprightShadowGeoM(20, 20, 100, 100, projection)
		topX, topY := geo.Apply(10, 0)
		bottomX, bottomY := geo.Apply(10, 20)
		if math.Abs(bottomX-100) > 1e-9 || math.Abs(bottomY-110) > 1e-9 {
			t.Errorf("azimuth %d moved the foot anchor to (%v, %v)", tt.azimuth, bottomX, bottomY)
		}
		dx := topX - bottomX
		dy := topY - bottomY
		if tt.dxSign < 0 && dx >= 0 || tt.dxSign > 0 && dx <= 0 || tt.dySign < 0 && dy >= 0 || tt.dySign > 0 && dy <= 0 {
			t.Errorf("azimuth %d projected vector = (%v, %v)", tt.azimuth, dx, dy)
		}
	}
}

func TestCurrentCharacterShadowState(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() {
		gs = originalSettings
		gNight = NightInfo{}
	})

	gs.CharacterShadows = true
	gs.MaxNightLevel = 100
	gNight = NightInfo{Shadows: 50, Azimuth: -1}
	alpha, azimuth, ok := currentCharacterShadowState()
	if !ok || alpha != 0.5 || azimuth != 359 {
		t.Fatalf("currentCharacterShadowState() = (%v, %d, %v)", alpha, azimuth, ok)
	}

	gs.CharacterShadows = false
	if _, _, ok := currentCharacterShadowState(); ok {
		t.Error("disabled character shadows should not render")
	}
	gs.CharacterShadows = true
	gs.MaxNightLevel = 0
	if _, _, ok := currentCharacterShadowState(); ok {
		t.Error("zero max night level should disable shadows")
	}
	gs.MaxNightLevel = 100
	gNight = NightInfo{}
	if _, _, ok := currentCharacterShadowState(); ok {
		t.Error("zero area shadow level should disable shadows")
	}
}
