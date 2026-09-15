package main

import (
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestHealingBurstShaderCompiles(t *testing.T) {
	shader, err := ebiten.NewShader(healingBurstShaderSource)
	if err != nil {
		t.Fatalf("compile healing burst shader: %v", err)
	}
	shader.Deallocate()
}

func TestReplacementShaderReloadUsesSourceTreeNotUserData(t *testing.T) {
	originalSourceDir, originalDataDir := replacementEffectsShaderSourceDir, dataDirPath
	replacementEffectsShaderSourceDir = t.TempDir()
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		replacementEffectsShaderSourceDir, dataDirPath = originalSourceDir, originalDataDir
	})

	const source = "package main\nfunc Fragment(_ vec4, _ vec2, _ vec4) vec4 { return vec4(1) }\n"
	if err := os.WriteFile(filepath.Join(replacementEffectsShaderSourceDir, "reload-test.kage"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDirPath, "shaders"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDirPath, "shaders", "reload-test.kage"), []byte("not valid kage"), 0o644); err != nil {
		t.Fatal(err)
	}

	shader, err := compileReplacementEffectShader("reload-test.kage", []byte("not valid kage"))
	if err != nil {
		t.Fatalf("reload should use the source-tree shader: %v", err)
	}
	shader.Deallocate()
}

func TestFirePlumeShaderCompiles(t *testing.T) {
	shader, err := ebiten.NewShader(firePlumeShaderSource)
	if err != nil {
		t.Fatalf("compile fire plume shader: %v", err)
	}
	shader.Deallocate()
}

func TestBloodGushShaderCompiles(t *testing.T) {
	shader, err := ebiten.NewShader(bloodGushShaderSource)
	if err != nil {
		t.Fatalf("compile blood gush shader: %v", err)
	}
	shader.Deallocate()
}

func TestWavingFlagShaderCompiles(t *testing.T) {
	shader, err := ebiten.NewShader(wavingFlagShaderSource)
	if err != nil {
		t.Fatalf("compile waving flag shader: %v", err)
	}
	shader.Deallocate()
}

func TestWallTorchShaderCompiles(t *testing.T) {
	shader, err := ebiten.NewShader(wallTorchShaderSource)
	if err != nil {
		t.Fatalf("compile wall torch shader: %v", err)
	}
	shader.Deallocate()
}

func TestHiddenPathShaderCompiles(t *testing.T) {
	shader, err := ebiten.NewShader(hiddenPathShaderSource)
	if err != nil {
		t.Fatalf("compile hidden path shader: %v", err)
	}
	shader.Deallocate()
}

func TestMysticOrbitWardShaderCompiles(t *testing.T) {
	shader, err := ebiten.NewShader(mysticOrbitWardShaderSource)
	if err != nil {
		t.Fatalf("compile mystic orbit ward shader: %v", err)
	}
	shader.Deallocate()
}

func TestLavaPoolShaderCompiles(t *testing.T) {
	shader, err := ebiten.NewShader(lavaPoolShaderSource)
	if err != nil {
		t.Fatalf("compile lava pool shader: %v", err)
	}
	shader.Deallocate()
}

func TestMagicMoteRingShaderCompiles(t *testing.T) {
	shader, err := ebiten.NewShader(magicMoteRingShaderSource)
	if err != nil {
		t.Fatalf("compile magic mote ring shader: %v", err)
	}
	shader.Deallocate()
}

func TestTownPuddleShaderCompiles(t *testing.T) {
	shader, err := ebiten.NewShader(townPuddleShaderSource)
	if err != nil {
		t.Fatalf("compile town puddle shader: %v", err)
	}
	shader.Deallocate()
}

func TestShoreWaveShaderCompiles(t *testing.T) {
	shader, err := ebiten.NewShader(shoreWaveShaderSource)
	if err != nil {
		t.Fatalf("compile shore wave shader: %v", err)
	}
	shader.Deallocate()
}

func TestStoneFormShaderCompiles(t *testing.T) {
	shader, err := ebiten.NewShader(stoneFormShaderSource)
	if err != nil {
		t.Fatalf("compile stone form shader: %v", err)
	}
	shader.Deallocate()
}

func TestStoneFormPreviewPhaseDoesNotRestart(t *testing.T) {
	const elapsed = 3.25
	if got := replacementEffectPreviewPhase(replacementEffectStoneForm, elapsed); got != elapsed {
		t.Fatalf("stone form preview phase = %v, want %v", got, elapsed)
	}
}

func TestReplacementEffectPreviewSelection(t *testing.T) {
	originalSelection, originalMode, originalScale, originalUPS := replacementEffectsPreviewSelection, replacementEffectsPreviewMode, replacementEffectsPreviewScale, replacementEffectsPreviewUPS
	t.Cleanup(func() {
		replacementEffectsPreviewSelection, replacementEffectsPreviewMode, replacementEffectsPreviewScale, replacementEffectsPreviewUPS = originalSelection, originalMode, originalScale, originalUPS
	})

	replacementEffectsPreviewSelection = 1
	items := replacementEffectPreviewItems()
	if len(items) != 1 || items[0].kind != replacementEffectFirePlume || items[0].pictID != 481 {
		t.Fatalf("selected preview = %#v, want fire plume 481", items)
	}
	replacementEffectsPreviewMode = replacementEffectPreviewBoth
	if got := replacementEffectPreviewLabel(replacementEffectsPreviewMode); got != "Original + new effect" {
		t.Fatalf("both preview label = %q", got)
	}
	replacementEffectsPreviewScale = replacementEffectPreviewDoubleSize
	if replacementEffectsPreviewScale != replacementEffectPreviewDoubleSize {
		t.Fatal("preview scale did not retain the selected 200% mode")
	}
	replacementEffectsPreviewUPS = 2
	if replacementEffectsPreviewUPS != 2 {
		t.Fatal("preview did not retain the selected animation rate")
	}

	replacementEffectsPreviewSelection = -1
	if got := len(replacementEffectPreviewItems()); got != len(replacementEffectsPreviews) {
		t.Fatalf("gallery has %d effects, want %d", got, len(replacementEffectsPreviews))
	}
}

func TestReplacementEffectsPreviewGrassPatternIsStable(t *testing.T) {
	first := replacementEffectsPreviewGrassPattern(4, 7)
	if got := replacementEffectsPreviewGrassPattern(4, 7); got != first {
		t.Fatalf("grass pattern changed between calls: %08x then %08x", first, got)
	}
	if other := replacementEffectsPreviewGrassPattern(5, 7); other == first {
		t.Fatalf("adjacent grass tiles share pattern %08x", first)
	}
}

func TestFirePlumeIsAnOverscannedOneShot(t *testing.T) {
	if !replacementEffectIsOneShot(replacementEffectFirePlume) {
		t.Fatal("fire plume should play as a brief impact")
	}
	if !replacementEffectNeedsOverscan(replacementEffectFirePlume) {
		t.Fatal("fire plume needs room for rising embers")
	}
}

func TestWavingFlagIsPersistent(t *testing.T) {
	if replacementEffectIsPersistent(replacementEffectWavingFlag) == false {
		t.Fatal("waving flags must remain visible with their source sprite")
	}
	if replacementEffectIsPersistent(replacementEffectFirePlume) {
		t.Fatal("fire plume must remain a transient effect")
	}
	if !replacementEffectIsPersistent(replacementEffectMysticOrbitWard) {
		t.Fatal("orbiting mystic ward must remain visible with its source sprite")
	}
	if !replacementEffectIsPersistent(replacementEffectLavaPool) {
		t.Fatal("lava pool must remain visible with its source sprite")
	}
	if replacementEffectUsesPlayerMask(replacementEffectLavaPool) {
		t.Fatal("lava pool must remain on the ground instead of pinning to a nearby player")
	}
	if !replacementEffectIsPersistent(replacementEffectMagicMoteRing) {
		t.Fatal("magic mote ring must remain visible with its source sprite")
	}
	if !replacementEffectIsPersistent(replacementEffectTownPuddle) {
		t.Fatal("town puddle must remain visible with its source sprite")
	}
	if replacementEffectUsesPlayerMask(replacementEffectTownPuddle) {
		t.Fatal("town puddle must remain on the ground instead of pinning to a nearby player")
	}
	if !replacementEffectDrawsBelowMobiles(replacementEffectTownPuddle) || !replacementEffectDrawsBelowMobiles(replacementEffectLavaPool) {
		t.Fatal("ground replacement effects must draw below mobiles")
	}
	if replacementEffectDrawsBelowMobiles(replacementEffectFirePlume) {
		t.Fatal("spell replacement effects must remain above mobiles")
	}
}

func TestReplacementEffectFramePhaseUsesSourceAnimationTimeline(t *testing.T) {
	if got := replacementEffectFramePhase(0, 4, 1.35); got != 0 {
		t.Fatalf("first frame phase = %v, want 0", got)
	}
	if got, want := replacementEffectFramePhase(3, 4, 1.35), float32(1.35); math.Abs(float64(got-want)) > 0.0001 {
		t.Fatalf("last frame phase = %v, want %v", got, want)
	}
	if got, want := replacementEffectFramePhase(7, 4, 1), float32(1); got != want {
		t.Fatalf("wrapped source frame phase = %v, want %v", got, want)
	}
}

func TestReplacementEffectPreviewPhaseLoopsOneShots(t *testing.T) {
	cycle := float64(replacementEffectSequenceDuration(replacementEffectFirePlume))
	if got := replacementEffectPreviewPhase(replacementEffectFirePlume, cycle*2); math.Abs(float64(got)) > 0.0001 {
		t.Fatalf("fire preview phase = %v, want a new local cycle", got)
	}
	wardCycle := float64(replacementEffectSequenceDuration(replacementEffectMysticWard))
	if got, want := replacementEffectPreviewPhase(replacementEffectMysticWard, wardCycle*2+0.20), float32(0.20); math.Abs(float64(got-want)) > 0.0001 {
		t.Fatalf("ward preview phase = %v, want %v", got, want)
	}
	if got, want := replacementEffectPreviewPhase(replacementEffectHealing, 2.90), float32(2.90); got != want {
		t.Fatalf("healing preview phase = %v, want continuous %v", got, want)
	}
	if got, want := replacementEffectPreviewPhase(replacementEffectMysticOrbitWard, 2.90), float32(2.90); got != want {
		t.Fatalf("orbit ward preview phase = %v, want continuous %v", got, want)
	}
	if got, want := replacementEffectSequenceDuration(replacementEffectMysticOrbitWard), float32(0.8); got != want {
		t.Fatalf("orbit ward cycle = %v, want %v", got, want)
	}
	if got, want := replacementEffectSequenceDuration(replacementEffectLavaPool), float32(0.8); got != want {
		t.Fatalf("lava pool cycle = %v, want %v", got, want)
	}
	if got, want := replacementEffectSequenceDuration(replacementEffectMagicMoteRing), float32(0.8); got != want {
		t.Fatalf("magic mote ring cycle = %v, want %v", got, want)
	}
	if got, want := replacementEffectSequenceDuration(replacementEffectTownPuddle), float32(3.2); got != want {
		t.Fatalf("town puddle cycle = %v, want %v", got, want)
	}
}

func TestOriginalEffectPreviewDoesNotSkipReplacementSheet(t *testing.T) {
	originalReady, originalEnabled := replacementEffectsShadersReady, gs.ReplacementEffects
	replacementEffectsShadersReady, gs.ReplacementEffects = true, true
	t.Cleanup(func() {
		replacementEffectsShadersReady, gs.ReplacementEffects = originalReady, originalEnabled
	})

	key := makeSheetKey(1286, nil, false)
	if !shouldSkipArtworkSheet(key, false) {
		t.Fatal("normal world artwork loading should skip a replacement effect")
	}
	if shouldSkipArtworkSheet(key, true) {
		t.Fatal("original-effect preview must load the legacy replacement sheet")
	}
}

func TestWavingFlagThemes(t *testing.T) {
	for _, test := range []struct {
		id   uint16
		want float32
	}{
		{885, 0}, {886, 1}, {887, 2}, {5645, 3}, {5646, 4}, {5647, 5},
	} {
		if got := replacementEffectFlagTheme(test.id); got != test.want {
			t.Errorf("flag theme for %d = %v, want %v", test.id, got, test.want)
		}
	}
}

func TestWavingFlagMirrors(t *testing.T) {
	for _, id := range []uint16{5645, 5646, 5647} {
		if got := replacementEffectFlagMirror(id); got != 1 {
			t.Errorf("flag %d mirror = %v, want 1", id, got)
		}
	}
	if got := replacementEffectFlagMirror(885); got != 0 {
		t.Fatalf("flag 885 mirror = %v, want 0", got)
	}
}

func TestWallTorchMirrors(t *testing.T) {
	if got := replacementEffectWallTorchMirror(330); got != 1 {
		t.Fatalf("torch 330 mirror = %v, want 1", got)
	}
	if got := replacementEffectWallTorchMirror(331); got != 0 {
		t.Fatalf("torch 331 mirror = %v, want 0", got)
	}
}

func TestFirePlumeThemes(t *testing.T) {
	for _, test := range []struct {
		id   uint16
		want float32
	}{
		{481, 0}, {482, 0}, {572, 0}, {1739, 1}, {1740, 2}, {1741, 3},
	} {
		if got := replacementEffectFireTheme(test.id); got != test.want {
			t.Errorf("fire theme for %d = %v, want %v", test.id, got, test.want)
		}
	}
}

func TestWallTorchStartOffsetsAreStableAndDistinct(t *testing.T) {
	first := replacementEffectInstanceStartOffset(replacementEffectWallTorch, 0x100020003)
	if got := replacementEffectInstanceStartOffset(replacementEffectWallTorch, 0x100020003); got != first {
		t.Fatalf("wall torch offset changed from %v to %v", first, got)
	}
	second := replacementEffectInstanceStartOffset(replacementEffectWallTorch, 0x100020004)
	if second == first {
		t.Fatalf("distinct wall torches share offset %v", first)
	}
	if first < 0 || first > 4*time.Second || second < 0 || second > 4*time.Second {
		t.Fatalf("wall torch offsets outside phase range: %v, %v", first, second)
	}
	if got := replacementEffectInstanceStartOffset(replacementEffectHealing, 0x100020003); got != 0 {
		t.Fatalf("healing received wall-torch phase offset %v", got)
	}
}

func TestHiddenPathVariants(t *testing.T) {
	if got := replacementEffectPathVariant(445); got != 0 {
		t.Fatalf("path variant 445 = %v, want 0", got)
	}
	if got := replacementEffectPathVariant(446); got != 1 {
		t.Fatalf("path variant 446 = %v, want 1", got)
	}
}

func TestMagicMoteRingThemes(t *testing.T) {
	for id := uint16(1039); id <= 1044; id++ {
		if got, want := replacementEffectMagicTheme(id), float32(id-1039); got != want {
			t.Errorf("magic theme for %d = %v, want %v", id, got, want)
		}
	}
}

func TestTownPuddleVariants(t *testing.T) {
	if got := replacementEffectPuddleVariant(888); got != 0 {
		t.Fatalf("puddle 888 variant = %v, want 0", got)
	}
	if got := replacementEffectPuddleVariant(889); got != 1 {
		t.Fatalf("puddle 889 variant = %v, want 1", got)
	}
}

func TestTownPuddleCycleFollowsMovingPicture(t *testing.T) {
	originalReady, originalEnabled := replacementEffectsShadersReady, gs.ReplacementEffects
	originalDraws, originalNow := replacementEffectDraws, drawFrameNow
	replacementEffectsShadersReady, gs.ReplacementEffects = true, true
	replacementEffectDraws = make(map[uint64]replacementEffectDraw)
	drawFrameNow = time.Unix(1_000, 0)
	t.Cleanup(func() {
		replacementEffectsShadersReady, gs.ReplacementEffects = originalReady, originalEnabled
		replacementEffectDraws, drawFrameNow = originalDraws, originalNow
	})

	picture := framePicture{PictID: 888, H: 12, V: 18, lightKey: 0x1234}
	identity := replacementEffectGroundPictureInstanceKey(picture)
	if !queueReplacementPictureEffect(picture.PictID, 1, picture.H, picture.V, identity, 50, 80, 60, 9, 1, nil, 0, 0, 0) {
		t.Fatal("first puddle was not queued")
	}
	key := identity | uint64(replacementEffectTownPuddle)<<56
	started := replacementEffectDraws[key].started
	if started.IsZero() {
		t.Fatal("puddle cycle did not start")
	}

	beginReplacementEffects()
	drawFrameNow = drawFrameNow.Add(400 * time.Millisecond)
	picture.H, picture.V = 24, 30
	if got := replacementEffectGroundPictureInstanceKey(picture); got != identity {
		t.Fatalf("moving picture identity = %x, want %x", got, identity)
	}
	if !queueReplacementPictureEffect(picture.PictID, 2, picture.H, picture.V, identity, 62, 92, 60, 9, 1, nil, 0, 0, 0) {
		t.Fatal("moving puddle was not queued")
	}
	if got := len(replacementEffectDraws); got != 1 {
		t.Fatalf("moving puddle created %d effects, want one", got)
	}
	effect := replacementEffectDraws[key]
	if effect.started != started || effect.left != 62 || effect.top != 92 || !effect.seen {
		t.Fatalf("moving puddle restarted or lost its anchor: %#v", effect)
	}
	picture.PictID = 889
	if got := replacementEffectGroundPictureInstanceKey(picture); got == identity {
		t.Fatal("different puddle shapes shared an instance identity")
	}
	beginReplacementEffects()
	drawReplacementEffectsLayer(nil, 0, 0, nil, nil, 0, 0, 0, true)
	if len(replacementEffectDraws) != 0 {
		t.Fatal("puddle retained a stale cycle after its source picture disappeared")
	}
}

func TestTownPuddleReflectionProximity(t *testing.T) {
	const width, height, mobileSize = 84.0, 13.0, 64.0
	if got := townPuddleReflectionProximity(0, 0, width, height, mobileSize); got != 1 {
		t.Fatalf("centered mobile proximity = %v, want 1", got)
	}
	if got := townPuddleReflectionProximity(120, 0, width, height, mobileSize); got != 0 {
		t.Fatalf("distant mobile proximity = %v, want 0", got)
	}
	if got := townPuddleReflectionProximity(0, 90, width, height, mobileSize); got != 0 {
		t.Fatalf("mobile on different ground proximity = %v, want 0", got)
	}
	if got := townPuddleReflectionProximity(20, 0, width, height, mobileSize); got <= 0 || got >= 1 {
		t.Fatalf("nearby mobile proximity = %v, want a partial reflection", got)
	}
}

func TestTownPuddleMobileMotionIgnoresCameraShift(t *testing.T) {
	previous := map[uint8]frameMobile{7: {Index: 7, H: 30, V: 40}}
	if got := townPuddleMobileMotion(frameMobile{Index: 7, H: 35, V: 43}, previous, 5, 3); got != 0 {
		t.Fatalf("camera-only displacement produced foot ripples: %v", got)
	}
	if got := townPuddleMobileMotion(frameMobile{Index: 7, H: 40, V: 43}, previous, 5, 3); got <= 0 {
		t.Fatalf("mobile movement relative to ground produced no ripples: %v", got)
	}
	if got := townPuddleMobileMotion(frameMobile{Index: 8, H: 40, V: 43}, previous, 5, 3); got != 0 {
		t.Fatalf("mobile without previous position produced foot ripples: %v", got)
	}
	if got := townPuddleMobileMotion(frameMobile{Index: 7, H: 140, V: 43}, previous, 5, 3); got != 0 {
		t.Fatalf("teleport produced foot ripples: %v", got)
	}
}

func TestTownPuddleFootRippleFollowsMovementAndLingers(t *testing.T) {
	originalScale := gs.GameScale
	gs.GameScale = 2
	t.Cleanup(func() { gs.GameScale = originalScale })
	now := time.Unix(1_000, 0)
	var effect replacementEffectDraw
	effect.recordTownPuddleFoot(7, 30, 5, 1, now)
	effect.recordTownPuddleFoot(7, 31, 5, 1, now.Add(16*time.Millisecond))
	if !effect.puddleRipples[0].started.IsZero() {
		t.Fatal("stationary/subpixel foot motion emitted a ripple")
	}
	effect.recordTownPuddleFoot(7, 32, 5, 1, now.Add(32*time.Millisecond))
	if effect.puddleRipples[0].started.IsZero() || effect.puddleRipples[0].x != 32 {
		t.Fatal("moving foot did not start a ripple at its contact point")
	}
	effect.recordTownPuddleFoot(7, 32, 5, 1, now.Add(48*time.Millisecond))
	if effect.puddleRippleNext != 1 {
		t.Fatal("stationary foot restarted its ripple")
	}
	var state replacementEffectShaderState
	populateTownPuddleRippleUniforms(&effect, now.Add(432*time.Millisecond), &state)
	if state.rippleMotion[0] <= 0 || math.Abs(float64(state.rippleAges[0]-0.4)) > 0.0001 {
		t.Fatalf("ripple vanished while expanding: motion=%v age=%v", state.rippleMotion[0], state.rippleAges[0])
	}
	state.rippleMotion = [6]float32{}
	populateTownPuddleRippleUniforms(&effect, now.Add(1100*time.Millisecond), &state)
	if state.rippleMotion[0] != 0 {
		t.Fatal("expired foot ripple remained active")
	}
	effect.recordTownPuddleFoot(7, 200, 5, 1, now.Add(60*time.Millisecond))
	if effect.puddleRippleNext != 1 {
		t.Fatal("teleport emitted a puddle ripple")
	}
}

func TestTownPuddleReflectionVerticalPosition(t *testing.T) {
	const puddleTop, height, footFraction = 90.0, 20.0, 0.9
	const footY = 100.0
	centered := townPuddleReflectionVerticalTop(footY, puddleTop, height, footFraction)
	reflectedHeight := height * 0.82
	if got, want := centered-reflectedHeight*footFraction, footY-puddleTop; math.Abs(got-want) > 0.0001 {
		t.Fatalf("reflected foot row = %v, want contact point %v", got, want)
	}
	above := townPuddleReflectionVerticalTop(footY-10, puddleTop, height, footFraction)
	below := townPuddleReflectionVerticalTop(footY+10, puddleTop, height, footFraction)
	if above >= centered || below <= centered {
		t.Fatalf("ground reflection did not follow vertical movement: above=%v centered=%v below=%v", above, centered, below)
	}
	if math.Abs((above-centered)+10) > 0.0001 || math.Abs((below-centered)-10) > 0.0001 {
		t.Fatalf("ground reflection was not translated by the mobile's screen delta: above=%v centered=%v below=%v", above, centered, below)
	}
	if math.Abs((centered-above)-(below-centered)) > 0.0001 {
		t.Fatalf("reflection vertical movement is asymmetric: above=%v centered=%v below=%v", above, centered, below)
	}
	if got := townPuddleReflectionVerticalTop(footY, puddleTop, height, 0.75) - reflectedHeight*0.75; math.Abs(got-(footY-puddleTop)) > 0.0001 {
		t.Fatalf("different mobile pose foot row = %v, want %v", got, footY-puddleTop)
	}
}

func TestShoreWaveDirections(t *testing.T) {
	tests := []struct {
		id        uint16
		direction float32
	}{
		{id: 3568, direction: 2},
		{id: 3569, direction: 3},
		{id: 3570, direction: 0},
		{id: 3571, direction: 1},
	}
	for _, test := range tests {
		if got := replacementEffectWaveDirection(test.id); got != test.direction {
			t.Errorf("wave %d direction = %v, want %v", test.id, got, test.direction)
		}
	}
	if !replacementEffectIsPersistent(replacementEffectShoreWave) {
		t.Fatal("shore wave must persist while its source picture is present")
	}
	if replacementEffectUsesPlayerMask(replacementEffectShoreWave) {
		t.Fatal("shore wave must not use a player mask")
	}
	if !replacementEffectDrawsBelowMobiles(replacementEffectShoreWave) {
		t.Fatal("shore wave must draw below mobiles")
	}
	if got, want := replacementEffectSequenceDuration(replacementEffectShoreWave), float32(12.8); got != want {
		t.Fatalf("shore wave duration = %v, want %v", got, want)
	}
	for frame := range 4 {
		if got := replacementEffectFrameStartOffset(replacementEffectShoreWave, 3568, frame); got != 0 {
			t.Errorf("shore wave frame %d start offset = %v, want zero", frame, got)
		}
	}
}

func TestConcurrentReplacementShaderInitializationDoesNotRepeatShaders(t *testing.T) {
	replacementEffectsShaderInitMu.Lock()
	originalReady := replacementEffectsShadersReady
	originalAttempted := replacementEffectsShaderInitAttempted
	originalIndex := replacementEffectsShaderInitIndex
	originalCompile := compileReplacementEffectShaderForInit
	replacementEffectsShadersReady = false
	replacementEffectsShaderInitAttempted = false
	replacementEffectsShaderInitIndex = 0
	var compileCount atomic.Int32
	compileReplacementEffectShaderForInit = func(string, []byte) (*ebiten.Shader, error) {
		compileCount.Add(1)
		return nil, nil
	}
	replacementEffectsShaderInitMu.Unlock()

	t.Cleanup(func() {
		replacementEffectsShaderInitMu.Lock()
		replacementEffectsShadersReady = originalReady
		replacementEffectsShaderInitAttempted = originalAttempted
		replacementEffectsShaderInitIndex = originalIndex
		compileReplacementEffectShaderForInit = originalCompile
		replacementEffectsShaderInitMu.Unlock()
	})

	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := loadNextReplacementEffectShader(); err != nil {
				t.Errorf("loadNextReplacementEffectShader: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := compileCount.Load(); got != replacementEffectsShaderCount {
		t.Fatalf("compiled %d replacement shaders, want %d", got, replacementEffectsShaderCount)
	}
	if !replacementEffectsShadersReady || !replacementEffectsShaderInitAttempted {
		t.Fatal("replacement shader initialization did not finish")
	}
	if replacementEffectsShaderInitIndex != replacementEffectsShaderCount {
		t.Fatalf("replacement shader index = %d, want %d", replacementEffectsShaderInitIndex, replacementEffectsShaderCount)
	}
}

func TestReplacementEffectKindLookup(t *testing.T) {
	tests := []struct {
		id   uint16
		kind replacementEffectKind
		ok   bool
	}{
		{id: 1759, kind: replacementEffectHealing, ok: true},
		{id: 481, kind: replacementEffectFirePlume, ok: true},
		{id: 33, kind: replacementEffectBloodGush, ok: true},
		{id: 482, kind: replacementEffectFirePlume, ok: true},
		{id: 572, kind: replacementEffectFirePlume, ok: true},
		{id: 1739, kind: replacementEffectFirePlume, ok: true},
		{id: 1740, kind: replacementEffectFirePlume, ok: true},
		{id: 1741, kind: replacementEffectFirePlume, ok: true},
		{id: 885, kind: replacementEffectWavingFlag, ok: true},
		{id: 5647, kind: replacementEffectWavingFlag, ok: true},
		{id: 330, kind: replacementEffectWallTorch, ok: true},
		{id: 331, kind: replacementEffectWallTorch, ok: true},
		{id: 1286, kind: replacementEffectMysticWard, ok: true},
		{id: 1587, kind: replacementEffectMysticOrbitWard, ok: true},
		{id: 597, kind: replacementEffectLavaPool, ok: true},
		{id: 598, kind: replacementEffectLavaPool, ok: true},
		{id: 888, kind: replacementEffectTownPuddle, ok: true},
		{id: 889, kind: replacementEffectTownPuddle, ok: true},
		{id: 3568, kind: replacementEffectShoreWave, ok: true},
		{id: 3569, kind: replacementEffectShoreWave, ok: true},
		{id: 3570, kind: replacementEffectShoreWave, ok: true},
		{id: 3571, kind: replacementEffectShoreWave, ok: true},
		{id: 965, ok: false},
		{id: 977, ok: false},
		{id: 978, ok: false},
		{id: 979, ok: false},
		{id: 1039, kind: replacementEffectMagicMoteRing, ok: true},
		{id: 1044, kind: replacementEffectMagicMoteRing, ok: true},
		{id: 445, kind: replacementEffectHiddenPath, ok: true},
		{id: 446, kind: replacementEffectHiddenPath, ok: true},
		{id: 2976, kind: replacementEffectTeleportGold, ok: true},
		{id: 2977, kind: replacementEffectTeleportBlue, ok: true},
		{id: 2978, kind: replacementEffectTeleportPrismatic, ok: true},
		{id: 3125, kind: replacementEffectStoneForm, ok: true},
		{id: coinRewardFirstPictID, kind: replacementEffectCoinReward, ok: true},
		{id: coinRewardLastPictID, kind: replacementEffectCoinReward, ok: true},
		{id: 1, ok: false},
	}
	for _, test := range tests {
		kind, ok := replacementEffectKindForPict(test.id)
		if kind != test.kind || ok != test.ok {
			t.Errorf("replacementEffectKindForPict(%d) = (%d, %v), want (%d, %v)", test.id, kind, ok, test.kind, test.ok)
		}
	}
}

var benchmarkReplacementEffectKind replacementEffectKind

func BenchmarkReplacementEffectKindLookup(b *testing.B) {
	ids := [...]uint16{1, 33, 330, 331, 445, 446, 481, 482, 539, 540, 572, 597, 598, 885, 888, 889, 1039, 1044, 1286, 1587, 1739, 1740, 1741, 1759, 1847, 2976, 2977, 2978, 3125, 3568, 3569, 3570, 3571, 5000, 5647}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		kind, _ := replacementEffectKindForPict(ids[i%len(ids)])
		benchmarkReplacementEffectKind = kind
	}
}
