package main

import (
	_ "embed"
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

var shadowCoverageClearBlend = ebiten.Blend{
	BlendFactorSourceRGB:        ebiten.BlendFactorZero,
	BlendFactorSourceAlpha:      ebiten.BlendFactorZero,
	BlendFactorDestinationRGB:   ebiten.BlendFactorOneMinusSourceAlpha,
	BlendFactorDestinationAlpha: ebiten.BlendFactorOneMinusSourceAlpha,
	BlendOperationRGB:           ebiten.BlendOperationAdd,
	BlendOperationAlpha:         ebiten.BlendOperationAdd,
}

var (
	uprightShadowVertices = [4]ebiten.Vertex{}
	uprightShadowIndices  = [6]uint16{0, 1, 2, 1, 3, 2}
	uprightShadowDrawOpts = ebiten.DrawTrianglesOptions{
		Filter:         ebiten.FilterLinear,
		Address:        ebiten.AddressClampToZero,
		DisableMipmaps: true,
		Blend:          shadowDarkenBlend,
	}
	detailedCharacterShadowMask       *ebiten.Image
	frameDetailedShadowMask           *ebiten.Image
	frameDetailedShadowBounds         image.Rectangle
	contactShadowTexture              *ebiten.Image
	frameCharacterShadowDraws         []characterShadowDraw
	frameLayeredShadowDraws           [256]characterShadowDraw
	frameLayeredShadowReady           [256]bool
	layeredShadowCompositeShader      *ebiten.Shader
	layeredShadowCoverage             *ebiten.Image
	layeredShadowIncoming             *ebiten.Image
	layeredShadowScene                *ebiten.Image
	layeredShadowOrigin               image.Point
	frameLayeredShadowCoverageBounds  image.Rectangle
	frameLayeredShadowCoverageRects   []image.Rectangle
	frameLayeredShadowCompositeActive bool
)

//go:embed data/shaders/character_shadow_composite.kage
var layeredShadowCompositeShaderSource []byte

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

type characterShadowDraw struct {
	texture    characterShadowTexture
	size       int
	x, y       int
	alpha      float32
	projection characterShadowProjection
	upright    bool
	quad       [4]shadowPoint
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
	frameDetailedShadowMask = nil
	frameDetailedShadowBounds = image.Rectangle{}
	frameCharacterShadowDraws = frameCharacterShadowDraws[:0]
	// drawScene can start the layered compositor before negative-plane shadow
	// pictures are drawn. Preserve that coverage so character shadows share the
	// same maximum-opacity mask instead of darkening those pictures again.
	if !frameLayeredShadowCompositeActive {
		resetLayeredCharacterShadows()
	}
	shadowAlpha, azimuth, kind := currentCharacterShadowRenderState()
	if kind != characterShadowDirectional || clImages == nil {
		return
	}
	projection := newCharacterShadowProjection(azimuth)
	shadeMobiles := gs.MobilesReceiveSunShadows && mobileShade != nil
	if shadeMobiles {
		frameMobileSunShadowCasters = frameMobileSunShadowCasters[:0]
		frameMobileSunShadowReceivers = frameMobileSunShadowReceivers[:0]
		resetMobileSunShadowBlocks()
	}
	useLayered := layeredShadowCompositeShader != nil &&
		(frameLayeredShadowCompositeActive || layeredCharacterShadowsEnabled())
	useMask := !useLayered && characterShadowCompositeEnabled() && lightingShader != nil
	if useLayered && !frameLayeredShadowCompositeActive {
		beginLayeredCharacterShadowComposite(screen.Bounds())
	}

	for _, mobile := range mobiles {
		desc, ok := descMap[mobile.Index]
		if !ok {
			continue
		}
		state := mobile.State
		colors := playerColorsForDescriptor(desc)
		if mobileGPURecolorEligible(desc.PictID, colors) {
			colors = nil
		}
		img := loadMobileFrame(desc.PictID, state, colors)
		if img == nil {
			continue
		}
		img = getScaledMobileFrame(makeMobileKey(desc.PictID, state, colors), img)
		size := mobileSize(desc.PictID)
		if size == 0 {
			size = img.Bounds().Dx()
		}
		key := makeMobileKey(desc.PictID, state, colors)
		texture := characterShadowTextureForMobile(key, img)
		x, y := mobileScreenPosition(ox, oy, mobile, prevMobiles, shiftX, shiftY, alpha, maxDist)
		if shadeMobiles {
			frameMobileSunShadowReceivers = append(frameMobileSunShadowReceivers, mobileSunShadowReceiverFor(mobile.Index, texture, size, x, y))
		}
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
			key.state = state
			texture = characterShadowTextureForMobile(key, img)
		}
		casterAlpha := shadowAlpha * mobileSunShadowAppearance(mobile, desc, prevMobiles, alpha)
		quad := mobileSunShadowQuad(texture, size, x, y, projection, upright)
		command := characterShadowDraw{
			texture: texture, size: size, x: x, y: y, alpha: casterAlpha,
			projection: projection, upright: upright, quad: quad,
		}
		if shadeMobiles {
			casterIndex := len(frameMobileSunShadowCasters)
			frameMobileSunShadowCasters = append(frameMobileSunShadowCasters, mobileSunShadowCaster{
				index: mobile.Index, quad: quad,
				strength: characterShadowDrawAlpha(casterAlpha, projection),
			})
			addMobileSunShadowBlocks(casterIndex, quad)
		}
		if useLayered {
			queueLayeredCharacterShadow(mobile.Index, command)
		} else {
			frameCharacterShadowDraws = append(frameCharacterShadowDraws, command)
		}
	}
	if shadeMobiles {
		for _, receiver := range frameMobileSunShadowReceivers {
			mobileShade[receiver.index] = mobileSunShadowAmount(receiver, frameMobileSunShadowCasters, frameMobileSunShadowBlocks)
		}
	}
	if useLayered {
		return
	}
	if !useMask {
		for _, command := range frameCharacterShadowDraws {
			drawCharacterShadow(screen, command.texture, command.size, command.x, command.y, command.alpha, command.projection, command.upright, shadowDarkenBlend)
		}
		return
	}
	if len(frameCharacterShadowDraws) == 0 {
		return
	}
	activeBounds := shadowQuadBounds(frameCharacterShadowDraws[0].quad)
	for _, command := range frameCharacterShadowDraws[1:] {
		activeBounds = activeBounds.Union(shadowQuadBounds(command.quad))
	}
	activeBounds = activeBounds.Intersect(screen.Bounds())
	if activeBounds.Empty() {
		return
	}
	shadowTarget := characterShadowMask(activeBounds.Size())
	shadowTarget.SubImage(image.Rectangle{Max: activeBounds.Size()}).(*ebiten.Image).Clear()
	for _, command := range frameCharacterShadowDraws {
		drawCharacterShadow(
			shadowTarget,
			command.texture,
			command.size,
			command.x-activeBounds.Min.X,
			command.y-activeBounds.Min.Y,
			command.alpha,
			command.projection,
			command.upright,
			shadowMaskBlend,
		)
	}
	frameDetailedShadowMask = shadowTarget
	frameDetailedShadowBounds = activeBounds
}

func resetLayeredCharacterShadows() {
	clear(frameLayeredShadowDraws[:])
	clear(frameLayeredShadowReady[:])
	frameLayeredShadowCoverageBounds = image.Rectangle{}
	frameLayeredShadowCoverageRects = frameLayeredShadowCoverageRects[:0]
	frameLayeredShadowCompositeActive = false
}

func beginLayeredCharacterShadowComposite(bounds image.Rectangle) {
	if bounds.Empty() {
		return
	}
	layeredShadowOrigin = bounds.Min
	layeredShadowCoverage = ensureCharacterShadowImage(layeredShadowCoverage, bounds.Size())
	layeredShadowCoverage.SubImage(image.Rectangle{Max: bounds.Size()}).(*ebiten.Image).Clear()
	frameLayeredShadowCoverageBounds = image.Rectangle{}
	frameLayeredShadowCoverageRects = frameLayeredShadowCoverageRects[:0]
	frameLayeredShadowCompositeActive = true
}

func ensureCharacterShadowImage(current *ebiten.Image, size image.Point) *ebiten.Image {
	if size.X < 1 || size.Y < 1 {
		return current
	}
	if current != nil && current.Bounds().Dx() >= size.X && current.Bounds().Dy() >= size.Y {
		return current
	}
	width, height := size.X, size.Y
	if current != nil {
		width = max(width, current.Bounds().Dx())
		height = max(height, current.Bounds().Dy())
		current.Deallocate()
	}
	return newUnmanagedImage(width, height)
}

func queueLayeredCharacterShadow(index uint8, command characterShadowDraw) {
	frameLayeredShadowDraws[index] = command
	frameLayeredShadowReady[index] = true
}

func takeLayeredCharacterShadow(index uint8) (characterShadowDraw, bool) {
	if !frameLayeredShadowReady[index] {
		return characterShadowDraw{}, false
	}
	frameLayeredShadowReady[index] = false
	command := frameLayeredShadowDraws[index]
	frameLayeredShadowDraws[index] = characterShadowDraw{}
	return command, true
}

// drawLayeredCharacterShadow draws a prepared directional shadow immediately
// before its caster. Later scenery and mobiles can then cover it naturally,
// matching the scene's painter order.
func drawLayeredCharacterShadow(screen *ebiten.Image, index uint8) {
	if screen == nil {
		return
	}
	command, ok := takeLayeredCharacterShadow(index)
	if !ok {
		return
	}
	compositeLayeredCharacterShadow(screen, command)
}

// Coverage rectangles deliberately survive partial alpha clears: stale bounds
// can only select the slower composition path, never miss existing coverage.
func overlapsLayeredShadowCoverage(bounds image.Rectangle) bool {
	if !bounds.Overlaps(frameLayeredShadowCoverageBounds) {
		return false
	}
	for _, previous := range frameLayeredShadowCoverageRects {
		if bounds.Overlaps(previous) {
			return true
		}
	}
	return false
}

func recordLayeredShadowCoverage(bounds image.Rectangle) {
	if bounds.Empty() {
		return
	}
	frameLayeredShadowCoverageBounds = frameLayeredShadowCoverageBounds.Union(bounds)
	frameLayeredShadowCoverageRects = append(frameLayeredShadowCoverageRects, bounds)
}

func compositeLayeredCharacterShadow(screen *ebiten.Image, command characterShadowDraw) {
	if !frameLayeredShadowCompositeActive || layeredShadowCompositeShader == nil || layeredShadowCoverage == nil {
		drawCharacterShadow(screen, command.texture, command.size, command.x, command.y, command.alpha, command.projection, command.upright, shadowDarkenBlend)
		return
	}
	bounds := shadowQuadBounds(command.quad).Intersect(screen.Bounds())
	if bounds.Empty() {
		return
	}
	// With no earlier coverage under this caster, maximum-opacity composition
	// is identical to a direct darken. Draw the shadow and its coverage mask in
	// two submissions instead of clearing/copying/compositing three scratch
	// targets. Individual caster bounds avoid treating gaps in the union as
	// covered. Bounds remain conservative when foreground artwork clears pixels.
	if !overlapsLayeredShadowCoverage(bounds) {
		drawCharacterShadow(screen, command.texture, command.size, command.x, command.y, command.alpha, command.projection, command.upright, shadowDarkenBlend)
		drawCharacterShadow(
			layeredShadowCoverage,
			command.texture,
			command.size,
			command.x-layeredShadowOrigin.X,
			command.y-layeredShadowOrigin.Y,
			command.alpha,
			command.projection,
			command.upright,
			shadowMaskBlend,
		)
		recordLayeredShadowCoverage(bounds)
		return
	}
	size := bounds.Size()
	layeredShadowIncoming = ensureCharacterShadowImage(layeredShadowIncoming, size)
	layeredShadowScene = ensureCharacterShadowImage(layeredShadowScene, size)
	incoming := layeredShadowIncoming.SubImage(image.Rectangle{Max: size}).(*ebiten.Image)
	scene := layeredShadowScene.SubImage(image.Rectangle{Max: size}).(*ebiten.Image)
	incoming.Clear()

	drawCharacterShadow(incoming, command.texture, command.size, command.x-bounds.Min.X, command.y-bounds.Min.Y, command.alpha, command.projection, command.upright, shadowMaskBlend)
	sceneCopyOp := &ebiten.DrawImageOptions{Blend: ebiten.BlendCopy}
	scene.DrawImage(screen.SubImage(bounds).(*ebiten.Image), sceneCopyOp)
	coverageRect := bounds.Sub(layeredShadowOrigin)
	coverageSource := layeredShadowCoverage.SubImage(coverageRect).(*ebiten.Image)

	shaderOp := &ebiten.DrawRectShaderOptions{}
	shaderOp.Images[0] = scene
	shaderOp.Images[1] = coverageSource
	shaderOp.Images[2] = incoming
	shaderOp.Blend = ebiten.BlendCopy
	shaderOp.GeoM.Translate(float64(bounds.Min.X), float64(bounds.Min.Y))
	screen.DrawRectShader(size.X, size.Y, layeredShadowCompositeShader, shaderOp)

	updateOp := &ebiten.DrawImageOptions{Blend: shadowMaskBlend}
	updateOp.GeoM.Translate(float64(coverageRect.Min.X), float64(coverageRect.Min.Y))
	layeredShadowCoverage.DrawImage(incoming, updateOp)
	recordLayeredShadowCoverage(bounds)
}

// compositeLayeredShadowImage draws explicit shadow artwork through the same
// maximum-opacity coverage used by character shadows. The source image's alpha
// (including PictDef blend metadata and draw opacity) becomes shadow strength;
// its RGB values are deliberately ignored.
func compositeLayeredShadowImage(screen, source *ebiten.Image, drawOp *ebiten.DrawImageOptions, bounds image.Rectangle) bool {
	if screen == nil || source == nil || drawOp == nil || !frameLayeredShadowCompositeActive ||
		layeredShadowCompositeShader == nil || layeredShadowCoverage == nil {
		return false
	}
	bounds = bounds.Intersect(screen.Bounds())
	if bounds.Empty() {
		return true
	}
	if !overlapsLayeredShadowCoverage(bounds) {
		darkenOp := *drawOp
		darkenOp.Blend = shadowDarkenBlend
		screen.DrawImage(source, &darkenOp)

		coverageOp := *drawOp
		coverageOp.Blend = shadowMaskBlend
		coverageOp.GeoM.Translate(float64(-layeredShadowOrigin.X), float64(-layeredShadowOrigin.Y))
		layeredShadowCoverage.DrawImage(source, &coverageOp)
		recordLayeredShadowCoverage(bounds)
		return true
	}

	size := bounds.Size()
	layeredShadowIncoming = ensureCharacterShadowImage(layeredShadowIncoming, size)
	layeredShadowScene = ensureCharacterShadowImage(layeredShadowScene, size)
	incoming := layeredShadowIncoming.SubImage(image.Rectangle{Max: size}).(*ebiten.Image)
	scene := layeredShadowScene.SubImage(image.Rectangle{Max: size}).(*ebiten.Image)
	incoming.Clear()

	incomingOp := *drawOp
	incomingOp.Blend = shadowMaskBlend
	incomingOp.GeoM.Translate(float64(-bounds.Min.X), float64(-bounds.Min.Y))
	incoming.DrawImage(source, &incomingOp)
	scene.DrawImage(screen.SubImage(bounds).(*ebiten.Image), &ebiten.DrawImageOptions{Blend: ebiten.BlendCopy})
	coverageRect := bounds.Sub(layeredShadowOrigin)
	coverageSource := layeredShadowCoverage.SubImage(coverageRect).(*ebiten.Image)

	shaderOp := &ebiten.DrawRectShaderOptions{}
	shaderOp.Images[0] = scene
	shaderOp.Images[1] = coverageSource
	shaderOp.Images[2] = incoming
	shaderOp.Blend = ebiten.BlendCopy
	shaderOp.GeoM.Translate(float64(bounds.Min.X), float64(bounds.Min.Y))
	screen.DrawRectShader(size.X, size.Y, layeredShadowCompositeShader, shaderOp)

	updateOp := &ebiten.DrawImageOptions{Blend: shadowMaskBlend}
	updateOp.GeoM.Translate(float64(coverageRect.Min.X), float64(coverageRect.Min.Y))
	layeredShadowCoverage.DrawImage(incoming, updateOp)
	recordLayeredShadowCoverage(bounds)
	return true
}

// clearLayeredShadowCoverageImage removes previous shadow coverage where a new
// receiver was painted. A later caster can then shadow that newly visible
// artwork without multiplying darkness on the still-visible surroundings.
func clearLayeredShadowCoverageImage(source *ebiten.Image, drawOp *ebiten.DrawImageOptions) {
	if !frameLayeredShadowCompositeActive || layeredShadowCoverage == nil || source == nil || drawOp == nil {
		return
	}
	op := *drawOp
	op.Blend = shadowCoverageClearBlend
	op.GeoM.Translate(float64(-layeredShadowOrigin.X), float64(-layeredShadowOrigin.Y))
	layeredShadowCoverage.DrawImage(source, &op)
}

func clearLayeredShadowCoverageFrameBlend(previous, current *ebiten.Image, options frameBlendDrawOptions) {
	if !frameLayeredShadowCompositeActive || previous == nil || current == nil {
		return
	}
	previousBounds := previous.Bounds()
	currentBounds := current.Bounds()
	width := max(previousBounds.Dx(), currentBounds.Dx())
	height := max(previousBounds.Dy(), currentBounds.Dy())
	for _, frame := range []struct {
		image  *ebiten.Image
		bounds image.Rectangle
	}{
		{image: previous, bounds: previousBounds},
		{image: current, bounds: currentBounds},
	} {
		offsetX := float64(width-frame.bounds.Dx()) / 2
		offsetY := float64(height-frame.bounds.Dy()) / 2
		op := &ebiten.DrawImageOptions{}
		op.Filter = ebiten.FilterNearest
		if options.Linear {
			op.Filter = ebiten.FilterLinear
		}
		op.DisableMipmaps = true
		op.GeoM.Scale(options.ScaleX, options.ScaleY)
		op.GeoM.Translate(options.Left+offsetX*options.ScaleX, options.Top+offsetY*options.ScaleY)
		op.ColorScale.Scale(1, 1, 1, options.Alpha)
		clearLayeredShadowCoverageImage(frame.image, op)
	}
}

func clearLayeredShadowCoverageRect(x, y, width, height float64) {
	if !frameLayeredShadowCompositeActive || whiteImage == nil || width <= 0 || height <= 0 {
		return
	}
	bounds := whiteImage.Bounds()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(width/float64(bounds.Dx()), height/float64(bounds.Dy()))
	op.GeoM.Translate(x, y)
	clearLayeredShadowCoverageImage(whiteImage, op)
}

func applyDetailedCharacterShadow(dst *ebiten.Image) {
	if dst == nil || frameDetailedShadowMask == nil || frameDetailedShadowBounds.Empty() {
		return
	}
	bounds := frameDetailedShadowBounds.Intersect(dst.Bounds())
	if bounds.Empty() {
		return
	}
	sourceRect := image.Rectangle{Max: bounds.Size()}
	source := frameDetailedShadowMask.SubImage(sourceRect).(*ebiten.Image)
	op := &ebiten.DrawImageOptions{Blend: shadowDarkenBlend}
	op.GeoM.Translate(float64(bounds.Min.X), float64(bounds.Min.Y))
	dst.DrawImage(source, op)
}

func shadowQuadBounds(quad [4]shadowPoint) image.Rectangle {
	minX, maxX := quad[0].x, quad[0].x
	minY, maxY := quad[0].y, quad[0].y
	for _, point := range quad[1:] {
		minX = math.Min(minX, point.x)
		maxX = math.Max(maxX, point.x)
		minY = math.Min(minY, point.y)
		maxY = math.Max(maxY, point.y)
	}
	const filterMargin = 2
	return image.Rect(
		int(math.Floor(minX))-filterMargin,
		int(math.Floor(minY))-filterMargin,
		int(math.Ceil(maxX))+filterMargin,
		int(math.Ceil(maxY))+filterMargin,
	)
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
	// Sample the receiver's ground-contact area against only the projected
	// shadows in the same reused 128-pixel blockmap cells.
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
	if mobileGPURecolorEligible(desc.PictID, colors) {
		colors = nil
	}
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
	texture := characterShadowTextureForMobile(key, img)
	x, y := mobileScreenPosition(ox, oy, mobile, prevMobiles, shiftX, shiftY, alpha, maxDist)
	projection := characterShadowProjection{
		dropOffsetX: lyingShadowOffset * gs.GameScale,
		dropOffsetY: lyingShadowOffset * gs.GameScale,
		contrast:    1,
	}
	if frameLayeredShadowCompositeActive {
		command := characterShadowDraw{
			texture: texture, size: size, x: x, y: y, alpha: shadowAlpha,
			projection: projection,
		}
		command.quad = mobileSunShadowQuad(texture, size, x, y, projection, false)
		compositeLayeredCharacterShadow(screen, command)
		return
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
	if mobileGPURecolorEligible(desc.PictID, colors) {
		colors = nil
	}
	img := loadMobileFrame(desc.PictID, mobile.State, colors)
	if img == nil {
		return
	}
	key := makeMobileKey(desc.PictID, mobile.State, colors)
	img = getScaledMobileFrame(key, img)
	metrics := mobileSpriteMetricsFor(key, img)
	x, y := mobileScreenPosition(ox, oy, mobile, prevMobiles, shiftX, shiftY, alpha, maxDist)
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
	left, top := float64(x)-width/2, footY-height/2
	op.GeoM.Translate(left, top)
	const filterMargin = 2
	bounds := image.Rect(
		int(math.Floor(left))-filterMargin,
		int(math.Floor(top))-filterMargin,
		int(math.Ceil(left+width))+filterMargin,
		int(math.Ceil(top+height))+filterMargin,
	)
	if blend != shadowDarkenBlend || !compositeLayeredShadowImage(screen, img, op, bounds) {
		screen.DrawImage(img, op)
	}
	releaseDrawOpts(op)
}

func characterShadowMask(size image.Point) *ebiten.Image {
	if detailedCharacterShadowMask == nil || detailedCharacterShadowMask.Bounds().Dx() < size.X || detailedCharacterShadowMask.Bounds().Dy() < size.Y {
		width, height := size.X, size.Y
		if detailedCharacterShadowMask != nil {
			width = max(width, detailedCharacterShadowMask.Bounds().Dx())
			height = max(height, detailedCharacterShadowMask.Bounds().Dy())
		}
		if detailedCharacterShadowMask != nil {
			detailedCharacterShadowMask.Deallocate()
		}
		detailedCharacterShadowMask = newUnmanagedImage(width, height)
	}
	return detailedCharacterShadowMask
}

func characterShadowTextureFor(img *ebiten.Image) characterShadowTexture {
	if img == nil {
		return characterShadowTexture{}
	}
	bounds := img.Bounds()
	return characterShadowTexture{
		image:       img,
		contentSize: bounds.Dx(),
		footY:       float64(bounds.Dy()),
	}

}

func characterShadowTextureForMobile(key mobileKey, img *ebiten.Image) characterShadowTexture {
	texture := characterShadowTextureFor(img)
	if texture.image == nil {
		return texture
	}
	metrics := mobileSpriteMetricsFor(key, img)
	texture.footY = float64(texture.image.Bounds().Dy()) * float64(metrics.footFraction)
	return texture
}

func clearCharacterShadowCache() {
	resetLayeredCharacterShadows()
	if detailedCharacterShadowMask != nil {
		detailedCharacterShadowMask.Deallocate()
		detailedCharacterShadowMask = nil
	}
	if contactShadowTexture != nil {
		deallocateImage(contactShadowTexture)
		contactShadowTexture = nil
	}
	for _, target := range []*ebiten.Image{layeredShadowCoverage, layeredShadowIncoming, layeredShadowScene} {
		if target != nil {
			target.Deallocate()
		}
	}
	layeredShadowCoverage = nil
	layeredShadowIncoming = nil
	layeredShadowScene = nil
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
	if characterShadowCompositeEnabled() {
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
	drawSize := texture.contentSize
	if upright {
		geo := uprightShadowGeoMWithFoot(drawSize, texture.padding, texture.footY, size, x, y, projection)
		drawUprightShadowTexture(screen, texture, geo, alpha, blend)
		return
	}

	target := float64(roundToInt(float64(size) * gs.GameScale))
	baseScale := target / float64(drawSize)
	var geo ebiten.GeoM
	geo.Scale(baseScale, baseScale)
	padding := float64(texture.padding) * baseScale
	geo.Translate(float64(x)-target/2-padding+projection.dropOffsetX, float64(y)-target/2-padding+projection.dropOffsetY)
	drawUprightShadowTexture(screen, texture, geo, alpha, blend)
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
