package main

import (
	"image"
	"image/color"
	"math"

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
	contactShadowOpacity   = 0.55
	contactShadowWidth     = 0.78
	contactShadowHeight    = 0.24
	contactShadowTexSize   = 64
	characterShadowPadding = 1
	lyingShadowOffset      = 2.0
)

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

// Detailed shadows are combined by maximum opacity before touching the scene.
// This prevents neighboring character shadows from repeatedly darkening the
// same ground pixels.
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
	contactShadowTexture        *ebiten.Image
	characterShadowTextureCache = make(map[*ebiten.Image]characterShadowTexture)
)

type characterShadowKind uint8

const (
	characterShadowNone characterShadowKind = iota
	characterShadowDirectional
	characterShadowContact
)

type characterShadowTexture struct {
	image       *ebiten.Image
	contentSize int
	padding     int
	footY       float64
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

func isLyingShadowState(state uint8) bool {
	return state == poseDead || state == poseLie
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
	alpha, azimuth, kind := currentCharacterShadowRenderState()
	return alpha, azimuth, kind != characterShadowNone
}

func currentCharacterShadowRenderState() (float32, int, characterShadowKind) {
	if !gs.CharacterShadows || gs.MaxNightLevel == 0 {
		return 0, 0, characterShadowNone
	}
	gNight.mu.Lock()
	level := gNight.Shadows
	azimuth := gNight.Azimuth
	cloudy := gNight.Cloudy
	flags := gNight.Flags
	gNight.mu.Unlock()
	if cloudy || flags&kLightNoShadows != 0 {
		return contactShadowOpacity, normalizeShadowAzimuth(azimuth), characterShadowContact
	}
	if level <= 0 {
		return 0, 0, characterShadowNone
	}
	if level > 100 {
		level = 100
	}
	return float32(level) / 100, normalizeShadowAzimuth(azimuth), characterShadowDirectional
}

func drawMobileShadows(screen *ebiten.Image, ox, oy int, mobiles []frameMobile, descMap map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64, maxDist int) {
	shadowAlpha, azimuth, kind := currentCharacterShadowRenderState()
	if kind != characterShadowDirectional || clImages == nil {
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
		if isLyingShadowState(state) {
			continue
		}
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
		if img == nil {
			continue
		}
		img = getScaledMobileFrame(makeMobileKey(desc.PictID, state, colors), img)
		size := mobileSize(desc.PictID)
		if size == 0 {
			size = img.Bounds().Dx()
		}
		shadowTexture := characterShadowTextureFor(img)
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

func drawMobileImmediateShadow(screen *ebiten.Image, ox, oy int, mobile frameMobile, descMap map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64, maxDist int, shadowAlpha float32, kind characterShadowKind) {
	if kind == characterShadowNone {
		return
	}
	if isLyingShadowState(mobile.State) {
		drawMobileDropShadow(screen, ox, oy, mobile, descMap, prevMobiles, shiftX, shiftY, alpha, maxDist, shadowAlpha)
		return
	}
	if kind == characterShadowContact {
		drawMobileContactShadow(screen, ox, oy, mobile, descMap, prevMobiles, shiftX, shiftY, alpha, maxDist, shadowAlpha)
	}
}

func drawMobileDropShadow(screen *ebiten.Image, ox, oy int, mobile frameMobile, descMap map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64, maxDist int, shadowAlpha float32) {
	desc, ok := descMap[mobile.Index]
	if !ok {
		return
	}
	colors := playerColorsForDescriptor(desc)
	img := loadMobileFrame(desc.PictID, mobile.State, colors)
	if img == nil {
		return
	}
	key := makeMobileKey(desc.PictID, mobile.State, colors)
	img = getScaledMobileFrame(key, img)
	size := mobileSize(desc.PictID)
	if size <= 0 {
		size = img.Bounds().Dx()
	}
	texture := characterShadowTextureFor(img)
	x, y := mobileScreenPosition(ox, oy, mobile, prevMobiles, shiftX, shiftY, alpha, maxDist)
	projection := characterShadowProjection{
		dropOffsetX: lyingShadowOffset * gs.GameScale,
		dropOffsetY: lyingShadowOffset * gs.GameScale,
		contrast:    1,
	}
	drawCharacterShadow(screen, texture, size, x, y, shadowAlpha, projection, false, shadowDarkenBlend)
}

func drawMobileContactShadow(screen *ebiten.Image, ox, oy int, mobile frameMobile, descMap map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64, maxDist int, shadowAlpha float32) {
	if clImages == nil {
		return
	}
	desc, ok := descMap[mobile.Index]
	if !ok {
		return
	}
	size := mobileSize(desc.PictID)
	if size <= 0 {
		return
	}
	colors := playerColorsForDescriptor(desc)
	img := loadMobileFrame(desc.PictID, mobile.State, colors)
	if img == nil {
		return
	}
	key := makeMobileKey(desc.PictID, mobile.State, colors)
	img = getScaledMobileFrame(key, img)
	metrics := mobileSpriteMetricsFor(key, img)
	x, y := mobileScreenPosition(ox, oy, mobile, prevMobiles, shiftX, shiftY, alpha, maxDist)
	if contactShadowNearOtherLight(mobile.Index, float32(x), float32(y)) {
		return
	}
	drawContactShadow(screen, size, x, y, metrics.footFraction, shadowAlpha, shadowDarkenBlend)
}

func contactShadowImage() *ebiten.Image {
	if contactShadowTexture != nil {
		return contactShadowTexture
	}
	img := image.NewNRGBA(image.Rect(0, 0, contactShadowTexSize, contactShadowTexSize))
	center := float64(contactShadowTexSize-1) / 2
	for y := 0; y < contactShadowTexSize; y++ {
		for x := 0; x < contactShadowTexSize; x++ {
			dx := (float64(x) - center) / center
			dy := (float64(y) - center) / center
			radiusSquared := dx*dx + dy*dy
			if radiusSquared >= 1 {
				continue
			}
			softness := 1 - radiusSquared
			img.SetNRGBA(x, y, color.NRGBA{A: uint8(255 * softness)})
		}
	}
	contactShadowTexture = newImageFromImage(img)
	return contactShadowTexture
}

func drawContactShadow(screen *ebiten.Image, size, x, y int, footFraction, alpha float32, blend ebiten.Blend) {
	img := contactShadowImage()
	if img == nil || size <= 0 {
		return
	}
	target := float64(roundToInt(float64(size) * gs.GameScale))
	width := target * contactShadowWidth
	height := target * contactShadowHeight
	if width < 1 || height < 1 {
		return
	}
	drawAlpha := characterShadowDrawAlpha(alpha, characterShadowProjection{contrast: 1})
	op := acquireDrawOpts()
	op.Filter = ebiten.FilterLinear
	op.DisableMipmaps = true
	op.Blend = blend
	op.ColorScale.Scale(0, 0, 0, drawAlpha)
	op.GeoM.Scale(width/float64(img.Bounds().Dx()), height/float64(img.Bounds().Dy()))
	footY := float64(y) - target/2 + target*float64(footFraction)
	op.GeoM.Translate(float64(x)-width/2, footY-height/2)
	screen.DrawImage(img, op)
	releaseDrawOpts(op)
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

func characterShadowTextureFor(img *ebiten.Image) characterShadowTexture {
	if img == nil {
		return characterShadowTexture{}
	}
	if texture, ok := characterShadowTextureCache[img]; ok {
		return texture
	}
	bounds := img.Bounds()
	pixels := make([]byte, 4*bounds.Dx()*bounds.Dy())
	img.ReadPixels(pixels)
	footY := opaqueFootY(pixels, bounds.Dx(), bounds.Dy())
	padded := newImage(bounds.Dx()+characterShadowPadding*2, bounds.Dy()+characterShadowPadding*2)
	op := acquireDrawOpts()
	op.GeoM.Translate(characterShadowPadding, characterShadowPadding)
	padded.DrawImage(img, op)
	releaseDrawOpts(op)
	texture := characterShadowTexture{
		image:       padded,
		contentSize: bounds.Dx(),
		padding:     characterShadowPadding,
		footY:       float64(footY + characterShadowPadding),
	}
	characterShadowTextureCache[img] = texture
	return texture
}

func clearCharacterShadowCache() {
	if detailedCharacterShadowMask != nil {
		detailedCharacterShadowMask.Deallocate()
		detailedCharacterShadowMask = nil
	}
	if contactShadowTexture != nil {
		contactShadowTexture.Deallocate()
		contactShadowTexture = nil
	}
	for _, texture := range characterShadowTextureCache {
		texture.image.Deallocate()
	}
	characterShadowTextureCache = make(map[*ebiten.Image]characterShadowTexture)
}

func drawCharacterShadow(screen *ebiten.Image, texture characterShadowTexture, size, x, y int, alpha float32, projection characterShadowProjection, upright bool, blend ebiten.Blend) {
	drawSize := texture.contentSize
	if drawSize <= 0 || size <= 0 {
		return
	}
	shadowAlpha := characterShadowDrawAlpha(alpha, projection)
	drawCharacterShadowLayer(screen, texture, size, x, y, shadowAlpha, projection, upright, blend)
}

func characterShadowDrawAlpha(alpha float32, projection characterShadowProjection) float32 {
	shadowAlpha := alpha * projection.contrast
	if gs.DetailedCharacterShadows {
		shadowAlpha *= detailedCoreOpacity
	} else {
		shadowAlpha *= normalShadowOpacity
	}
	shadowAlpha *= float32(gs.CharacterShadowDarkness)
	if shadowAlpha > 1 {
		shadowAlpha = 1
	}
	return shadowAlpha
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
	padding := float64(texture.padding) * baseScale
	op.GeoM.Translate(float64(x)-target/2-padding+projection.dropOffsetX, float64(y)-target/2-padding+projection.dropOffsetY)

	screen.DrawImage(img, op)
	releaseDrawOpts(op)
}

// drawUprightShadowTexture projects the source frame alpha onto the ground.
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
