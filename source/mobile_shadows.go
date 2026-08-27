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
	mobileSunShadeScale    = 0.65
	maximumMobileSunShade  = 0.75
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

type shadowPoint struct {
	x, y float64
}

type mobileSunShadowCaster struct {
	index    uint8
	quad     [4]shadowPoint
	strength float32
}

type mobileSunShadowReceiver struct {
	index                 uint8
	footX, footY          float64
	radius                float64 // fallback for older callers and tests
	halfWidth, halfHeight float64
}

var (
	frameMobileSunShadowCasters   []mobileSunShadowCaster
	frameMobileSunShadowReceivers []mobileSunShadowReceiver
	frameMobileSunShadowBlocks    = make(map[obscuringBlockKey][]int)
	frameMobileSunShadowUsed      []obscuringBlockKey
)

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

func drawMobileShadows(screen *ebiten.Image, ox, oy int, mobiles []frameMobile, descMap map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64, maxDist int, mobileShade *[256]float32) {
	shadowAlpha, azimuth, kind := currentCharacterShadowRenderState()
	if kind != characterShadowDirectional || clImages == nil {
		return
	}
	projection := newCharacterShadowProjection(azimuth)
	frameMobileSunShadowCasters = frameMobileSunShadowCasters[:0]
	frameMobileSunShadowReceivers = frameMobileSunShadowReceivers[:0]
	resetMobileSunShadowBlocks()
	shadowTarget := screen
	shadowBlend := shadowDarkenBlend
	useMask := gs.DetailedCharacterShadows
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
		texture := characterShadowTextureFor(img)
		x, y := mobileScreenPosition(ox, oy, mobile, prevMobiles, shiftX, shiftY, alpha, maxDist)
		frameMobileSunShadowReceivers = append(frameMobileSunShadowReceivers, mobileSunShadowReceiverFor(mobile.Index, texture, size, x, y))
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
			img = loadMobileFrame(desc.PictID, state, colors)
			if img == nil {
				continue
			}
			img = getScaledMobileFrame(makeMobileKey(desc.PictID, state, colors), img)
			texture = characterShadowTextureFor(img)
		}
		casterAlpha := shadowAlpha * mobileSunShadowAppearance(mobile, desc, prevMobiles, alpha)
		casterIndex := len(frameMobileSunShadowCasters)
		frameMobileSunShadowCasters = append(frameMobileSunShadowCasters, mobileSunShadowCaster{
			index:    mobile.Index,
			quad:     mobileSunShadowQuad(texture, size, x, y, projection, upright),
			strength: characterShadowDrawAlpha(casterAlpha, projection),
		})
		addMobileSunShadowBlocks(casterIndex, frameMobileSunShadowCasters[casterIndex].quad)
		drawCharacterShadow(shadowTarget, texture, size, x, y, casterAlpha, projection, upright, shadowBlend)
	}
	if gs.MobilesReceiveSunShadows && mobileShade != nil {
		for _, receiver := range frameMobileSunShadowReceivers {
			mobileShade[receiver.index] = mobileSunShadowAmount(receiver, frameMobileSunShadowCasters, frameMobileSunShadowBlocks)
		}
	}

	if useMask {
		op := acquireDrawOpts()
		op.Blend = shadowDarkenBlend
		screen.DrawImage(shadowTarget, op)
		releaseDrawOpts(op)
	}
}

// mobileSunShadowAppearance fades only a newly arrived edge caster. An empty
// history means picture-shift matching failed (normally a snell change), so the
// new scene's shadows should appear immediately with the rest of the scene.
func mobileSunShadowAppearance(mobile frameMobile, desc frameDescriptor, prevMobiles map[uint8]frameMobile, alpha float64) float32 {
	if len(prevMobiles) == 0 {
		return 1
	}
	if _, existed := prevMobiles[mobile.Index]; existed || !mobileOnEdge(mobile, desc) {
		return 1
	}
	if alpha <= 0 {
		return 0
	}
	if alpha >= 1 {
		return 1
	}
	return float32(alpha)
}

func resetMobileSunShadowBlocks() {
	for _, key := range frameMobileSunShadowUsed {
		frameMobileSunShadowBlocks[key] = frameMobileSunShadowBlocks[key][:0]
	}
	frameMobileSunShadowUsed = frameMobileSunShadowUsed[:0]
}

func addMobileSunShadowBlocks(casterIndex int, quad [4]shadowPoint) {
	minX, maxX := quad[0].x, quad[0].x
	minY, maxY := quad[0].y, quad[0].y
	for _, point := range quad[1:] {
		minX = math.Min(minX, point.x)
		maxX = math.Max(maxX, point.x)
		minY = math.Min(minY, point.y)
		maxY = math.Max(maxY, point.y)
	}
	minBlockX := obscuringBlockCoordinate(int(math.Floor(minX)))
	maxBlockX := obscuringBlockCoordinate(int(math.Ceil(maxX)))
	minBlockY := obscuringBlockCoordinate(int(math.Floor(minY)))
	maxBlockY := obscuringBlockCoordinate(int(math.Ceil(maxY)))
	for blockY := minBlockY; blockY <= maxBlockY; blockY++ {
		for blockX := minBlockX; blockX <= maxBlockX; blockX++ {
			key := obscuringBlockKey{blockX, blockY}
			entries := frameMobileSunShadowBlocks[key]
			if len(entries) == 0 {
				frameMobileSunShadowUsed = append(frameMobileSunShadowUsed, key)
			}
			frameMobileSunShadowBlocks[key] = append(entries, casterIndex)
		}
	}
}

func mobileSunShadowReceiverFor(index uint8, texture characterShadowTexture, size, x, y int) mobileSunShadowReceiver {
	target := float64(roundToInt(float64(size) * gs.GameScale))
	footY := float64(y) + target*0.45
	if texture.contentSize > 0 {
		baseScale := target / float64(texture.contentSize)
		footY = float64(y) - target/2 + (texture.footY-float64(texture.padding))*baseScale
	}
	return mobileSunShadowReceiver{
		index:      index,
		footX:      float64(x),
		footY:      footY,
		radius:     target * 0.14,
		halfWidth:  target * 0.14,
		halfHeight: target * 0.06,
	}
}

func mobileSunShadowQuad(texture characterShadowTexture, size, x, y int, projection characterShadowProjection, upright bool) [4]shadowPoint {
	if upright {
		geo := uprightShadowGeoMWithFoot(texture.contentSize, texture.padding, texture.footY, size, x, y, projection)
		bounds := texture.image.Bounds()
		return transformedShadowQuad(geo, float64(bounds.Dx()), float64(bounds.Dy()))
	}
	drawSize := texture.contentSize
	target := float64(roundToInt(float64(size) * gs.GameScale))
	baseScale := target / float64(drawSize)
	padding := float64(texture.padding) * baseScale
	left := float64(x) - target/2 - padding + projection.dropOffsetX
	top := float64(y) - target/2 - padding + projection.dropOffsetY
	extent := target + padding*2
	return [4]shadowPoint{{left, top}, {left + extent, top}, {left + extent, top + extent}, {left, top + extent}}
}

func transformedShadowQuad(geo ebiten.GeoM, width, height float64) [4]shadowPoint {
	x0, y0 := geo.Apply(0, 0)
	x1, y1 := geo.Apply(width, 0)
	x2, y2 := geo.Apply(width, height)
	x3, y3 := geo.Apply(0, height)
	return [4]shadowPoint{{x0, y0}, {x1, y1}, {x2, y2}, {x3, y3}}
}

func mobileSunShadowAmount(receiver mobileSunShadowReceiver, casters []mobileSunShadowCaster, blocks map[obscuringBlockKey][]int) float32 {
	if receiver.radius <= 0 {
		return 0
	}
	halfWidth, halfHeight := receiver.halfWidth, receiver.halfHeight
	if halfWidth <= 0 {
		halfWidth = receiver.radius
	}
	if halfHeight <= 0 {
		halfHeight = receiver.radius
	}
	// Sample the receiver's expected ground-contact bounds against the exact
	// projected shadow quad. A small regular grid gives stable partial coverage
	// without the generous corners of the old circular hitbox.
	offsets := [...]float64{-1, -0.5, 0, 0.5, 1}
	coveredStrength := float32(0)
	for _, oy := range offsets {
		for _, ox := range offsets {
			point := shadowPoint{receiver.footX + ox*halfWidth, receiver.footY + oy*halfHeight}
			key := obscuringBlockKey{obscuringBlockCoordinate(int(math.Floor(point.x))), obscuringBlockCoordinate(int(math.Floor(point.y)))}
			pointStrength := float32(0)
			for _, casterIndex := range blocks[key] {
				caster := casters[casterIndex]
				if caster.index != receiver.index && caster.strength > pointStrength && pointInShadowQuad(point, caster.quad) {
					pointStrength = caster.strength
				}
			}
			coveredStrength += pointStrength
		}
	}
	shade := coveredStrength / float32(len(offsets)*len(offsets)) * mobileSunShadeScale
	if shade > maximumMobileSunShade {
		shade = maximumMobileSunShade
	}
	return shade
}

func pointInShadowQuad(point shadowPoint, quad [4]shadowPoint) bool {
	inside := false
	for i, j := 0, len(quad)-1; i < len(quad); j, i = i, i+1 {
		a, b := quad[i], quad[j]
		if (a.y > point.y) != (b.y > point.y) && point.x < (b.x-a.x)*(point.y-a.y)/(b.y-a.y)+a.x {
			inside = !inside
		}
	}
	return inside
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
	contactShadowTexture = newManagedImageFromImage(img)
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
		detailedCharacterShadowMask = newUnmanagedImage(bounds.Dx(), bounds.Dy())
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
	padded := newManagedImage(bounds.Dx()+characterShadowPadding*2, bounds.Dy()+characterShadowPadding*2)
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
