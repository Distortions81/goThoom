package main

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

type mobilePaletteShaderState struct {
	key        mobileKey
	r, g, b, a [maxColors]float32
	op         ebiten.DrawTrianglesShaderOptions
}

type mobilePalettePairKey struct {
	previous mobileKey
	current  mobileKey
}

type mobilePaletteBlendShaderState struct {
	op ebiten.DrawTrianglesShaderOptions
}

var mobilePaletteBlendCache = make(map[mobilePalettePairKey]*mobilePaletteBlendShaderState)

func mobileGPURecolorEligible(id uint16, colors []byte) bool {
	return len(colors) != 0 && clImages != nil && clImages.HasCustomColors(uint32(id)) &&
		gs.ShadersEnabled && !gs.DenoiseImages && mobileRecolorShader != nil && mobileRecolorBlendShader != nil
}

func mobileRecolorSharedKey(id uint16, state uint8) mobileKey {
	return makeMobileKey(id, state, nil)
}

func mobileRecolorMaskFor(key mobileKey) *ebiten.Image {
	factor, mode := 1, artworkUpscaleOff
	if artworkUpscaleEnabled() {
		factor, mode = artworkUpscaleFactor(), artworkUpscaleMode()
	}
	key.colorsLen = 0
	clear(key.colors[:])
	imageMu.Lock()
	mask := mobileRecolorMaskCache[scaledMobileKey{mobileKey: key, scale: uint8(factor), mode: uint8(mode)}]
	imageMu.Unlock()
	return mask
}

func loadGPURecoloredMobileFrame(id uint16, state uint8, colors []byte) (base, influence *ebiten.Image, palette *mobilePaletteShaderState, ok bool) {
	if !mobileGPURecolorEligible(id, colors) {
		return nil, nil, nil, false
	}
	key := mobileRecolorSharedKey(id, state)
	base = loadMobileFrame(id, state, nil)
	base = getScaledMobileFrame(key, base)
	if base == nil {
		return nil, nil, nil, false
	}
	influence = mobileRecolorMaskFor(key)
	if influence == nil || influence.Bounds().Size() != base.Bounds().Size() {
		return nil, nil, nil, false
	}
	return base, influence, mobilePaletteState(id, colors), true
}

func mobilePaletteState(id uint16, colors []byte) *mobilePaletteShaderState {
	key := makeMobileKey(id, 0, colors)
	imageMu.Lock()
	if cached := mobilePaletteDeltaCache[key]; cached != nil {
		imageMu.Unlock()
		return cached
	}
	imageMu.Unlock()

	state := &mobilePaletteShaderState{key: key}
	deltas := clImages.CustomPaletteDeltas(uint32(id), colors)
	for slot := 0; slot < maxColors && slot*4+3 < len(deltas); slot++ {
		state.r[slot] = deltas[slot*4]
		state.g[slot] = deltas[slot*4+1]
		state.b[slot] = deltas[slot*4+2]
		state.a[slot] = deltas[slot*4+3]
	}
	state.op.Uniforms = map[string]any{
		"PaletteR": state.r[:],
		"PaletteG": state.g[:],
		"PaletteB": state.b[:],
		"PaletteA": state.a[:],
	}
	imageMu.Lock()
	if cached := mobilePaletteDeltaCache[key]; cached != nil {
		imageMu.Unlock()
		return cached
	}
	mobilePaletteDeltaCache[key] = state
	imageMu.Unlock()
	return state
}

func mobilePaletteBlendState(previous, current *mobilePaletteShaderState) *mobilePaletteBlendShaderState {
	key := mobilePalettePairKey{previous: previous.key, current: current.key}
	imageMu.Lock()
	if cached := mobilePaletteBlendCache[key]; cached != nil {
		imageMu.Unlock()
		return cached
	}
	imageMu.Unlock()
	state := &mobilePaletteBlendShaderState{}
	state.op.Uniforms = map[string]any{
		"PreviousPaletteR": previous.r[:], "PreviousPaletteG": previous.g[:],
		"PreviousPaletteB": previous.b[:], "PreviousPaletteA": previous.a[:],
		"CurrentPaletteR": current.r[:], "CurrentPaletteG": current.g[:],
		"CurrentPaletteB": current.b[:], "CurrentPaletteA": current.a[:],
	}
	imageMu.Lock()
	if cached := mobilePaletteBlendCache[key]; cached != nil {
		imageMu.Unlock()
		return cached
	}
	mobilePaletteBlendCache[key] = state
	imageMu.Unlock()
	return state
}

func mobileShaderVertices(bounds image.Rectangle, options frameBlendDrawOptions, custom [4]float32) [4]ebiten.Vertex {
	left := float32(options.Left)
	top := float32(options.Top)
	right := float32(options.Left + float64(bounds.Dx())*options.ScaleX)
	bottom := float32(options.Top + float64(bounds.Dy())*options.ScaleY)
	sourceLeft := float32(bounds.Min.X)
	sourceTop := float32(bounds.Min.Y)
	sourceRight := float32(bounds.Max.X)
	sourceBottom := float32(bounds.Max.Y)
	vertices := [4]ebiten.Vertex{
		{DstX: left, DstY: top, SrcX: sourceLeft, SrcY: sourceTop},
		{DstX: right, DstY: top, SrcX: sourceRight, SrcY: sourceTop},
		{DstX: left, DstY: bottom, SrcX: sourceLeft, SrcY: sourceBottom},
		{DstX: right, DstY: bottom, SrcX: sourceRight, SrcY: sourceBottom},
	}
	red, green, blue, alpha := premultipliedDrawColor(options.Red, options.Green, options.Blue, options.Alpha)
	for index := range vertices {
		vertices[index].ColorR, vertices[index].ColorG = red, green
		vertices[index].ColorB, vertices[index].ColorA = blue, alpha
		vertices[index].Custom0, vertices[index].Custom1 = custom[0], custom[1]
		vertices[index].Custom2, vertices[index].Custom3 = custom[2], custom[3]
	}
	return vertices
}

func drawRecoloredMobile(destination, base, influence *ebiten.Image, palette *mobilePaletteShaderState, options frameBlendDrawOptions) bool {
	if destination == nil || base == nil || influence == nil || palette == nil || mobileRecolorShader == nil {
		return false
	}
	linear := float32(0)
	if options.Linear {
		linear = 1
	}
	vertices := mobileShaderVertices(base.Bounds(), options, [4]float32{linear})
	op := &palette.op
	op.Images[0], op.Images[1] = base, influence
	destination.DrawTrianglesShader32(vertices[:], frameBlendIndices, mobileRecolorShader, op)
	return true
}

func drawRecoloredMobileFrameBlend(destination, previous, previousInfluence, current, currentInfluence *ebiten.Image, previousPalette, currentPalette *mobilePaletteShaderState, options frameBlendDrawOptions) bool {
	if destination == nil || previous == nil || previousInfluence == nil || current == nil || currentInfluence == nil ||
		previousPalette == nil || currentPalette == nil || mobileRecolorBlendShader == nil {
		return false
	}
	previousBounds, currentBounds := previous.Bounds(), current.Bounds()
	width, height := max(previousBounds.Dx(), currentBounds.Dx()), max(previousBounds.Dy(), currentBounds.Dy())
	if width < 1 || height < 1 {
		return false
	}
	previousOffsetX := float32((width - previousBounds.Dx()) / 2)
	previousOffsetY := float32((height - previousBounds.Dy()) / 2)
	currentOffsetX := float32((width - currentBounds.Dx()) / 2)
	currentOffsetY := float32((height - currentBounds.Dy()) / 2)
	linear := float32(0)
	if options.Linear {
		linear = 1
	}
	custom := [4]float32{options.Fade, previousOffsetX - currentOffsetX, previousOffsetY - currentOffsetY, linear}
	virtualBounds := image.Rect(
		previousBounds.Min.X-int(previousOffsetX),
		previousBounds.Min.Y-int(previousOffsetY),
		previousBounds.Min.X-int(previousOffsetX)+width,
		previousBounds.Min.Y-int(previousOffsetY)+height,
	)
	vertices := mobileShaderVertices(virtualBounds, options, custom)
	state := mobilePaletteBlendState(previousPalette, currentPalette)
	op := &state.op
	op.Images[0], op.Images[1] = previous, previousInfluence
	op.Images[2], op.Images[3] = current, currentInfluence
	destination.DrawTrianglesShader32(vertices[:], frameBlendIndices, mobileRecolorBlendShader, op)
	return true
}
