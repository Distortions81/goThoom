package main

import (
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
	shadowHeadOpacity      = 0.10
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
)

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
		sunDirection := (normalizeShadowAzimuth(azimuth) + 23) / 45
		shadowFacing := (facing + sunDirection + 6) & 7
		return uint8(shadowFacing*4 + int(state&3)), true
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
	if gs.DetailedCharacterShadows {
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
		img = getScaledMobileFrame(key, img)
		if img == nil {
			continue
		}

		size := mobileSize(desc.PictID)
		if size == 0 {
			size = img.Bounds().Dx()
		}
		x, y := mobileScreenPosition(ox, oy, mobile, prevMobiles, shiftX, shiftY, alpha, maxDist)
		drawCharacterShadow(shadowTarget, img, size, x, y, shadowAlpha, projection, upright, shadowBlend)
	}

	if gs.DetailedCharacterShadows {
		op := acquireDrawOpts()
		op.Blend = shadowDarkenBlend
		screen.DrawImage(shadowTarget, op)
		releaseDrawOpts(op)
	}
}

func characterShadowMask(screen *ebiten.Image) *ebiten.Image {
	bounds := screen.Bounds()
	if detailedCharacterShadowMask == nil || detailedCharacterShadowMask.Bounds().Dx() != bounds.Dx() || detailedCharacterShadowMask.Bounds().Dy() != bounds.Dy() {
		detailedCharacterShadowMask = ebiten.NewImage(bounds.Dx(), bounds.Dy())
	}
	return detailedCharacterShadowMask
}

func drawCharacterShadow(screen, img *ebiten.Image, size, x, y int, alpha float32, projection characterShadowProjection, upright bool, blend ebiten.Blend) {
	drawSize := img.Bounds().Dx()
	if drawSize <= 0 || size <= 0 {
		return
	}
	shadowAlpha := alpha * projection.contrast
	if gs.DetailedCharacterShadows {
		shadowAlpha *= detailedCoreOpacity
	} else {
		shadowAlpha *= normalShadowOpacity
	}
	drawCharacterShadowLayer(screen, img, drawSize, size, x, y, shadowAlpha, projection, upright, blend)
}

func drawCharacterShadowLayer(screen, img *ebiten.Image, drawSize, size, x, y int, alpha float32, projection characterShadowProjection, upright bool, blend ebiten.Blend) {
	if upright {
		geo := uprightShadowGeoM(drawSize, size, x, y, projection)
		drawUprightShadowGradient(screen, img, geo, alpha, blend)
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
	op.GeoM.Translate(float64(x)-target/2+projection.dropOffsetX, float64(y)-target/2+projection.dropOffsetY)

	screen.DrawImage(img, op)
	releaseDrawOpts(op)
}

// drawUprightShadowGradient keeps the shadow darkest at the feet and linearly
// fades it toward the projected head. DrawTriangles already uses a four-vertex
// quad internally, so this adds no render pass and negligible GPU work.
func drawUprightShadowGradient(screen, img *ebiten.Image, geo ebiten.GeoM, alpha float32, blend ebiten.Blend) {
	bounds := img.Bounds()
	w, h := float64(bounds.Dx()), float64(bounds.Dy())
	topLeftX, topLeftY := geo.Apply(0, 0)
	topRightX, topRightY := geo.Apply(w, 0)
	bottomLeftX, bottomLeftY := geo.Apply(0, h)
	bottomRightX, bottomRightY := geo.Apply(w, h)
	topAlpha := alpha * shadowHeadOpacity

	uprightShadowVertices = [4]ebiten.Vertex{
		{DstX: float32(topLeftX), DstY: float32(topLeftY), SrcX: float32(bounds.Min.X), SrcY: float32(bounds.Min.Y), ColorA: topAlpha},
		{DstX: float32(topRightX), DstY: float32(topRightY), SrcX: float32(bounds.Max.X), SrcY: float32(bounds.Min.Y), ColorA: topAlpha},
		{DstX: float32(bottomLeftX), DstY: float32(bottomLeftY), SrcX: float32(bounds.Min.X), SrcY: float32(bounds.Max.Y), ColorA: alpha},
		{DstX: float32(bottomRightX), DstY: float32(bottomRightY), SrcX: float32(bounds.Max.X), SrcY: float32(bounds.Max.Y), ColorA: alpha},
	}
	uprightShadowDrawOpts.Blend = blend
	screen.DrawTriangles(uprightShadowVertices[:], uprightShadowIndices[:], img, &uprightShadowDrawOpts)
}

func uprightShadowGeoM(drawSize, size, x, y int, projection characterShadowProjection) ebiten.GeoM {
	target := float64(roundToInt(float64(size) * gs.GameScale))
	baseScale := target / float64(drawSize)

	var geo ebiten.GeoM
	geo.Translate(-float64(drawSize)/2, -float64(drawSize))
	// The classic OpenGL path swaps the left and right shadow vertices before
	// rotating the quad.
	geo.Scale(-baseScale, baseScale*projection.length)
	geo.Rotate(-(projection.angle + math.Pi/2))
	geo.Translate(float64(x), float64(y)+target/2)
	return geo
}
