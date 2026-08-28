package main

import (
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

func TestLyingShadowStates(t *testing.T) {
	if !isLyingShadowState(poseDead) || !isLyingShadowState(poseLie) {
		t.Fatal("dead and lying poses should use drop shadows")
	}
	if isLyingShadowState(0) || isLyingShadowState(40) {
		t.Fatal("upright poses should not use drop shadows")
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

func TestClearCharacterShadowCache(t *testing.T) {
	detailedCharacterShadowMask = ebiten.NewImage(8, 8)
	layeredShadowCoverage = ebiten.NewImage(8, 8)
	layeredShadowIncoming = ebiten.NewImage(8, 8)
	layeredShadowPrevious = ebiten.NewImage(8, 8)
	layeredShadowScene = ebiten.NewImage(8, 8)
	frameLayeredShadowCompositeActive = true
	clearCharacterShadowCache()
	if detailedCharacterShadowMask != nil {
		t.Fatal("character shadow mask was not cleared")
	}
	if layeredShadowCoverage != nil || layeredShadowIncoming != nil || layeredShadowPrevious != nil || layeredShadowScene != nil || frameLayeredShadowCompositeActive {
		t.Fatal("layered character shadow composite was not cleared")
	}
}

func TestLayeredCharacterShadowCommandsFollowTheirCaster(t *testing.T) {
	resetLayeredCharacterShadows()
	t.Cleanup(resetLayeredCharacterShadows)

	want := characterShadowDraw{size: 37, x: 12, y: 24, alpha: 0.6}
	queueLayeredCharacterShadow(9, want)
	if _, ok := takeLayeredCharacterShadow(8); ok {
		t.Fatal("a different mobile consumed the prepared shadow")
	}
	got, ok := takeLayeredCharacterShadow(9)
	if !ok || got.size != want.size || got.x != want.x || got.y != want.y || got.alpha != want.alpha {
		t.Fatalf("prepared shadow = (%+v, %v), want (%+v, true)", got, ok, want)
	}
	if _, ok := takeLayeredCharacterShadow(9); ok {
		t.Fatal("the same layered shadow was drawn more than once")
	}
}

func TestUprightShadowCellBottomStaysAttached(t *testing.T) {
	originalScale := gs.GameScale
	gs.GameScale = 1
	t.Cleanup(func() { gs.GameScale = originalScale })

	projection := newCharacterShadowProjection(90)
	geo := uprightShadowGeoMWithFoot(20, 6, 21, 20, 100, 100, projection)
	footX, footY := geo.Apply(16, 21)
	if math.Abs(footX-100) > 1e-9 || math.Abs(footY-105) > 1e-9 {
		t.Fatalf("cell-bottom anchor = (%v, %v), want (100, 105)", footX, footY)
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

func TestCharacterShadowDarknessScalesFinalOpacity(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })

	projection := characterShadowProjection{contrast: 1}
	gs.ShadersEnabled = true
	gs.CharacterShadowDarkness = 0.01
	faint := characterShadowDrawAlpha(1, projection)
	gs.CharacterShadowDarkness = 1
	normal := characterShadowDrawAlpha(1, projection)
	gs.CharacterShadowDarkness = 2
	dark := characterShadowDrawAlpha(1, projection)
	if !(faint < normal && normal < dark) {
		t.Fatalf("shadow darkness opacities = faint %v, normal %v, dark %v", faint, normal, dark)
	}
	if dark > 1 {
		t.Fatalf("very dark shadow alpha = %v, want at most 1", dark)
	}
}

func TestMobileSunShadowAmountUsesSpatialBlocksAndExcludesSelf(t *testing.T) {
	resetMobileSunShadowBlocks()
	t.Cleanup(resetMobileSunShadowBlocks)

	casters := []mobileSunShadowCaster{
		{index: 1, quad: [4]shadowPoint{{0, 0}, {40, 0}, {40, 40}, {0, 40}}, strength: 1},
		{index: 3, quad: [4]shadowPoint{{256, 0}, {300, 0}, {300, 40}, {256, 40}}, strength: 1},
	}
	for i, caster := range casters {
		addMobileSunShadowBlocks(i, caster.quad)
	}

	inside := mobileSunShadowReceiver{index: 2, footX: 20, footY: 20, radius: 5}
	if got := mobileSunShadowAmount(inside, casters, frameMobileSunShadowBlocks); got != mobileSunShadeScale {
		t.Fatalf("full shadow amount = %v, want %v", got, mobileSunShadeScale)
	}
	self := inside
	self.index = 1
	if got := mobileSunShadowAmount(self, casters[:1], frameMobileSunShadowBlocks); got != 0 {
		t.Fatalf("self shadow amount = %v, want 0", got)
	}
	outside := mobileSunShadowReceiver{index: 2, footX: 150, footY: 20, radius: 5}
	if got := mobileSunShadowAmount(outside, casters, frameMobileSunShadowBlocks); got != 0 {
		t.Fatalf("empty block shadow amount = %v, want 0", got)
	}
	partial := mobileSunShadowReceiver{index: 2, footX: 40, footY: 20, radius: 10}
	if got := mobileSunShadowAmount(partial, casters, frameMobileSunShadowBlocks); got <= 0 || got >= mobileSunShadeScale {
		t.Fatalf("partial shadow amount = %v, want between 0 and %v", got, mobileSunShadeScale)
	}
}

func TestMobileSunShadowAmountUsesStrongestOverlappingCaster(t *testing.T) {
	resetMobileSunShadowBlocks()
	t.Cleanup(resetMobileSunShadowBlocks)

	quad := [4]shadowPoint{{0, 0}, {40, 0}, {40, 40}, {0, 40}}
	casters := []mobileSunShadowCaster{
		{index: 1, quad: quad, strength: 0.25},
		{index: 2, quad: quad, strength: 0.75},
	}
	for i, caster := range casters {
		addMobileSunShadowBlocks(i, caster.quad)
	}
	receiver := mobileSunShadowReceiver{index: 3, footX: 20, footY: 20, radius: 5}
	want := float32(0.75 * mobileSunShadeScale)
	if got := mobileSunShadowAmount(receiver, casters, frameMobileSunShadowBlocks); math.Abs(float64(got-want)) > 1e-6 {
		t.Fatalf("overlapping shadow amount = %v, want strongest caster %v", got, want)
	}
}

func TestMobileSunShadowAppearanceFadesOnlyNewEdgeCasters(t *testing.T) {
	original := mobileSizeFunc
	mobileSizeFunc = func(uint16) int { return 20 }
	t.Cleanup(func() { mobileSizeFunc = original })

	desc := frameDescriptor{PictID: 1}
	edge := frameMobile{Index: 2, H: int16(-fieldCenterX - 8), V: 0}
	previous := map[uint8]frameMobile{1: {Index: 1}}
	if got := mobileSunShadowAppearance(edge, desc, previous, 0.25); got != 0.25 {
		t.Fatalf("new edge caster appearance = %v, want 0.25", got)
	}
	previous[edge.Index] = edge
	if got := mobileSunShadowAppearance(edge, desc, previous, 0.25); got != 1 {
		t.Fatalf("existing edge caster appearance = %v, want 1", got)
	}
	if got := mobileSunShadowAppearance(frameMobile{Index: 3}, desc, previous, 0.25); got != 1 {
		t.Fatalf("new interior caster appearance = %v, want 1", got)
	}
	if got := mobileSunShadowAppearance(edge, desc, nil, 0.25); got != 1 {
		t.Fatalf("snell-change caster appearance = %v, want 1", got)
	}
}

func BenchmarkMobileSunShadowSpatialLookup(b *testing.B) {
	resetMobileSunShadowBlocks()
	defer resetMobileSunShadowBlocks()
	casters := make([]mobileSunShadowCaster, 0, 128)
	for i := 0; i < 128; i++ {
		x := float64((i % 16) * 48)
		y := float64((i / 16) * 48)
		quad := [4]shadowPoint{{x, y}, {x + 56, y}, {x + 56, y + 56}, {x, y + 56}}
		casters = append(casters, mobileSunShadowCaster{index: uint8(i), quad: quad, strength: 0.75})
		addMobileSunShadowBlocks(i, quad)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		caster := casters[i%len(casters)]
		receiver := mobileSunShadowReceiver{index: 255, footX: caster.quad[0].x + 24, footY: caster.quad[0].y + 24, radius: 10}
		_ = mobileSunShadowAmount(receiver, casters, frameMobileSunShadowBlocks)
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

func TestCharacterShadowRenderStateUsesContactShadowsWithoutSun(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() {
		gs = originalSettings
		gNight = NightInfo{}
	})

	gs.CharacterShadows = true
	gs.MaxNightLevel = 100

	gNight = NightInfo{Shadows: 25, Azimuth: 90, Cloudy: true}
	alpha, _, kind := currentCharacterShadowRenderState()
	if kind != characterShadowContact || alpha != contactShadowOpacity {
		t.Fatalf("cloudy shadow state = (%v, %v), want contact opacity %v", alpha, kind, contactShadowOpacity)
	}

	gNight = NightInfo{Shadows: 0, Azimuth: 90, Flags: kLightNoShadows}
	alpha, _, kind = currentCharacterShadowRenderState()
	if kind != characterShadowContact || alpha != contactShadowOpacity {
		t.Fatalf("indoor shadow state = (%v, %v), want contact opacity %v", alpha, kind, contactShadowOpacity)
	}

	gNight = NightInfo{Shadows: 50, Azimuth: 90}
	alpha, _, kind = currentCharacterShadowRenderState()
	if kind != characterShadowDirectional || alpha != 0.5 {
		t.Fatalf("clear shadow state = (%v, %v), want directional opacity 0.5", alpha, kind)
	}
}
