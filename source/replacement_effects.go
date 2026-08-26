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

var mysticWardPictIDs = map[uint16]struct{}{
	1286: {},
}

//go:embed data/shaders/healing_burst.kage
var healingBurstShaderSource []byte

//go:embed data/shaders/mystic_ward.kage
var mysticWardShaderSource []byte

var healingBurstShader *ebiten.Shader
var mysticWardShader *ebiten.Shader
var replacementEffectsStarted = time.Now()

type replacementEffectKind uint8

const (
	replacementEffectHealing replacementEffectKind = iota + 1
	replacementEffectMysticWard
)

type replacementEffectDraw struct {
	pictID                   uint16
	kind                     replacementEffectKind
	left, top, width, height float64
	alpha                    float32
	started, lastSeen        time.Time
	seen                     bool
	mobileIndex              uint8
	hasMobileAnchor          bool
	mobileOffsetX            float64
	mobileOffsetY            float64
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
	healingSource := healingBurstShaderSource
	if b, err := os.ReadFile("data/shaders/healing_burst.kage"); err == nil {
		healingSource = b
	}
	healingShader, err := ebiten.NewShader(healingSource)
	if err != nil {
		return err
	}
	wardSource := mysticWardShaderSource
	if b, err := os.ReadFile("data/shaders/mystic_ward.kage"); err == nil {
		wardSource = b
	}
	wardShader, err := ebiten.NewShader(wardSource)
	if err != nil {
		return err
	}
	healingBurstShader = healingShader
	mysticWardShader = wardShader
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
	_, ok := replacementEffectKindForPict(id)
	return ok
}

func replacementEffectKindForPict(id uint16) (replacementEffectKind, bool) {
	if _, ok := healingBurstPictIDs[id]; ok {
		return replacementEffectHealing, true
	}
	if _, ok := mysticWardPictIDs[id]; ok {
		return replacementEffectMysticWard, true
	}
	return 0, false
}

func replacementEffectShader(kind replacementEffectKind) *ebiten.Shader {
	if kind == replacementEffectMysticWard {
		return mysticWardShader
	}
	return healingBurstShader
}

// queueReplacementPictureEffect preserves the legacy effect's world anchor
// while deferring the visual to a full-bright pass after scene lighting.
func queueReplacementPictureEffect(pictID uint16, h, v int16, instanceKey uint64, left, top, width, height float64, alpha float32, mobileImg *ebiten.Image, mobileX, mobileY, mobileSize float64) bool {
	if !replacementEffectReplacesPict(pictID) || width <= 0 || height <= 0 {
		return false
	}
	kind, _ := replacementEffectKindForPict(pictID)
	now := drawFrameNow
	if now.IsZero() {
		now = time.Now()
	}
	// 1759 and 1760 are two legacy representations of the same healing
	// family. Prefer the pinned mobile identity so movement does not restart
	// the effect; unpinned effects fall back to their world position.
	key := instanceKey
	if key != 0 {
		key |= uint64(kind) << 56
	} else {
		key = uint64(uint16(h))<<16 | uint64(uint16(v))
		key |= uint64(kind) << 48
	}
	effect, ok := replacementEffectDraws[key]
	if !ok || now.Sub(effect.lastSeen) > replacementEffectFadeOut {
		effect = replacementEffectDraw{pictID: pictID, kind: kind, started: now}
	}
	effect.left, effect.top = left, top
	effect.width, effect.height = width, height
	effect.alpha = alpha
	effect.lastSeen = now
	effect.seen = true
	effect.hasMobileAnchor = instanceKey != 0
	if effect.hasMobileAnchor {
		effect.mobileIndex = uint8(instanceKey)
		effect.mobileOffsetX = left - mobileX
		effect.mobileOffsetY = top - mobileY
	}
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

func drawReplacementEffects(screen *ebiten.Image, ox, oy int, mobiles []frameMobile, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64) {
	now := drawFrameNow
	if now.IsZero() {
		now = time.Now()
	}
	for key, effect := range replacementEffectDraws {
		if effect.hasMobileAnchor {
			for _, mobile := range mobiles {
				if mobile.Index != effect.mobileIndex {
					continue
				}
				x, y := mobileScreenPosition(ox, oy, mobile, prevMobiles, shiftX, shiftY, alpha, maxMobileInterpPixels)
				effect.left = float64(x) + effect.mobileOffsetX
				effect.top = float64(y) + effect.mobileOffsetY
				break
			}
		}
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
		shader := replacementEffectShader(effect.kind)
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
			screen.DrawRectShader(w, h, shader, lightOp)
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
		screen.DrawRectShader(w, h, shader, op)
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
