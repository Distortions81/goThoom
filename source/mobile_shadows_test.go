package main

import (
	"image/color"
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
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
		{name: "preserves another facing subpose", state: 11, azimuth: 0, want: 3, casts: true},
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

func TestUprightShadowPoseMatchesWalkCycle(t *testing.T) {
	for facing := uint8(0); facing < 8; facing++ {
		first, casts := chooseUprightShadowPose(facing*4, 73)
		if !casts {
			t.Fatalf("facing %d did not cast", facing)
		}
		for subpose := uint8(0); subpose < 4; subpose++ {
			got, casts := chooseUprightShadowPose(facing*4+subpose, 73)
			if !casts || got/4 != first/4 || got%4 != subpose {
				t.Fatalf("facing %d subpose %d = (%d, %v), want facing %d subpose %d", facing, subpose, got, casts, first/4, subpose)
			}
		}
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
	if math.Abs(characterShadowSunHeight(0)-7) > 1e-9 || math.Abs(characterShadowSunHeight(90)-55) > 1e-9 {
		t.Errorf("classic sun heights = (%v, %v), want (7, 55)", characterShadowSunHeight(0), characterShadowSunHeight(90))
	}
	if long > 9 {
		t.Errorf("low-sun shadow length %v should remain visually bounded", long)
	}
}

func TestUprightShadowProjectionChangesWithSun(t *testing.T) {
	previous := newCharacterShadowProjection(0)
	for _, azimuth := range []int{30, 60, 90} {
		current := newCharacterShadowProjection(azimuth)
		if current.angle == previous.angle {
			t.Fatalf("azimuth %d retained angle %v", azimuth, current.angle)
		}
		if current.length == previous.length {
			t.Fatalf("azimuth %d retained length %v", azimuth, current.length)
		}
		previous = current
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

func TestUprightShadowGradientFadesTowardHead(t *testing.T) {
	const alpha = 0.8
	_, headOpacity := characterShadowTreatmentAtDistance(48)
	head := float32(alpha * headOpacity)
	toe := float32(alpha)
	if head <= 0 || head >= toe {
		t.Fatalf("gradient alpha head=%v toe=%v", head, toe)
	}
}

func TestCharacterShadowTreatment(t *testing.T) {
	contactSoftness, contactOpacity := characterShadowTreatmentAtDistance(0)
	if contactSoftness != 0 || contactOpacity != 1 {
		t.Fatalf("contact treatment = (%v, %v), want (0, 1)", contactSoftness, contactOpacity)
	}
	headSoftness, headOpacity := characterShadowTreatmentAtDistance(48)
	if headSoftness != 3 || math.Abs(headOpacity-0.35) > 1e-9 {
		t.Fatalf("48px treatment = (%v, %v), want (3, 0.35)", headSoftness, headOpacity)
	}
	farSoftness, farOpacity := characterShadowTreatmentAtDistance(96)
	if farSoftness != headSoftness || farOpacity != headOpacity {
		t.Fatalf("treatment did not clamp: far=(%v, %v) head=(%v, %v)", farSoftness, farOpacity, headSoftness, headOpacity)
	}
}

func TestCharacterShadowUpscaleFactor(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })

	gs.GameScale = 1
	gs.PotatoGPU = false
	if got := characterShadowUpscaleFactor(); got != 2 {
		t.Fatalf("normal 1x shadow upscale = %d, want 2", got)
	}
	gs.GameScale = 4
	if got := characterShadowUpscaleFactor(); got != 4 {
		t.Fatalf("normal 4x shadow upscale = %d, want 4", got)
	}
	gs.PotatoGPU = true
	if got := characterShadowUpscaleFactor(); got != 1 {
		t.Fatalf("potato shadow upscale = %d, want 1", got)
	}
}

func TestClearCharacterShadowCache(t *testing.T) {
	detailedCharacterShadowMask = ebiten.NewImage(8, 8)
	key := characterShadowTextureKey{id: 22, state: 0}
	characterShadowTextures[key] = characterShadowTexture{image: ebiten.NewImage(8, 8), contentSize: 4, padding: 2}
	clearCharacterShadowCache()
	if detailedCharacterShadowMask != nil {
		t.Fatal("character shadow mask was not cleared")
	}
	if len(characterShadowTextures) != 0 {
		t.Fatal("character shadow texture cache was not cleared")
	}
}

func TestCharacterShadowFootYUsesOpaqueSilhouette(t *testing.T) {
	pixels := make([]byte, 8*8*4)
	pixels[(1*8+2)*4+3] = 0xff
	if got := characterShadowFootYFromPixels(pixels, 8, 8, 3, 4); got != 10 {
		t.Fatalf("silhouette foot Y = %v, want 10", got)
	}
}

func TestCharacterShadowCacheSharesPaletteIndependentSilhouette(t *testing.T) {
	originalSettings := gs
	gs.GameScale = 1
	gs.PotatoGPU = false
	clearCharacterShadowCache()
	t.Cleanup(func() {
		clearCharacterShadowCache()
		gs = originalSettings
	})

	sprite := ebiten.NewImage(8, 8)
	sprite.Fill(color.White)
	first := characterShadowTextureForWithOpaqueBottom(makeMobileKey(447, 0, []byte{1, 2, 3}), sprite, 8, 8)
	second := characterShadowTextureForWithOpaqueBottom(makeMobileKey(447, 0, []byte{9, 8, 7}), sprite, 8, 8)
	if first.image != second.image {
		t.Fatal("palette-only change created a duplicate shadow texture")
	}
}

func TestCharacterShadowTextureCacheReusesShaderResult(t *testing.T) {
	originalSettings := gs
	gs.GameScale = 1
	gs.PotatoGPU = false
	clearCharacterShadowCache()
	t.Cleanup(func() {
		clearCharacterShadowCache()
		gs = originalSettings
	})

	sprite := ebiten.NewImage(8, 8)
	key := makeMobileKey(447, 0, nil)
	first := characterShadowTextureForWithOpaqueBottom(key, sprite, 8, 8)
	second := characterShadowTextureForWithOpaqueBottom(key, sprite, 8, 8)
	if first.image != second.image {
		t.Fatal("character shadow shader result was not reused")
	}
	if first.padding != 6 || first.contentSize != 16 {
		t.Fatalf("cached character shadow metadata = %+v", first)
	}

	gs.PotatoGPU = true
	potato := characterShadowTextureFor(key, sprite, 8)
	if potato.image != sprite {
		t.Fatal("potato mode did not bypass the filtered shadow texture")
	}
	if potato.padding != 0 || potato.contentSize != 8 {
		t.Fatalf("potato shadow metadata = %+v", potato)
	}
}

func TestUprightShadowOpaqueFootStaysAttached(t *testing.T) {
	originalScale := gs.GameScale
	gs.GameScale = 1
	t.Cleanup(func() { gs.GameScale = originalScale })

	projection := newCharacterShadowProjection(90)
	geo := uprightShadowGeoMWithFoot(20, 6, 21, 20, 100, 100, projection)
	footX, footY := geo.Apply(16, 21)
	if math.Abs(footX-100) > 1e-9 || math.Abs(footY-105) > 1e-9 {
		t.Fatalf("opaque foot anchor = (%v, %v), want (100, 105)", footX, footY)
	}
}

func TestUprightShadowPaddingPreservesFootAnchor(t *testing.T) {
	originalScale := gs.GameScale
	gs.GameScale = 1
	t.Cleanup(func() { gs.GameScale = originalScale })

	projection := newCharacterShadowProjection(90)
	plain := uprightShadowGeoM(20, 20, 100, 100, projection)
	padded := uprightShadowGeoMWithPadding(20, 7, 20, 100, 100, projection)
	plainX, plainY := plain.Apply(10, 20)
	paddedX, paddedY := padded.Apply(17, 27)
	if math.Abs(plainX-paddedX) > 1e-9 || math.Abs(plainY-paddedY) > 1e-9 {
		t.Fatalf("padded foot anchor = (%v, %v), want (%v, %v)", paddedX, paddedY, plainX, plainY)
	}
}

func TestCharacterShadowModesUseBalancedCoreOpacity(t *testing.T) {
	if normalShadowOpacity != detailedCoreOpacity {
		t.Fatalf("normal core opacity %v differs from detailed %v", normalShadowOpacity, detailedCoreOpacity)
	}
	if normalShadowOpacity <= 0.5 || normalShadowOpacity >= 1 {
		t.Fatalf("balanced shadow opacity %v should be between half and full strength", normalShadowOpacity)
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
