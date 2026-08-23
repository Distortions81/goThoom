package main

import (
	"image"
	"image/color"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestArtworkUpscaleCappedInPotatoMode(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })

	gs.SpriteUpscaleFilter = true
	gs.SpriteUpscaleMode = artworkUpscaleBalanced
	gs.GameScale = 4
	gs.PotatoGPU = false
	if !artworkUpscaleEnabled() || artworkUpscaleFactor() != 4 {
		t.Fatal("artwork upscale should use the full render scale normally")
	}
	gs.PotatoGPU = true
	if !artworkUpscaleEnabled() || artworkUpscaleFactor() != 2 {
		t.Fatal("potato mode should retain artwork upscaling capped at 2x")
	}
	gs.SpriteUpscaleFilter = false
	if artworkUpscaleEnabled() {
		t.Fatal("disabled artwork upscale should remain disabled in potato mode")
	}
}

func TestArtworkUpscaleStyles(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })

	tests := []struct {
		mode     int
		enabled  bool
		reach    float32
		strength float32
	}{
		{mode: artworkUpscaleOff, enabled: false, reach: 0, strength: 0},
		{mode: artworkUpscaleCrisp, enabled: true, reach: 1.35, strength: 0.65},
		{mode: artworkUpscaleBalanced, enabled: true, reach: 1.65, strength: 0.8},
		{mode: artworkUpscaleSmooth, enabled: true, reach: 2.75, strength: 1},
		{mode: artworkUpscaleUltraSmooth, enabled: true, reach: 2.75, strength: 0.82},
	}
	for _, tt := range tests {
		setArtworkUpscaleMode(tt.mode)
		if artworkUpscaleEnabled() != tt.enabled {
			t.Errorf("mode %d enabled = %v, want %v", tt.mode, artworkUpscaleEnabled(), tt.enabled)
		}
		if got := artworkUpscaleCornerReach(); got != tt.reach {
			t.Errorf("mode %d corner reach = %v, want %v", tt.mode, got, tt.reach)
		}
		if got := artworkUpscaleBlendStrength(); got != tt.strength {
			t.Errorf("mode %d blend strength = %v, want %v", tt.mode, got, tt.strength)
		}
	}
}

func TestKageArtworkUpscaleCreatesRequestedResolution(t *testing.T) {
	originalSettings := gs
	gs.PotatoGPU = false
	t.Cleanup(func() { gs = originalSettings })

	src := ebiten.NewImage(3, 5)
	src.Fill(color.RGBA{R: 255, A: 255})
	for factor := 2; factor <= 4; factor++ {
		scaled := upscaleSpriteImage(src, factor)
		if got, want := scaled.Bounds().Size(), image.Pt(3*factor, 5*factor); got != want {
			t.Fatalf("%dx shader upscale size = %v, want %v", factor, got, want)
		}
		scaled.Deallocate()
	}
}

func TestKageMobileUpscaleCacheReusesTexture(t *testing.T) {
	originalSettings := gs
	gs.GameScale = 2
	gs.PotatoGPU = false
	gs.SpriteUpscaleFilter = true
	gs.SpriteUpscaleMode = artworkUpscaleBalanced
	imageMu.Lock()
	originalCache := scaledMobileCache
	scaledMobileCache = make(map[scaledMobileKey]*ebiten.Image)
	imageMu.Unlock()
	t.Cleanup(func() {
		imageMu.Lock()
		scaledMobileCache = originalCache
		imageMu.Unlock()
		gs = originalSettings
	})

	src := ebiten.NewImage(4, 4)
	key := makeMobileKey(447, 0, nil)
	first := getScaledMobileFrame(key, src)
	second := getScaledMobileFrame(key, src)
	if first != second {
		t.Fatal("Kage-upscaled mobile texture was not reused")
	}
	if got := first.Bounds().Size(); got != image.Pt(8, 8) {
		t.Fatalf("cached Kage texture size = %v, want 8x8", got)
	}
	setArtworkUpscaleMode(artworkUpscaleSmooth)
	smooth := getScaledMobileFrame(key, src)
	if smooth == first {
		t.Fatal("smooth mode reused the balanced cached texture")
	}
}
