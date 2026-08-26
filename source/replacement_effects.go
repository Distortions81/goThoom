package main

import (
	_ "embed"
	"math"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

var healingBurstPictIDs = map[uint16]struct{}{
	1759: {},
	1760: {},
}

//go:embed data/shaders/healing_burst.kage
var healingBurstShaderSource []byte

var healingBurstShader *ebiten.Shader
var replacementEffectsStarted = time.Now()

type replacementEffectDraw struct {
	pictID                   uint16
	left, top, width, height float64
	alpha                    float32
	started, lastSeen        time.Time
	seen                     bool
	mask                     *ebiten.Image
	hasMask                  bool
}

var replacementEffectDraws = make(map[uint64]replacementEffectDraw)

const (
	replacementEffectFadeIn  = 180 * time.Millisecond
	replacementEffectFadeOut = 280 * time.Millisecond
)

func init() {
	if err := ReloadReplacementEffectsShader(); err != nil {
		panic(err)
	}
}

// ReloadReplacementEffectsShader recompiles the replacement-effects shader
// from disk. The embedded source keeps release builds self-contained.
func ReloadReplacementEffectsShader() error {
	source := healingBurstShaderSource
	if b, err := os.ReadFile("data/shaders/healing_burst.kage"); err == nil {
		source = b
	}
	shader, err := ebiten.NewShader(source)
	if err != nil {
		return err
	}
	healingBurstShader = shader
	return nil
}

func beginReplacementEffects() {
	for key, effect := range replacementEffectDraws {
		effect.seen = false
		replacementEffectDraws[key] = effect
	}
}

func replacementEffectReplacesPict(id uint16) bool {
	if !gs.ReplacementEffects {
		return false
	}
	_, ok := healingBurstPictIDs[id]
	return ok
}

// queueReplacementPictureEffect preserves the legacy effect's world anchor
// while deferring the visual to a full-bright pass after scene lighting.
func queueReplacementPictureEffect(pictID uint16, h, v int16, instanceKey uint64, left, top, width, height float64, alpha float32, mobileImg *ebiten.Image, mobileX, mobileY, mobileSize float64) bool {
	if !replacementEffectReplacesPict(pictID) || width <= 0 || height <= 0 {
		return false
	}
	now := drawFrameNow
	if now.IsZero() {
		now = time.Now()
	}
	// 1759 and 1760 are two legacy representations of the same healing
	// family. Prefer the pinned mobile identity so movement does not restart
	// the effect; unpinned effects fall back to their world position.
	key := instanceKey
	if key == 0 {
		key = uint64(uint16(h))<<16 | uint64(uint16(v))
	}
	effect, ok := replacementEffectDraws[key]
	if !ok || now.Sub(effect.lastSeen) > replacementEffectFadeOut {
		effect = replacementEffectDraw{pictID: pictID, started: now}
	}
	effect.left, effect.top = left, top
	effect.width, effect.height = width, height
	effect.alpha = alpha
	effect.lastSeen = now
	effect.seen = true
	updateReplacementEffectMask(&effect, mobileImg, mobileX, mobileY, mobileSize)
	replacementEffectDraws[key] = effect
	return true
}

func updateReplacementEffectMask(effect *replacementEffectDraw, mobileImg *ebiten.Image, mobileX, mobileY, mobileSize float64) {
	effect.hasMask = false
	w, h := int(math.Ceil(effect.width)), int(math.Ceil(effect.height))
	if w <= 0 || h <= 0 {
		return
	}
	if effect.mask == nil || effect.mask.Bounds().Dx() != w || effect.mask.Bounds().Dy() != h {
		if effect.mask != nil {
			effect.mask.Deallocate()
		}
		effect.mask = newImage(w, h)
	}
	effect.mask.Clear()
	// The shader references image slot 0 even when HasMask is false, so Kage
	// still requires a bound texture for unmatched or unpinned effects.
	if mobileImg == nil || mobileSize <= 0 {
		return
	}
	bounds := mobileImg.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}
	op := acquireDrawOpts()
	op.Filter = worldArtworkFilter()
	op.DisableMipmaps = true
	scale := mobileSize / float64(bounds.Dx())
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(
		mobileX-mobileSize/2-effect.left,
		mobileY-mobileSize/2-effect.top,
	)
	effect.mask.DrawImage(mobileImg, op)
	releaseDrawOpts(op)
	effect.hasMask = true
}

func drawReplacementEffects(screen *ebiten.Image) {
	now := drawFrameNow
	if now.IsZero() {
		now = time.Now()
	}
	for key, effect := range replacementEffectDraws {
		fadeIn := replacementEffectEase(float32(now.Sub(effect.started)) / float32(replacementEffectFadeIn))
		fadeOut := float32(1)
		if !effect.seen {
			age := now.Sub(effect.lastSeen)
			if age >= replacementEffectFadeOut {
				if effect.mask != nil {
					effect.mask.Deallocate()
				}
				delete(replacementEffectDraws, key)
				continue
			}
			fadeOut = 1 - replacementEffectEase(float32(age)/float32(replacementEffectFadeOut))
		}
		energy := fadeIn * fadeOut
		if energy <= 0 {
			continue
		}
		// Keep the rectangle identical to the alpha-mask texture. The shader
		// performs the visual wind-up scale around its own center.
		w, h := int(math.Ceil(effect.width)), int(math.Ceil(effect.height))
		if w <= 0 || h <= 0 {
			continue
		}
		phase := float32(time.Since(replacementEffectsStarted).Seconds())
		hasMask := float32(0)
		if effect.hasMask {
			hasMask = 1
		}
		// Light the mobile itself in a separate additive pass. This preserves
		// the original sprite detail while lifting its opaque pixels out of the
		// dusk-darkened scene with the burst's blue light.
		if effect.hasMask {
			lightOp := &ebiten.DrawRectShaderOptions{
				Blend: ebiten.BlendLighter,
				Uniforms: map[string]any{
					"Size":            []float32{float32(w), float32(h)},
					"Phase":           phase,
					"Alpha":           effect.alpha * energy,
					"Energy":          energy,
					"HasMask":         hasMask,
					"SpriteLightOnly": float32(1),
				},
			}
			lightOp.Images[0] = effect.mask
			lightOp.GeoM.Translate(effect.left, effect.top)
			screen.DrawRectShader(w, h, healingBurstShader, lightOp)
		}
		op := &ebiten.DrawRectShaderOptions{Uniforms: map[string]any{
			"Size":            []float32{float32(w), float32(h)},
			"Phase":           phase,
			"Alpha":           effect.alpha * energy,
			"Energy":          energy,
			"HasMask":         hasMask,
			"SpriteLightOnly": float32(0),
		}}
		op.Images[0] = effect.mask
		op.GeoM.Translate(effect.left, effect.top)
		screen.DrawRectShader(w, h, healingBurstShader, op)
	}
}

func replacementEffectEase(t float32) float32 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}
