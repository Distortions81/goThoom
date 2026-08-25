package main

import (
	_ "embed"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

const healingBurstPictID uint16 = 1759

//go:embed data/shaders/healing_burst.kage
var healingBurstShaderSource []byte

var healingBurstShader *ebiten.Shader
var replacementEffectsStarted = time.Now()

func init() {
	var err error
	healingBurstShader, err = ebiten.NewShader(healingBurstShaderSource)
	if err != nil {
		panic(err)
	}
}

// drawReplacementPictureEffect replaces selected legacy effect sprites while
// preserving their world position, scale, fade, and normal lighting behavior.
func drawReplacementPictureEffect(screen *ebiten.Image, pictID uint16, left, top, width, height float64, alpha float32) bool {
	if !gs.ReplacementEffects || pictID != healingBurstPictID || width <= 0 || height <= 0 {
		return false
	}
	w, h := int(math.Ceil(width)), int(math.Ceil(height))
	if w <= 0 || h <= 0 {
		return false
	}
	op := &ebiten.DrawRectShaderOptions{Uniforms: map[string]any{
		"Size":  []float32{float32(w), float32(h)},
		"Phase": float32(time.Since(replacementEffectsStarted).Seconds()),
		"Alpha": alpha,
	}}
	op.GeoM.Translate(left, top)
	screen.DrawRectShader(w, h, healingBurstShader, op)
	return true
}
