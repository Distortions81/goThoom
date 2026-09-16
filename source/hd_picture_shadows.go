package main

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// hdPictureContactShadowProfile describes a small, source-footprint-relative
// ground contact shadow. These belong to HD foreground artwork whose original
// sprites did not carry a useful shadow.
type hdPictureContactShadowProfile struct {
	width  float64
	y      float64
	radius float64
	alpha  float64
}

var hdPictureContactShadowProfiles = map[uint16]hdPictureContactShadowProfile{
	41:   {width: 0.30, y: 0.90, radius: 0.030, alpha: 0.26}, // notice sign
	985:  {width: 0.76, y: 0.96, radius: 0.018, alpha: 0.23}, // Puddleby Temple
	2245: {width: 0.82, y: 0.90, radius: 0.045, alpha: 0.30}, // temple bench
	3785: {width: 0.66, y: 0.80, radius: 0.050, alpha: 0.18}, // white flowers
	3786: {width: 0.66, y: 0.80, radius: 0.050, alpha: 0.18}, // red flowers
}

func drawHDPictureContactShadow(screen *ebiten.Image, id uint16, left, top, width, height float64, fade float32) {
	profile, ok := hdPictureContactShadowProfiles[id]
	if !ok || screen == nil || width <= 0 || height <= 0 || fade <= 0 {
		return
	}

	centerX := left + width/2
	centerY := top + height*profile.y
	radius := max(0.5, height*profile.radius)
	halfWidth := max(radius, width*profile.width/2)
	alpha := profile.alpha * float64(fade)

	// Two compact capsules approximate a low, softly feathered ellipse without
	// a texture allocation or a shadow that can extend beyond the source frame.
	drawContactShadowCapsule(screen, centerX, centerY, halfWidth*1.08, radius*1.55, uint8(math.Round(alpha*0.35*255)))
	drawContactShadowCapsule(screen, centerX, centerY, halfWidth, radius, uint8(math.Round(alpha*0.65*255)))
}

func drawContactShadowCapsule(screen *ebiten.Image, centerX, centerY, halfWidth, radius float64, alpha uint8) {
	if alpha == 0 || radius <= 0 || halfWidth <= 0 {
		return
	}
	clr := color.RGBA{A: alpha}
	lineHalfWidth := max(0.0, halfWidth-radius)
	vector.FillRect(screen, float32(centerX-lineHalfWidth), float32(centerY-radius), float32(lineHalfWidth*2), float32(radius*2), clr, true)
	vector.FillCircle(screen, float32(centerX-lineHalfWidth), float32(centerY), float32(radius), clr, true)
	vector.FillCircle(screen, float32(centerX+lineHalfWidth), float32(centerY), float32(radius), clr, true)
}
