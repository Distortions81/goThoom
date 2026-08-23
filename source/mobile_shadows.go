package main

import (
	_ "embed"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"

	"gothoom/climg"
)

const (
	poseLie                = 41
	shadowDropOffset       = 5.0
	minimumShadowSunHeight = 7.0
	nominalNoonSunHeight   = 55.0
	minimumShadowContrast  = 0.70
	normalShadowOpacity    = 0.75
	detailedCoreOpacity    = 0.75
	// Treat distance from the visible contact edge in logical character pixels.
	// This keeps differently padded and scaled sprite sheets visually consistent.
	characterShadowSoftnessPerPixel = 1.0 / 16.0
	characterShadowMaxSoftness      = 3.0
	characterShadowFadePerPixel     = 0.65 / 48.0
	characterShadowMinimumOpacity   = 0.35
	characterShadowMinUpscale       = 2
)

//go:embed data/shaders/character_shadow_blur.kage
var characterShadowBlurShaderSource []byte

// shadowDarkenBlend directly attenuates the scene beneath the silhouette while
// preserving the destination alpha. This keeps ground color and detail visible.
var shadowDarkenBlend = ebiten.Blend{
	BlendFactorSourceRGB:        ebiten.BlendFactorZero,
	BlendFactorSourceAlpha:      ebiten.BlendFactorZero,
	BlendFactorDestinationRGB:   ebiten.BlendFactorOneMinusSourceAlpha,
	BlendFactorDestinationAlpha: ebiten.BlendFactorOne,
	BlendOperationRGB:           ebiten.BlendOperationAdd,
	BlendOperationAlpha:         ebiten.BlendOperationAdd,
}

// Detailed layers are combined by maximum opacity before touching the scene.
// This prevents the core, soft edges, and neighboring character shadows from
// repeatedly darkening the same ground pixels.
var shadowMaskBlend = ebiten.Blend{
	BlendOperationRGB:   ebiten.BlendOperationMax,
	BlendOperationAlpha: ebiten.BlendOperationMax,
}

var (
	uprightShadowVertices = [4]ebiten.Vertex{}
	uprightShadowIndices  = [6]uint16{0, 1, 2, 1, 3, 2}
	uprightShadowDrawOpts = ebiten.DrawTrianglesOptions{
		Filter: ebiten.FilterLinear,
		Blend:  shadowDarkenBlend,
	}
	detailedCharacterShadowMask *ebiten.Image
	characterShadowBlurShader   *ebiten.Shader
	characterShadowCacheMu      sync.Mutex
	characterShadowTextures     = make(map[characterShadowTextureKey]characterShadowTexture)
)

type characterShadowTextureKey struct {
	id      uint16
	state   uint8
	upscale uint8
	size    uint16
}

type characterShadowTexture struct {
	image       *ebiten.Image
	contentSize int
	padding     int
	footY       float64
}

func init() {
	var err error
	characterShadowBlurShader, err = ebiten.NewShader(characterShadowBlurShaderSource)
	if err != nil {
		panic(err)
	}
}

type characterShadowProjection struct {
	angle       float64
	length      float64
	dropOffsetX float64
	dropOffsetY float64
	contrast    float32
}

func normalizeShadowAzimuth(azimuth int) int {
	azimuth %= 360
	if azimuth < 0 {
		azimuth += 360
	}
	return azimuth
}

func chooseUprightShadowPose(state uint8, azimuth int) (uint8, bool) {
	if state < poseDead {
		facing := int(state) / 4
		subpose := int(state) & 3
		sunDirection := (normalizeShadowAzimuth(azimuth) + 23) / 45
		shadowFacing := (facing + sunDirection + 6) & 7
		// Rotate the silhouette relative to the sun while keeping its walk-cycle
		// frame matched to the visible character.
		return uint8(shadowFacing*4 + subpose), true
	}
	if state == poseDead || state == poseLie {
		return 0, false
	}
	return state, true
}

func characterShadowSunHeight(azimuth int) float64 {
	angle := float64(normalizeShadowAzimuth(azimuth))
	if angle > 180 {
		angle -= 180
	}
	if angle > 90 {
		angle = 180 - angle
	}
	// Match the classic OpenGL client: estimate elevation linearly from the
	// folded sun azimuth, reaching 55 degrees at local noon.
	elevation := angle / 90 * nominalNoonSunHeight
	if elevation < minimumShadowSunHeight {
		elevation = minimumShadowSunHeight
	} else if elevation > 85 {
		elevation = 85
	}
	return elevation
}

// uprightShadowLength returns the ground-shadow length for unit height. The
// minimum sun height prevents very long, heavily stretched silhouettes.
func uprightShadowLength(azimuth int) float64 {
	elevation := characterShadowSunHeight(azimuth)
	return math.Tan((90 - elevation) * math.Pi / 180)
}

func newCharacterShadowProjection(azimuth int) characterShadowProjection {
	angle := float64(azimuth) * math.Pi / 180
	elevation := characterShadowSunHeight(azimuth)
	daylight := elevation / nominalNoonSunHeight
	if daylight > 1 {
		daylight = 1
	}
	return characterShadowProjection{
		angle:       angle,
		length:      math.Tan((90 - elevation) * math.Pi / 180),
		dropOffsetX: -shadowDropOffset * math.Cos(angle) * gs.GameScale,
		dropOffsetY: shadowDropOffset * math.Sin(angle) * gs.GameScale,
		contrast:    float32(minimumShadowContrast + (1-minimumShadowContrast)*daylight),
	}
}

func currentCharacterShadowState() (float32, int, bool) {
	if !gs.CharacterShadows || gs.MaxNightLevel == 0 {
		return 0, 0, false
	}
	gNight.mu.Lock()
	level := gNight.Shadows
	azimuth := gNight.Azimuth
	gNight.mu.Unlock()
	if level <= 0 {
		return 0, 0, false
	}
	if level > 100 {
		level = 100
	}
	return float32(level) / 100, normalizeShadowAzimuth(azimuth), true
}

func drawMobileShadows(screen *ebiten.Image, ox, oy int, mobiles []frameMobile, descMap map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64, maxDist int) {
	shadowAlpha, azimuth, ok := currentCharacterShadowState()
	if !ok || clImages == nil {
		return
	}
	projection := newCharacterShadowProjection(azimuth)
	shadowTarget := screen
	shadowBlend := shadowDarkenBlend
	useMask := gs.DetailedCharacterShadows && !gs.PotatoGPU
	if useMask {
		shadowTarget = characterShadowMask(screen)
		shadowTarget.Clear()
		shadowBlend = shadowMaskBlend
	}

	for _, mobile := range mobiles {
		desc, ok := descMap[mobile.Index]
		if !ok {
			continue
		}
		state := mobile.State
		upright := clImages.Flags(uint32(desc.PictID))&climg.PictDefFlagUprightShadow != 0
		if upright {
			var casts bool
			state, casts = chooseUprightShadowPose(state, azimuth)
			if !casts {
				continue
			}
		}

		colors := playerColorsForDescriptor(desc)
		img := loadMobileFrame(desc.PictID, state, colors)
		key := makeMobileKey(desc.PictID, state, colors)
		if img == nil {
			continue
		}
		size := mobileSize(desc.PictID)
		if size == 0 {
			size = img.Bounds().Dx()
		}
		shadowTexture := characterShadowTextureFor(key, img, size)
		x, y := mobileScreenPosition(ox, oy, mobile, prevMobiles, shiftX, shiftY, alpha, maxDist)
		drawCharacterShadow(shadowTarget, shadowTexture, size, x, y, shadowAlpha, projection, upright, shadowBlend)
	}

	if useMask {
		op := acquireDrawOpts()
		op.Blend = shadowDarkenBlend
		screen.DrawImage(shadowTarget, op)
		releaseDrawOpts(op)
	}
}

func characterShadowMask(screen *ebiten.Image) *ebiten.Image {
	bounds := screen.Bounds()
	if detailedCharacterShadowMask == nil || detailedCharacterShadowMask.Bounds().Dx() != bounds.Dx() || detailedCharacterShadowMask.Bounds().Dy() != bounds.Dy() {
		if detailedCharacterShadowMask != nil {
			detailedCharacterShadowMask.Deallocate()
		}
		detailedCharacterShadowMask = ebiten.NewImage(bounds.Dx(), bounds.Dy())
	}
	return detailedCharacterShadowMask
}

func characterShadowTextureFor(key mobileKey, img *ebiten.Image, size int) characterShadowTexture {
	return characterShadowTextureForWithOpaqueBottom(key, img, size, -1)
}

// characterShadowTextureForWithOpaqueBottom accepts a source-pixel edge for
// tests. Production passes -1 and reads the cached sprite once on a cache miss.
func characterShadowTextureForWithOpaqueBottom(key mobileKey, img *ebiten.Image, size, opaqueBottom int) characterShadowTexture {
	if gs.PotatoGPU {
		return characterShadowTexture{image: img, contentSize: img.Bounds().Dx(), footY: float64(img.Bounds().Dy())}
	}
	upscale := characterShadowUpscaleFactor()
	// Custom palette colors do not change the silhouette alpha. Sharing these
	// textures prevents identical shadows from being processed and cached once
	// per player palette.
	cacheKey := characterShadowTextureKey{id: key.id, state: key.state, upscale: uint8(upscale), size: uint16(size)}
	characterShadowCacheMu.Lock()
	if cached, ok := characterShadowTextures[cacheKey]; ok {
		characterShadowCacheMu.Unlock()
		return cached
	}
	characterShadowCacheMu.Unlock()

	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	contentSize := w * upscale
	contentHeight := h * upscale
	shadowSource := img
	if upscale > 1 {
		shadowSource = ebiten.NewImage(contentSize, contentHeight)
		upscaleOp := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, DisableMipmaps: true}
		upscaleOp.GeoM.Scale(float64(upscale), float64(upscale))
		shadowSource.DrawImage(img, upscaleOp)
	}

	logicalPixelsPerTexel := float64(size) / float64(contentSize)
	padding := int(math.Ceil(characterShadowMaxSoftness / logicalPixelsPerTexel))
	var footY float32
	if opaqueBottom < 0 {
		footY = characterShadowFootY(img, upscale, padding)
	} else {
		footY = float32(padding + opaqueBottom*upscale)
	}
	textureW := contentSize + 2*padding
	textureH := contentHeight + 2*padding
	padded := ebiten.NewImage(textureW, textureH)
	padOp := &ebiten.DrawImageOptions{}
	padOp.GeoM.Translate(float64(padding), float64(padding))
	padded.DrawImage(shadowSource, padOp)
	if shadowSource != img {
		shadowSource.Deallocate()
	}

	uniforms := map[string]any{
		"Direction":        []float32{1, 0},
		"SoftnessPerPixel": float32(characterShadowSoftnessPerPixel),
		"RadiusLimit":      float32(characterShadowMaxSoftness / logicalPixelsPerTexel),
		"FadePerPixel":     float32(characterShadowFadePerPixel * logicalPixelsPerTexel),
		"MinimumOpacity":   float32(characterShadowMinimumOpacity),
		"FootY":            footY,
		"ApplyGradient":    float32(1),
	}
	op := &ebiten.DrawRectShaderOptions{Uniforms: uniforms}
	op.Images[0] = padded
	horizontal := ebiten.NewImage(textureW, textureH)
	horizontal.DrawRectShader(textureW, textureH, characterShadowBlurShader, op)

	uniforms["Direction"] = []float32{0, 1}
	uniforms["ApplyGradient"] = float32(0)
	op.Images[0] = horizontal
	blurred := ebiten.NewImage(textureW, textureH)
	blurred.DrawRectShader(textureW, textureH, characterShadowBlurShader, op)
	padded.Deallocate()
	horizontal.Deallocate()

	texture := characterShadowTexture{
		image:       blurred,
		contentSize: contentSize,
		padding:     padding,
		footY:       float64(footY),
	}
	characterShadowCacheMu.Lock()
	if cached, ok := characterShadowTextures[cacheKey]; ok {
		characterShadowCacheMu.Unlock()
		blurred.Deallocate()
		return cached
	}
	characterShadowTextures[cacheKey] = texture
	characterShadowCacheMu.Unlock()
	return texture
}

// characterShadowFootY anchors treatment to the silhouette rather than the
// bottom of its square sprite cell. Facings and poses can have different
// transparent padding even though their logical size is identical.
func characterShadowFootY(img *ebiten.Image, upscale, padding int) float32 {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w <= 0 || h <= 0 || upscale < 1 {
		return float32(padding)
	}
	pixels := make([]byte, w*h*4)
	img.ReadPixels(pixels)
	return characterShadowFootYFromPixels(pixels, w, h, upscale, padding)
}

func characterShadowFootYFromPixels(pixels []byte, w, h, upscale, padding int) float32 {
	bottom := -1
	for y := h - 1; y >= 0 && bottom < 0; y-- {
		for x := 0; x < w; x++ {
			if pixels[(y*w+x)*4+3] != 0 {
				bottom = y
				break
			}
		}
	}
	if bottom < 0 {
		bottom = h - 1
	}
	return float32(padding + (bottom+1)*upscale)
}

func characterShadowTreatmentAtDistance(distance float64) (softness, opacity float64) {
	if distance < 0 {
		distance = 0
	}
	softness = math.Min(distance*characterShadowSoftnessPerPixel, characterShadowMaxSoftness)
	opacity = math.Max(1-distance*characterShadowFadePerPixel, characterShadowMinimumOpacity)
	return softness, opacity
}

func characterShadowUpscaleFactor() int {
	if gs.PotatoGPU {
		return 1
	}
	factor := spriteUpscaleFactor()
	if factor < characterShadowMinUpscale {
		return characterShadowMinUpscale
	}
	return factor
}

func clearCharacterShadowCache() {
	if detailedCharacterShadowMask != nil {
		detailedCharacterShadowMask.Deallocate()
		detailedCharacterShadowMask = nil
	}
	characterShadowCacheMu.Lock()
	for _, texture := range characterShadowTextures {
		texture.image.Deallocate()
	}
	characterShadowTextures = make(map[characterShadowTextureKey]characterShadowTexture)
	characterShadowCacheMu.Unlock()
}

func drawCharacterShadow(screen *ebiten.Image, texture characterShadowTexture, size, x, y int, alpha float32, projection characterShadowProjection, upright bool, blend ebiten.Blend) {
	drawSize := texture.contentSize
	if drawSize <= 0 || size <= 0 {
		return
	}
	shadowAlpha := alpha * projection.contrast
	if gs.DetailedCharacterShadows {
		shadowAlpha *= detailedCoreOpacity
	} else {
		shadowAlpha *= normalShadowOpacity
	}
	drawCharacterShadowLayer(screen, texture, size, x, y, shadowAlpha, projection, upright, blend)
}

func drawCharacterShadowLayer(screen *ebiten.Image, texture characterShadowTexture, size, x, y int, alpha float32, projection characterShadowProjection, upright bool, blend ebiten.Blend) {
	img := texture.image
	drawSize := texture.contentSize
	if upright {
		geo := uprightShadowGeoMWithFoot(drawSize, texture.padding, texture.footY, size, x, y, projection)
		drawUprightShadowTexture(screen, texture, geo, alpha, blend)
		return
	}

	target := float64(roundToInt(float64(size) * gs.GameScale))
	baseScale := target / float64(drawSize)
	op := acquireDrawOpts()
	op.Filter = ebiten.FilterLinear
	op.DisableMipmaps = true
	op.Blend = blend
	op.ColorScale.Scale(0, 0, 0, alpha)
	op.GeoM.Scale(baseScale, baseScale)
	pad := float64(texture.padding) * baseScale
	op.GeoM.Translate(float64(x)-target/2-pad+projection.dropOffsetX, float64(y)-target/2-pad+projection.dropOffsetY)

	screen.DrawImage(img, op)
	releaseDrawOpts(op)
}

// drawUprightShadowTexture projects the cached, pre-filtered texture onto the ground.
func drawUprightShadowTexture(screen *ebiten.Image, texture characterShadowTexture, geo ebiten.GeoM, alpha float32, blend ebiten.Blend) {
	img := texture.image
	bounds := img.Bounds()
	w, h := float64(bounds.Dx()), float64(bounds.Dy())
	topLeftX, topLeftY := geo.Apply(0, 0)
	topRightX, topRightY := geo.Apply(w, 0)
	bottomLeftX, bottomLeftY := geo.Apply(0, h)
	bottomRightX, bottomRightY := geo.Apply(w, h)

	uprightShadowVertices = [4]ebiten.Vertex{
		{DstX: float32(topLeftX), DstY: float32(topLeftY), SrcX: float32(bounds.Min.X), SrcY: float32(bounds.Min.Y), ColorA: alpha},
		{DstX: float32(topRightX), DstY: float32(topRightY), SrcX: float32(bounds.Max.X), SrcY: float32(bounds.Min.Y), ColorA: alpha},
		{DstX: float32(bottomLeftX), DstY: float32(bottomLeftY), SrcX: float32(bounds.Min.X), SrcY: float32(bounds.Max.Y), ColorA: alpha},
		{DstX: float32(bottomRightX), DstY: float32(bottomRightY), SrcX: float32(bounds.Max.X), SrcY: float32(bounds.Max.Y), ColorA: alpha},
	}
	uprightShadowDrawOpts.Blend = blend
	screen.DrawTriangles(uprightShadowVertices[:], uprightShadowIndices[:], img, &uprightShadowDrawOpts)
}

func uprightShadowGeoM(drawSize, size, x, y int, projection characterShadowProjection) ebiten.GeoM {
	return uprightShadowGeoMWithPadding(drawSize, 0, size, x, y, projection)
}

func uprightShadowGeoMWithPadding(drawSize, padding, size, x, y int, projection characterShadowProjection) ebiten.GeoM {
	return uprightShadowGeoMWithFoot(drawSize, padding, float64(padding+drawSize), size, x, y, projection)
}

func uprightShadowGeoMWithFoot(drawSize, padding int, footY float64, size, x, y int, projection characterShadowProjection) ebiten.GeoM {
	target := float64(roundToInt(float64(size) * gs.GameScale))
	baseScale := target / float64(drawSize)
	contentFootY := footY - float64(padding)
	screenFootY := float64(y) - target/2 + contentFootY*baseScale

	var geo ebiten.GeoM
	geo.Translate(-float64(padding)-float64(drawSize)/2, -footY)
	// The classic OpenGL path swaps the left and right shadow vertices before
	// rotating the quad.
	geo.Scale(-baseScale, baseScale*projection.length)
	geo.Rotate(-(projection.angle + math.Pi/2))
	geo.Translate(float64(x), screenFootY)
	return geo
}
