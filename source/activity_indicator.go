package main

import (
	"image"
	"image/color"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type clientActivity uint32

const (
	clientActivityNone clientActivity = 0
	clientActivityData clientActivity = 1 << iota
	clientActivityAudio
	clientActivityGPU
)

const clientActivityIndicatorRadius = float32(5)
const clientActivityIndicatorInset = float32(11)
const clientActivityIndicatorSpacing = float32(14)

var (
	clientActivityIndicatorsEnabled atomic.Bool
	clientActivityPending           atomic.Uint32
	clientActivityColors            = map[clientActivity]color.RGBA{
		clientActivityData:  {R: 52, G: 211, B: 104, A: 255},
		clientActivityAudio: {R: 255, G: 176, B: 32, A: 255},
		clientActivityGPU:   {R: 239, G: 68, B: 68, A: 255},
	}
)

func noteClientActivity(activity clientActivity) {
	if !clientActivityIndicatorsEnabled.Load() {
		return
	}
	clientActivityPending.Or(uint32(activity))
}

func setClientActivityIndicatorsEnabled(enabled bool) {
	clientActivityIndicatorsEnabled.Store(enabled)
	if !enabled {
		clientActivityPending.Store(0)
	}
}

func takeClientActivity() clientActivity {
	if !clientActivityIndicatorsEnabled.Load() {
		return clientActivityNone
	}
	return clientActivity(clientActivityPending.Swap(0))
}

func clientActivityIndicatorPosition(bounds image.Rectangle, slot int) (float32, float32) {
	return float32(bounds.Max.X) - clientActivityIndicatorInset - float32(slot)*clientActivityIndicatorSpacing,
		float32(bounds.Max.Y) - clientActivityIndicatorInset
}

func drawClientActivityIndicators(screen *ebiten.Image, activity clientActivity) {
	if screen == nil || screen.Bounds().Empty() || activity == clientActivityNone {
		return
	}
	// Fixed left-to-right slots: artwork processing, audio decoding, GPU upload.
	// Multiple dots may light during the same frame when independent subsystems are active.
	for slot, candidate := range [...]clientActivity{clientActivityGPU, clientActivityAudio, clientActivityData} {
		if activity&candidate == 0 {
			continue
		}
		x, y := clientActivityIndicatorPosition(screen.Bounds(), slot)
		vector.FillCircle(screen, x, y, clientActivityIndicatorRadius+2, color.RGBA{R: 24, G: 24, B: 24, A: 220}, false)
		vector.FillCircle(screen, x, y, clientActivityIndicatorRadius, clientActivityColors[candidate], false)
	}
}
